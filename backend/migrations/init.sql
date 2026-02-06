-- Enable extensions for full-text search
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- Users table
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL DEFAULT '',
    name VARCHAR(100),
    auth_provider VARCHAR(20) NOT NULL DEFAULT 'email',
    provider_id VARCHAR(255),
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Papers table (cached from arXiv/PubMed)
CREATE TABLE IF NOT EXISTS papers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    external_id VARCHAR(100) UNIQUE NOT NULL,
    source VARCHAR(20) NOT NULL,
    title TEXT NOT NULL,
    abstract TEXT,
    authors JSONB,
    published_date DATE,
    pdf_url TEXT,
    metadata JSONB,
    created_at TIMESTAMP DEFAULT NOW(),
    search_vector tsvector GENERATED ALWAYS AS (
        setweight(to_tsvector('english', coalesce(title, '')), 'A') ||
        setweight(to_tsvector('english', coalesce(abstract, '')), 'B')
    ) STORED
);

-- User papers table (bookmarks/saved papers)
CREATE TABLE IF NOT EXISTS user_papers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    paper_id UUID REFERENCES papers(id) ON DELETE CASCADE,
    status VARCHAR(20) DEFAULT 'saved',
    is_bookmarked BOOLEAN DEFAULT false,
    reading_progress INT DEFAULT 0,
    notes TEXT,
    tags JSONB,
    saved_at TIMESTAMP DEFAULT NOW(),
    last_read_at TIMESTAMP,
    UNIQUE(user_id, paper_id)
);

-- Reading sessions table (analytics)
CREATE TABLE IF NOT EXISTS reading_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id),
    paper_id UUID REFERENCES papers(id),
    started_at TIMESTAMP DEFAULT NOW(),
    ended_at TIMESTAMP,
    pages_read INT
);

-- Refresh tokens table
CREATE TABLE IF NOT EXISTS refresh_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    token_hash VARCHAR(255) NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_papers_external_id ON papers(external_id);
CREATE INDEX IF NOT EXISTS idx_papers_source ON papers(source);
CREATE INDEX IF NOT EXISTS idx_papers_search_vector ON papers USING GIN(search_vector);
CREATE INDEX IF NOT EXISTS idx_papers_title_trgm ON papers USING GIN(title gin_trgm_ops);
CREATE INDEX IF NOT EXISTS idx_user_papers_user_id ON user_papers(user_id);
CREATE INDEX IF NOT EXISTS idx_user_papers_paper_id ON user_papers(paper_id);
CREATE INDEX IF NOT EXISTS idx_user_papers_status ON user_papers(status);
CREATE INDEX IF NOT EXISTS idx_user_papers_is_bookmarked ON user_papers(is_bookmarked);
CREATE INDEX IF NOT EXISTS idx_reading_sessions_user_id ON reading_sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user_id ON refresh_tokens(user_id);
CREATE INDEX IF NOT EXISTS idx_users_provider ON users(auth_provider, provider_id);