-- =============================================
-- Content Module - Tables
-- =============================================

BEGIN;

-- 内容分类表
CREATE TABLE IF NOT EXISTS content_category (
    id BIGSERIAL PRIMARY KEY,
    parent_id BIGINT DEFAULT 0,
    name VARCHAR(50) NOT NULL,
    code VARCHAR(50),
    icon VARCHAR(100),
    content_type VARCHAR(20) DEFAULT 'richtext',
    storage_config_id BIGINT DEFAULT NULL,
    sort INT DEFAULT 0,
    status CHAR(1) DEFAULT '1',
    remark TEXT,
    created_by BIGINT,
    updated_by BIGINT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at BIGINT DEFAULT 0
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_content_category_code ON content_category(code) WHERE deleted_at = 0;
CREATE INDEX IF NOT EXISTS idx_content_category_parent ON content_category(parent_id);

-- 文章表
CREATE TABLE IF NOT EXISTS content_article (
    id BIGSERIAL PRIMARY KEY,
    category_id BIGINT NOT NULL,
    title VARCHAR(200) NOT NULL,
    title_color VARCHAR(20) DEFAULT '#333333',
    cover_image VARCHAR(500),
    summary VARCHAR(500),
    content TEXT,
    content_type VARCHAR(20) DEFAULT 'richtext',
    author VARCHAR(50),
    source VARCHAR(100),
    keywords VARCHAR(200),
    tags VARCHAR(200),
    is_top BOOLEAN DEFAULT FALSE,
    top_sort INT DEFAULT 0,
    is_hot BOOLEAN DEFAULT FALSE,
    is_recommend BOOLEAN DEFAULT FALSE,
    allow_comment BOOLEAN DEFAULT TRUE,
    view_count INT DEFAULT 0,
    like_count INT DEFAULT 0,
    comment_count INT DEFAULT 0,
    publish_status VARCHAR(20) DEFAULT 'draft',
    published_at TIMESTAMP,
    scheduled_at TIMESTAMP,
    sort INT DEFAULT 0,
    status CHAR(1) DEFAULT '1',
    remark TEXT,
    created_by BIGINT,
    updated_by BIGINT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at BIGINT DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_content_article_category ON content_article(category_id);
CREATE INDEX IF NOT EXISTS idx_content_article_publish_status ON content_article(publish_status);
CREATE INDEX IF NOT EXISTS idx_content_article_published_at ON content_article(published_at);
CREATE INDEX IF NOT EXISTS idx_content_article_scheduled_at ON content_article(scheduled_at);
CREATE INDEX IF NOT EXISTS idx_content_article_is_top ON content_article(is_top, top_sort);

-- Banner组表
CREATE TABLE IF NOT EXISTS content_banner_group (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    code VARCHAR(50) NOT NULL,
    description VARCHAR(255),
    position VARCHAR(50),
    width INT,
    height INT,
    max_items INT DEFAULT 10,
    auto_play BOOLEAN DEFAULT TRUE,
    interval INT DEFAULT 5000,
    storage_config_id BIGINT DEFAULT NULL,
    sort INT DEFAULT 0,
    status CHAR(1) DEFAULT '1',
    remark TEXT,
    created_by BIGINT,
    updated_by BIGINT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at BIGINT DEFAULT 0
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_content_banner_group_code ON content_banner_group(code) WHERE deleted_at = 0;

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
