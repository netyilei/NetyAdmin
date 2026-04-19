BEGIN;

-- 默认测试用户种子数据 (密码均为 123456)
INSERT INTO users (id, username, password, nickname, phone, email, gender, status) VALUES
('01JQTESTUSER00001', 'testuser1', '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy', '测试用户1', '13800000001', 'test1@netyadmin.com', '1', '1'),
('01JQTESTUSER00002', 'testuser2', '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy', '测试用户2', '13800000002', 'test2@netyadmin.com', '2', '1'),
('01JQTESTUSER00003', 'testuser3', '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy', '测试用户3', '13800000003', 'test3@netyadmin.com', '0', '1')
ON CONFLICT (username) WHERE deleted_at = 0 DO NOTHING;

COMMIT;