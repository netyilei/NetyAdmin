// migrate.go：模块迁移执行器——goose/v3 引擎 + 自研 runner 外壳（15）。
// 外壳职责（docs/rules/migrations.md；CI check-migrations 的运行时对偶）：
//   - 启用模块过滤（禁用模块不执行，§25.3）；
//   - 文件名/补丁后缀/up-down 配对校验（号段与段归属校验归 CI check-migrations，不重复）；
//   - test_seed 仅 WithTestSeed（生产禁止）；
//   - migration_checksums(module, version, sha256)：已应用迁移内容变更即失败。
package database

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"regexp"
	"sort"
	"strings"
	"testing/fstest"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

// migrationNameRE 解析迁移文件名（运行时只解析提取 version/kind/dir；
// 号段/段归属/补丁后缀校验是 CI check-migrations 的职责，运行时不重复——G1-05）。
var migrationNameRE = regexp.MustCompile(`^(\d{5})_([a-z0-9]+)_([a-z0-9_]+?)_(schema|indexes|constraints|views|functions|triggers|seed|test_seed|backfill)\.(up|down)\.sql$`)

var patchSuffix = regexp.MustCompile(`_(fix|alter|v2|patch|new)$`)

// pgErrUndefinedTable 是 PG undefined_table（全新库版本/checksum 表未建——
// Status 只读面按零水位处理，不视为故障）。
const pgErrUndefinedTable = "42P01"

// MigrationSource 是一个启用模块的迁移源（FS 根即 migrations/ 目录）。
type MigrationSource struct {
	Module string
	FS     fs.FS
}

// RunnerOptions 是迁移执行器参数。
type RunnerOptions struct {
	Sources      []MigrationSource // 仅启用模块（调用方按 deployment 决定）
	WithTestSeed bool              // test_seed 仅测试环境执行（生产禁止）
}

// Runner 是迁移执行器（启动期/部署期一次性使用；非并发安全）。
type Runner struct {
	pool *Pool
	opts RunnerOptions
}

// NewRunner 创建迁移执行器。
func NewRunner(pool *Pool, opts RunnerOptions) *Runner {
	return &Runner{pool: pool, opts: opts}
}

type migrationFile struct {
	module  string
	version string // 5 位
	kind    string
	dir     string // up | down
	name    string // 完整文件名
	data    []byte
}

// collect 校验文件名/号段/配对并返回待迁移集合（按版本排序）。
// 模块名与目录声明一致性是硬校验（防模块间迁移混放）。
func (r *Runner) collect() (map[string]*migrationFile, error) {
	out := map[string]*migrationFile{}
	for _, src := range r.opts.Sources {
		entries, err := fs.ReadDir(src.FS, ".")
		if err != nil {
			return nil, fmt.Errorf("migrate: read dir %s: %w", src.Module, err)
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
				continue
			}
			base := e.Name()
			m := migrationNameRE.FindStringSubmatch(base)
			if m == nil {
				return nil, fmt.Errorf("migrate: %s/%s 文件名不符合 NNNNN_<module>_<table>_<kind>.<up|down>.sql", src.Module, base)
			}
			num, fileMod, table, kind, dir := m[1], m[2], m[3], m[4], m[5]
			if fileMod != src.Module {
				return nil, fmt.Errorf("migrate: %s/%s 模块名 %q 与源 %q 不一致", src.Module, base, fileMod, src.Module)
			}
			if patchSuffix.MatchString(table) {
				return nil, fmt.Errorf("migrate: %s/%s 禁止补丁式命名（_fix/_alter/_v2/_patch/_new）", src.Module, base)
			}
			if kind == "test_seed" && !r.opts.WithTestSeed {
				continue // 生产跳过测试种子
			}
			data, err := fs.ReadFile(src.FS, base)
			if err != nil {
				return nil, fmt.Errorf("migrate: read %s/%s: %w", src.Module, base, err)
			}
			key := num + "_" + dir
			if prev, ok := out[key]; ok && prev.name != base {
				return nil, fmt.Errorf("migrate: 编号 %s 重复（%s 与 %s/%s）", num, prev.name, src.Module, base)
			}
			out[key] = &migrationFile{module: src.Module, version: num, kind: kind, dir: dir, name: base, data: data}
		}
	}
	// up/down 配对
	byNum := map[string]map[string]bool{}
	for _, f := range out {
		if byNum[f.version] == nil {
			byNum[f.version] = map[string]bool{}
		}
		byNum[f.version][f.dir] = true
	}
	for num, dirs := range byNum {
		if !dirs["up"] || !dirs["down"] {
			missing := "up"
			if dirs["up"] {
				missing = "down"
			}
			return nil, fmt.Errorf("migrate: 编号 %s 缺 %s 文件", num, missing)
		}
	}
	return out, nil
}

