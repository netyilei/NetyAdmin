BEGIN;

-- Banner项表
CREATE TABLE IF NOT EXISTS content_banner_item (
    id BIGSERIAL PRIMARY KEY,
    group_id BIGINT NOT NULL,
    title VARCHAR(200) NOT NULL,
    subtitle VARCHAR(200),
    image_url VARCHAR(500) NOT NULL,
    image_alt VARCHAR(200),
    link_type VARCHAR(20) DEFAULT 'none',
    link_url VARCHAR(500),
    link_article_id BIGINT,
    content TEXT,
    custom_params TEXT,
    sort INT DEFAULT 0,
    start_time TIMESTAMP,
    end_time TIMESTAMP,
    view_count INT DEFAULT 0,
    click_count INT DEFAULT 0,
    status CHAR(1) DEFAULT '1',
    created_by BIGINT,
    updated_by BIGINT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at BIGINT DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_content_banner_item_group ON content_banner_item(group_id);
CREATE INDEX IF NOT EXISTS idx_content_banner_item_status ON content_banner_item(status);
CREATE INDEX IF NOT EXISTS idx_content_banner_item_time ON content_banner_item(start_time, end_time);

COMMIT;