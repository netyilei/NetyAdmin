-- 默认测试用户种子数据 (密码均为 123456)
-- bcrypt hash 经 bcrypt.GenerateFromPassword("123456", DefaultCost) 生成，可被 bcrypt.CompareHashAndPassword 验证通过
INSERT INTO users (id, username, password, nickname, phone, email, gender, status) VALUES
('01JQTESTUSER00001', 'testuser1', '$2a$10$Pt4W8dN05Y6ZniY6RfCYe.FgAEMAbg3pMa7UhFh7aFmO1i2WdAlBC', '测试用户1', '13800000001', 'test1@netyadmin.com', '1', '1'),
('01JQTESTUSER00002', 'testuser2', '$2a$10$Pt4W8dN05Y6ZniY6RfCYe.FgAEMAbg3pMa7UhFh7aFmO1i2WdAlBC', '测试用户2', '13800000002', 'test2@netyadmin.com', '2', '1'),
('01JQTESTUSER00003', 'testuser3', '$2a$10$Pt4W8dN05Y6ZniY6RfCYe.FgAEMAbg3pMa7UhFh7aFmO1i2WdAlBC', '测试用户3', '13800000003', 'test3@netyadmin.com', '0', '1')
ON CONFLICT (username) WHERE deleted_at = 0 DO NOTHING;

