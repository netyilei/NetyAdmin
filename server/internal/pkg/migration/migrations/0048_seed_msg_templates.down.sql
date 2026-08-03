-- 0052_seed_msg_templates.down.sql
-- 回滚 0052：删除种子初始化的消息模板（按 code 精确匹配）
DELETE FROM msg_templates
WHERE code IN (
    'verify_code_sms',
    'verify_code_email',
    'welcome_internal',
    'reset_password_email',
    'system_notice_internal'
);
