// +build ignore

package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	_ "github.com/lib/pq"
)

func main() {
	dsn := "host=localhost port=5432 user=postgres password=123456 dbname=so sslmode=disable"

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("连接数据库失败: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("数据库 Ping 失败: %v", err)
	}
	fmt.Println("✅ 数据库连接成功")

	// 检查 schema_migrations 表
	var tableExists bool
	err = db.QueryRow("SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'schema_migrations')").Scan(&tableExists)
	if err != nil {
		log.Fatalf("检查表失败: %v", err)
	}

	if tableExists {
		var version int
		var dirty bool
		err = db.QueryRow("SELECT version, dirty FROM schema_migrations").Scan(&version, &dirty)
		if err != nil {
			if err == sql.ErrNoRows {
				fmt.Println("⚠️ schema_migrations 表存在但为空")
			} else {
				log.Fatalf("查询 schema_migrations 失败: %v", err)
			}
		} else {
			fmt.Printf("📊 schema_migrations: version=%d, dirty=%v\n", version, dirty)
			if dirty {
				fmt.Println("⚠️ 脏状态！正在修复...")
				_, err = db.Exec("UPDATE schema_migrations SET dirty = false WHERE version = $1", version)
				if err != nil {
					log.Fatalf("修复失败: %v", err)
				}
				fmt.Println("✅ 脏状态已修复")
			}
		}
	} else {
		fmt.Println("✅ schema_migrations 表不存在，golang-migrate 启动时会自动创建")
	}

	// 检查所有需要的基础表
	tables := []string{
		"admin_user", "admin_role", "admin_menu", "admin_api", "admin_button",
		"admin_user_roles", "admin_role_menus", "admin_role_apis", "admin_role_buttons",
		"admin_operation_log", "admin_error_log",
		"users", "user_token_hashes",
		"sys_dict_type", "sys_dict_data", "sys_configs", "captcha_tokens",
		"sys_apps", "sys_open_apis", "sys_open_platform_logs",
		"content_category", "content_article", "content_banner_group", "content_banner_item",
		"msg_templates", "msg_records", "msg_internal",
		"storage_config", "upload_record",
		"sys_ip_access_control", "sys_task_logs",
	}
	missing := 0
	for _, table := range tables {
		var exists bool
		db.QueryRow("SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = $1)", table).Scan(&exists)
		if !exists {
			fmt.Printf("❌ 缺少表: %s\n", table)
			missing++
		}
	}
	if missing == 0 {
		fmt.Println("✅ 全部 30+ 张表都存在，数据库状态正常")
	} else {
		fmt.Printf("⚠️ 缺少 %d 张表，需要运行迁移\n", missing)
	}

	os.Exit(0)
}
