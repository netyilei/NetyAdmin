-- 0063_add_composite_indexes.up.sql
-- 为高频查询添加联合索引，优化查询性能
-- 基于所有 Repository 层查询模式的完整分析

-- ============================================================
-- 一、关键缺失索引（CRITICAL — 无任何索引，查询必走全表扫描）
-- ============================================================

-- 1. admin_api.menu_id：每次 API 管理页加载、菜单删除级联查询
CREATE INDEX IF NOT EXISTS idx_admin_api_menu_id ON admin_api(menu_id);

-- 2. admin_button.menu_id：每次按钮管理页加载、菜单删除级联查询
CREATE INDEX IF NOT EXISTS idx_admin_button_menu_id ON admin_button(menu_id);

-- 3. admin_menu.parent_id：每次菜单树渲染、菜单管理查询
CREATE INDEX IF NOT EXISTS idx_admin_menu_parent_id ON admin_menu(parent_id) WHERE deleted_at = 0;

-- 4. msg_internal.msg_record_id：所有站内信 JOIN 查询的核心关联列
CREATE INDEX IF NOT EXISTS idx_msg_internal_record_id ON msg_internal(msg_record_id);

-- ============================================================
-- 二、高频查询联合索引（HIGH）
-- ============================================================

-- 5. content_article: 客户端文章列表（最高频的前台查询）
-- WHERE publish_status = 'published' ORDER BY published_at DESC
CREATE INDEX IF NOT EXISTS idx_content_article_status_pub_time ON content_article(publish_status, published_at DESC);

-- 6. content_article: 定时发布任务（每分钟执行）
-- WHERE publish_status = 'scheduled' AND scheduled_at <= ?
CREATE INDEX IF NOT EXISTS idx_content_article_status_scheduled ON content_article(publish_status, scheduled_at);

-- 7. msg_records: 消息记录列表（按 channel 过滤 + 时间排序）
-- WHERE channel = ? ORDER BY created_at DESC
CREATE INDEX IF NOT EXISTS idx_msg_records_channel_time ON msg_records(channel, created_at DESC);

-- 8. msg_records: 客户端用户消息列表
-- WHERE user_id = ? AND status = ?
CREATE INDEX IF NOT EXISTS idx_msg_records_user_status ON msg_records(user_id, status);

-- 9. sys_open_platform_logs: 日志管理（按应用 + 时间范围查询）
-- WHERE app_id = ? AND created_at >= ? AND created_at <= ? ORDER BY created_at DESC
CREATE INDEX IF NOT EXISTS idx_open_log_app_time ON sys_open_platform_logs(app_id, created_at DESC);

-- 10. sys_dict_data: 字典数据加载
-- WHERE dict_code = ? AND status = ? ORDER BY order_by ASC
CREATE INDEX IF NOT EXISTS idx_dict_data_code_status ON sys_dict_data(dict_code, status);

-- 11. content_banner_item: 客户端 Banner 加载
-- WHERE group_id = ? AND status = ? ORDER BY sort ASC
CREATE INDEX IF NOT EXISTS idx_banner_item_group_status ON content_banner_item(group_id, status);

-- 12. sys_task_logs: 获取最新任务日志
-- WHERE name = ? ORDER BY id DESC LIMIT 1
CREATE INDEX IF NOT EXISTS idx_task_logs_name_id ON sys_task_logs(name, id DESC);

-- 13. storage_config: 获取默认存储配置
-- WHERE is_default = ? AND status = ?
CREATE INDEX IF NOT EXISTS idx_storage_config_default_status ON storage_config(is_default, status) WHERE deleted_at = 0;

-- 14. sys_ip_access_control: 获取所有生效规则
-- WHERE status = ? AND (expired_at IS NULL OR expired_at > NOW())
CREATE INDEX IF NOT EXISTS idx_ipac_status_expired ON sys_ip_access_control(status, expired_at) WHERE deleted_at = 0;

-- 15. content_article: 管理后台按分类+状态筛选
-- WHERE category_id = ? AND publish_status = ?
CREATE INDEX IF NOT EXISTS idx_content_article_cat_status ON content_article(category_id, publish_status);

-- ============================================================
-- 三、推荐补充索引（MEDIUM — 提升管理后台查询体验）
-- ============================================================

-- 16. admin_user: 管理员列表按状态筛选
CREATE INDEX IF NOT EXISTS idx_admin_user_status ON admin_user(status) WHERE deleted_at = 0;

-- 17. users: 用户列表按状态筛选
CREATE INDEX IF NOT EXISTS idx_users_status ON users(status) WHERE deleted_at = 0;

-- 18. admin_menu: 菜单列表按状态+排序
-- WHERE status = ? ORDER BY order_by ASC, id ASC
CREATE INDEX IF NOT EXISTS idx_admin_menu_status_order ON admin_menu(status, order_by, id) WHERE deleted_at = 0;

-- 19. content_category: 分类列表按状态筛选
CREATE INDEX IF NOT EXISTS idx_content_category_status ON content_category(status) WHERE deleted_at = 0;

-- 20. msg_templates: 模板列表按渠道+状态筛选
CREATE INDEX IF NOT EXISTS idx_msg_templates_channel_status ON msg_templates(channel, status) WHERE deleted_at = 0;

-- 21. admin_operation_log: 操作日志按管理员+时间查询
CREATE INDEX IF NOT EXISTS idx_operation_log_admin_time ON admin_operation_log(admin_id, created_at DESC);

-- 22. content_banner_group: Banner 组按位置+状态查询
CREATE INDEX IF NOT EXISTS idx_banner_group_position_status ON content_banner_group(position, status) WHERE deleted_at = 0;

-- 23. admin_role: 角色列表按状态筛选
CREATE INDEX IF NOT EXISTS idx_admin_role_status ON admin_role(status) WHERE deleted_at = 0;