// Status 输出当前迁移水位与各模块已登记 checksum 数（迁移命令 status 子命令；
// 不修改任何状态）。
func (r *Runner) Status(ctx context.Context, w io.Writer) error {
	var version int64
	if err := r.pool.QueryRow(ctx,
		`SELECT COALESCE(MAX(version_id), 0) FROM goose_db_version`).Scan(&version); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgErrUndefinedTable { // 全新库无版本表 → 水位 0
			fmt.Fprintln(w, "watermark: 0")
			return nil
		}
		return fmt.Errorf("migrate: status: %w", err)
	}
	fmt.Fprintf(w, "watermark: %d\n", version)
	for _, src := range r.opts.Sources {
		var n int
		if err := r.pool.QueryRow(ctx,
			`SELECT count(*) FROM migration_checksums WHERE module = $1`, src.Module).Scan(&n); err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == pgErrUndefinedTable {
				n = 0 // checksum 表未建（全新库）→ 无记录（对照上方水位表 pgErrUndefinedTable 惯例）
			} else {
				return fmt.Errorf("migrate: status %s: %w", src.Module, err)
			}
		}
		fmt.Fprintf(w, "  %-12s applied=%d\n", src.Module, n)
	}
	return nil
}

// Up 执行全部待应用迁移（版本升序）。
// 顺序：校验 → checksum 比对（已应用但内容变更即失败）→ goose 引擎执行 → 记录 checksum。
func (r *Runner) Up(ctx context.Context) error {
	files, err := r.collect()
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return nil
	}

	if err := r.ensureChecksumTable(ctx); err != nil {
		return err
	}
	// checksum 比对：up 文件为粒度
	type pending struct {
		f   *migrationFile
		sum string
	}
	var toApply []pending
	for _, f := range files {
		if f.dir != "up" {
			continue
		}
		sum := sha256Hex(f.data)
		applied, stored, err := r.appliedChecksum(ctx, f.module, f.version)
		if err != nil {
			return err
		}
		if applied {
			if stored != sum {
				return fmt.Errorf(
					"migrate: %s/%s 已应用但内容变更（checksum 不一致；已应用迁移不可修改，docs/rules/migrations.md）stored=%s current=%s",
					f.module, f.name, stored[:12], sum[:12])
			}
			continue
		}
		toApply = append(toApply, pending{f: f, sum: sum})
	}
	if len(toApply) == 0 {
		return nil
	}
	sort.Slice(toApply, func(i, j int) bool { return toApply[i].f.version < toApply[j].f.version })

	// goose 引擎：全部启用模块的迁移合并进单一 MapFS（5 位号段全局唯一，无跨模块冲突）
	mapFS := fstest.MapFS{}
	for _, p := range toApply {
		mapFS[p.f.name] = &fstest.MapFile{Data: p.f.data}
	}
	sqlDB, err := r.openStdlib()
	if err != nil {
		return err
	}
	defer sqlDB.Close()
	// AllowOutofOrder：多包共享库 + 各包只应用自己号段的模型下，残留的他包
	// 高版本行会卡死本包低版本（实证：ipac 12620 残留卡死 identity
	// 11001）——乱序是正常态。完整性防线=Runner checksum 门禁（已应用但内容
	// 变更即失败）+ check-migrations 号段归属 CI，语义不弱化。
	provider, err := goose.NewProvider(goose.DialectPostgres, sqlDB, mapFS,
		goose.WithAllowOutofOrder(true))
	if err != nil {
		return fmt.Errorf("migrate: new provider: %w", err)
	}
	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("migrate: up: %w", err)
	}
	for _, p := range toApply {
		if err := r.recordChecksum(ctx, p.f.module, p.f.version, p.sum); err != nil {
			return err
		}
	}
	return nil
}

// —— checksum 表（module, version, sha256） ——

const checksumSchema = `CREATE TABLE IF NOT EXISTS migration_checksums (
	module TEXT NOT NULL,
	version TEXT NOT NULL,
	sha256 TEXT NOT NULL,
	applied_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	PRIMARY KEY (module, version)
)`

func (r *Runner) ensureChecksumTable(ctx context.Context) error {
	if _, err := r.pool.Exec(ctx, checksumSchema); err != nil {
		return fmt.Errorf("migrate: ensure checksum table: %w", err)
	}
	return nil
}

func (r *Runner) appliedChecksum(ctx context.Context, module, version string) (applied bool, stored string, err error) {
	row := r.pool.QueryRow(ctx,
		`SELECT sha256 FROM migration_checksums WHERE module = $1 AND version = $2`, module, version)
	var s string
	if err := row.Scan(&s); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, "", nil
		}
		return false, "", fmt.Errorf("migrate: query checksum %s/%s: %w", module, version, err)
	}
	return true, s, nil
}

func (r *Runner) recordChecksum(ctx context.Context, module, version, sum string) error {
	if _, err := r.pool.Exec(ctx,
		`INSERT INTO migration_checksums (module, version, sha256) VALUES ($1, $2, $3)
		 ON CONFLICT (module, version) DO UPDATE SET sha256 = EXCLUDED.sha256, applied_at = now()`,
		module, version, sum); err != nil {
		return fmt.Errorf("migrate: record checksum %s/%s: %w", module, version, err)
	}
	return nil
}

func (r *Runner) openStdlib() (*sql.DB, error) {
	// goose Provider 只接受 database/sql；pgx stdlib 是官方桥接
	return stdlib.OpenDB(*r.pool.Config().ConnConfig), nil
}

// sha256Hex 计算文件内容指纹（migration_checksums）。
func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
