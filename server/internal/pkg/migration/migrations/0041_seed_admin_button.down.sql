-- 0045_seed_admin_button.down.sql
-- 回滚 0045：删除种子初始化的按钮（按 code 精确匹配）
DELETE FROM admin_button
WHERE code IN (
    -- 管理员管理
    'user:add', 'user:edit', 'user:delete',
    -- 终端用户管理
    'user:query',
    -- 角色管理
    'role:add', 'role:edit', 'role:delete', 'role:auth',
    -- 菜单管理
    'menu:add', 'menu:edit', 'menu:delete',
    -- 按钮管理
    'button:add', 'button:edit', 'button:delete',
    -- API 管理
    'api:add', 'api:edit', 'api:delete',
    -- 字典管理
    'dict:add', 'dict:edit', 'dict:delete',
    -- IP 访问控制
    'ip:access:query', 'ip:access:add', 'ip:access:edit', 'ip:access:delete', 'ip:access:batchDelete',
    -- 内容分类
    'content:category:add', 'content:category:edit', 'content:category:delete',
    -- 文章管理
    'content:article:add', 'content:article:edit', 'content:article:delete',
    'content:article:publish', 'content:article:unpublish', 'content:article:top',
    -- Banner 管理
    'content:banner-group:add', 'content:banner-group:edit', 'content:banner-group:delete',
    'content:banner:add', 'content:banner:edit', 'content:banner:delete',
    -- 存储配置
    'storage:add', 'storage:edit', 'storage:delete', 'storage:test', 'storage:default',
    -- 上传记录
    'ops:upload-record:delete', 'ops:upload-record:batch-delete',
    -- 应用管理
    'open:app:query', 'open:app:add', 'open:app:edit', 'open:app:resetSecret',
    'open:app:delete', 'open:app:bindStorage',
    -- API 管理（开放平台）
    'open:api:query', 'open:api:add', 'open:api:edit', 'open:api:delete',
    -- 接口权限
    'open:scope:query', 'open:scope:add', 'open:scope:edit', 'open:scope:delete', 'open:scope:bindApis',
    -- 开放平台日志
    'open:log:query',
    -- 消息中心
    'message:template:query', 'message:template:add', 'message:template:edit',
    'message:template:delete', 'message:template:test',
    'message:send:sms', 'message:send:sms:query',
    'message:send:email', 'message:send:email:query',
    'message:send:internal', 'message:send:internal:query',
    'message:record:query', 'message:record:detail', 'message:record:retry', 'message:record:delete',
    -- 运维日志
    'ops:operation-log:query', 'ops:operation-log:delete', 'ops:operation-log:batch-delete',
    -- 错误日志
    'ops:error-log:query', 'ops:error-log:resolve', 'ops:error-log:delete', 'ops:error-log:batch-delete',
    -- 系统配置
    'email:test'
);
