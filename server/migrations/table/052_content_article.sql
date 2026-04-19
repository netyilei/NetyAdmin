BEGIN;

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

COMMIT;