package migration

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gorm.io/gorm"
)

type Migrator struct {
	db  *gorm.DB
	dir string
}

func NewMigrator(db *gorm.DB, dir string) *Migrator {
	return &Migrator{
		db:  db,
		dir: dir,
	}
}

func (m *Migrator) Run() error {
	allFiles, err := m.getMigrationFiles()
	if err != nil {
		return fmt.Errorf("读取迁移文件失败: %w", err)
	}

	if len(allFiles) == 0 {
		log.Println("[迁移] 没有找到迁移文件")
		return nil
	}

	var tableFiles []string
	var dataFiles []string
	var constraintFiles []string
	var otherFiles []string

	for _, file := range allFiles {
		base := filepath.Base(file)
		// 获取相对于迁移根目录的路径
		rel, _ := filepath.Rel(m.dir, file)
		relDir := filepath.ToSlash(filepath.Dir(rel)) // 统一使用正斜杠处理

		// 优先根据路径中的文件夹判断，其次根据文件前缀判断
		if strings.HasPrefix(relDir, "table") || strings.HasPrefix(base, "table_") {
			tableFiles = append(tableFiles, file)
		} else if strings.HasPrefix(relDir, "constraint") || strings.HasPrefix(base, "constraint_") || strings.HasPrefix(base, "fk_") {
			constraintFiles = append(constraintFiles, file)
		} else if strings.HasPrefix(relDir, "data") || strings.HasPrefix(base, "data_") {
			dataFiles = append(dataFiles, file)
		} else {
			otherFiles = append(otherFiles, file)
		}
	}

	sort.Strings(tableFiles)
	sort.Strings(dataFiles)
	sort.Strings(constraintFiles)
	sort.Strings(otherFiles)

	// 执行顺序: table_ -> constraint_ -> data_ -> other
	files := make([]string, 0, len(allFiles))
	files = append(files, tableFiles...)
	files = append(files, constraintFiles...)
	files = append(files, dataFiles...)
	files = append(files, otherFiles...)

	// 使用事务包裹所有迁移操作
	return m.db.Transaction(func(tx *gorm.DB) error {
		for _, file := range files {
			name := strings.TrimSuffix(filepath.Base(file), ".sql")

			content, err := os.ReadFile(file)
			if err != nil {
				return fmt.Errorf("读取迁移文件 %s 失败: %w", file, err)
			}

			if err := tx.Exec(string(content)).Error; err != nil {
				return fmt.Errorf("执行迁移 %s 失败: %w", name, err)
			}

			log.Printf("[迁移] %-30s | 完成 √", name)
		}
		return nil
	})
}

func (m *Migrator) getMigrationFiles() ([]string, error) {
	var files []string

	err := filepath.Walk(m.dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".sql") {
			files = append(files, path)
		}
		return nil
	})

	return files, err
}
