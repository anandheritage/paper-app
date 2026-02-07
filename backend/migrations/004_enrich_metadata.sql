-- Migration 004: Enrich paper metadata for OAI-PMH harvesting + category browsing

-- Add new metadata columns
ALTER TABLE papers ADD COLUMN IF NOT EXISTS primary_category VARCHAR(50);
ALTER TABLE papers ADD COLUMN IF NOT EXISTS categories TEXT[];
ALTER TABLE papers ADD COLUMN IF NOT EXISTS doi VARCHAR(255);
ALTER TABLE papers ADD COLUMN IF NOT EXISTS journal_ref TEXT;
ALTER TABLE papers ADD COLUMN IF NOT EXISTS comments TEXT;
ALTER TABLE papers ADD COLUMN IF NOT EXISTS license TEXT;
ALTER TABLE papers ADD COLUMN IF NOT EXISTS updated_date DATE;

-- Index for category-based filtering
CREATE INDEX IF NOT EXISTS idx_papers_primary_category ON papers(primary_category);
CREATE INDEX IF NOT EXISTS idx_papers_categories ON papers USING GIN(categories);
CREATE INDEX IF NOT EXISTS idx_papers_doi ON papers(doi) WHERE doi IS NOT NULL AND doi != '';
CREATE INDEX IF NOT EXISTS idx_papers_published_date ON papers(published_date DESC NULLS LAST);
CREATE INDEX IF NOT EXISTS idx_papers_citation_count ON papers(citation_count DESC);

-- Harvest checkpoint table: tracks incremental OAI-PMH harvesting progress
CREATE TABLE IF NOT EXISTS harvest_checkpoints (
    id SERIAL PRIMARY KEY,
    set_name VARCHAR(100) NOT NULL DEFAULT '_all',
    last_datestamp VARCHAR(30),
    last_resumption_token TEXT,
    total_harvested BIGINT DEFAULT 0,
    status VARCHAR(20) DEFAULT 'idle', -- idle, running, completed, failed
    started_at TIMESTAMP,
    completed_at TIMESTAMP,
    updated_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(set_name)
);

-- Composite index for efficient search+sort combos
CREATE INDEX IF NOT EXISTS idx_papers_cat_date ON papers(primary_category, published_date DESC NULLS LAST);
CREATE INDEX IF NOT EXISTS idx_papers_cat_citations ON papers(primary_category, citation_count DESC);
