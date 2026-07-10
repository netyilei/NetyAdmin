-- 0063_add_composite_indexes.down.sql
-- 回滚 0063 迁移添加的所有联合索引

DROP INDEX IF EXISTS idx_admin_role_status;
DROP INDEX IF EXISTS idx_banner_group_position_status;
DROP INDEX IF EXISTS idx_operation_log_admin_time;
DROP INDEX IF EXISTS idx_msg_templates_channel_status;
DROP INDEX IF EXISTS idx_content_category_status;
DROP INDEX IF EXISTS idx_admin_menu_status_order;
DROP INDEX IF EXISTS idx_users_status;
DROP INDEX IF EXISTS idx_admin_user_status;
DROP INDEX IF EXISTS idx_content_article_cat_status;
DROP INDEX IF EXISTS idx_ipac_status_expired;
DROP INDEX IF EXISTS idx_storage_config_default_status;
DROP INDEX IF EXISTS idx_task_logs_name_id;
DROP INDEX IF EXISTS idx_banner_item_group_status;
DROP INDEX IF EXISTS idx_dict_data_code_status;
DROP INDEX IF EXISTS idx_open_log_app_time;
DROP INDEX IF EXISTS idx_msg_records_user_status;
DROP INDEX IF EXISTS idx_msg_records_channel_time;
DROP INDEX IF EXISTS idx_content_article_status_scheduled;
DROP INDEX IF EXISTS idx_content_article_status_pub_time;
DROP INDEX IF EXISTS idx_msg_internal_record_id;
DROP INDEX IF EXISTS idx_admin_menu_parent_id;
DROP INDEX IF EXISTS idx_admin_button_menu_id;
DROP INDEX IF EXISTS idx_admin_api_menu_id;