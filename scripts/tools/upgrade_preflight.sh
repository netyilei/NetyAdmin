#!/usr/bin/env bash
# 基座升级预检（目标版本 2.0.1；检查项自 2.0.0 起未变）：
#   Redis 可达且 >= 8.1 / PostgreSQL 可达 / 当前迁移版本 / aes_key 已配置非默认。
# 用法：bash scripts/tools/upgrade_preflight.sh
# 退出码：0=全部通过；1=有 FAIL（禁止升级）。
set -u
REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
CONFIG="$REPO_ROOT/server/config.yaml"
FAIL=0
PASS=0

ok()   { echo "[PASS] $*"; PASS=$((PASS+1)); }
fail() { echo "[FAIL] $*"; FAIL=$((FAIL+1)); }
info() { echo "[INFO] $*"; }

[ -f "$CONFIG" ] || { echo "[FAIL] 未找到 $CONFIG"; exit 1; }

# 解析 config.yaml：python 逐行输出 9 个字段（空行=空值，行分隔固定 LF）。
# 不用空格分隔——read 的 IFS 会合并连续空白导致空字段错位；
# 用 sys.stdout.buffer 写 UTF-8 原始字节——绕开 Windows 文本模式的 CRLF 转换。
parse_cfg() {
  python - "$CONFIG" << 'PYEOF'
import sys
path = sys.argv[1]
nl = chr(10)
try:
    import yaml
    c = yaml.safe_load(open(path, encoding="utf-8")) or {}
    r, d, s = c.get("redis") or {}, c.get("database") or {}, c.get("security") or {}
    vals = [r.get("host", "localhost"), r.get("port", 6379), r.get("db", 0),
            r.get("password") or "",
            d.get("host", "localhost"), d.get("port", 5432), d.get("user", "postgres"),
            d.get("dbname", "netyadmin"), s.get("aes_key", "") or ""]
except Exception:
    import re
    raw = open(path, encoding="utf-8").read()
    def pick(sec, key):
        m = re.search("^" + sec + r":\s*$(.*?)(?=^\S)", raw, re.S | re.M)
        if not m:
            return ""
        mm = re.search(r"^\s+" + key + r":\s*\"?([^\"\n#]*)\"?", m.group(1), re.M)
        return mm.group(1).strip() if mm else ""
    vals = [pick("redis", "host") or "localhost", pick("redis", "port") or "6379",
            pick("redis", "db") or "0", pick("redis", "password"),
            pick("database", "host") or "localhost", pick("database", "port") or "5432",
            pick("database", "user") or "postgres", pick("database", "dbname") or "netyadmin",
            pick("security", "aes_key")]
out = nl.join(str(v) for v in vals) + nl
sys.stdout.buffer.write(out.encode("utf-8"))
PYEOF
}

{
  read -r R_HOST
  read -r R_PORT
  read -r R_DB
  read -r R_PASS
  read -r PG_HOST
  read -r PG_PORT
  read -r PG_USER
  read -r PG_DB
  read -r AES_KEY
} < <(parse_cfg)

[ -n "${R_HOST:-}" ] || { echo "[FAIL] config.yaml 解析失败"; exit 1; }

echo "=== 基座升级预检（目标版本 2.0.1）==="
echo "[INFO] Redis=$R_HOST:$R_PORT db=$R_DB  PG=$PG_HOST:$PG_PORT user=$PG_USER db=$PG_DB"

# 1. Redis 可达 + 版本 >= 8.1
RCLI=(redis-cli -h "$R_HOST" -p "$R_PORT" -n "$R_DB")
[ -n "${R_PASS:-}" ] && RCLI+=(-a "$R_PASS" --no-auth-warning)
if [ "$("${RCLI[@]}" ping 2>/dev/null)" = "PONG" ]; then
  ok "Redis 可达"
else
  fail "Redis 不可达（$R_HOST:$R_PORT）；基线要求 Redis >= 8.1（自 2.0.0 起）"
fi
RVER=$("${RCLI[@]}" info server 2>/dev/null | sed -n 's/^redis_version://p' | tr -d '[:space:]')
if [ -n "$RVER" ]; then
  MAJ=${RVER%%.*}
  REST=${RVER#*.}
  MIN=${REST%%.*}
  if [ "$MAJ" -gt 8 ] || { [ "$MAJ" -eq 8 ] && [ "$MIN" -ge 1 ]; }; then
    ok "Redis 版本 $RVER >= 8.1"
  else
    fail "Redis 版本 $RVER < 8.1（基线要求 8.1+，自 2.0.0 起）"
  fi
else
  fail "无法读取 Redis 版本（redis-cli 是否可用？）"
fi

# 2. PG 可达 + 迁移版本
if command -v psql >/dev/null 2>&1; then
  if [ -n "$(psql -h "$PG_HOST" -p "$PG_PORT" -U "$PG_USER" -d "$PG_DB" -t -A -c 'SELECT 1;' 2>/dev/null)" ]; then
    ok "PostgreSQL 可达"
    MV=$(psql -h "$PG_HOST" -p "$PG_PORT" -U "$PG_USER" -d "$PG_DB" -t -A \
         -c "SELECT max(version::bigint) FROM schema_migrations;" 2>/dev/null | tr -d '[:space:]')
    info "当前迁移版本：${MV:-未知}（升级启动后自动执行到 0061）"
  else
    fail "PostgreSQL 不可达（$PG_HOST:$PG_PORT user=$PG_USER db=$PG_DB）"
  fi
else
  fail "psql 不可用，无法校验 PG（请安装或手工确认）"
fi

# 3. aes_key：必须已配置且非模板默认值（PII 密文派生密钥，升级后禁止更换）
case "${AES_KEY:-}" in
  "")
    fail "aes_key 为空——PII 加密不可用" ;;
  netyadmin-aes-key-32-chars-long!|CHANGE_ME*)
    fail "aes_key 仍为模板默认值——请先设为生产密钥并单独备份（PII 密文派生自它，升级后禁止更换）" ;;
  *)
    ok "aes_key 已配置（升级后禁止更换，请单独备份）" ;;
esac

echo
if [ "$FAIL" -gt 0 ]; then
  echo "预检结果：FAIL（$FAIL 项未通过，禁止升级）"
  exit 1
fi
echo "预检结果：PASS（$PASS 项通过）"
echo "下一步：1) pg_dump 备份  2) 停止【全部】节点（不支持混合版本）  3) 部署新版本启动"
echo "       4) 启动日志出现 pii backfill completed（v1 升级时）  5) /health 返回所部署版本（当前发布 version=2.0.1）"
echo "详见 docs/upgrade-guide.md"
