-- Migration 003: Optimize schema for bulk arXiv data (~2.4M papers)

-- Add categories column for arXiv categories (e.g., 'cs.AI', 'hep-ph')
ALTER TABLE papers ADD COLUMN IF NOT EXISTS categories TEXT[];

-- Index for sorting by date
CREATE INDEX IF NOT EXISTS idx_papers_published_date ON papers(published_date DESC NULLS LAST);

-- Composite index for source + date
CREATE INDEX IF NOT EXISTS idx_papers_source_date ON papers(source, published_date DESC NULLS LAST);

-- Composite index for source + citations
CREATE INDEX IF NOT EXISTS idx_papers_source_citations ON papers(source, citation_count DESC);

-- GIN index for category filtering (WHERE 'cs.AI' = ANY(categories))
CREATE INDEX IF NOT EXISTS idx_papers_categories ON papers USING GIN(categories);

-- Refresh table statistics for better query planning
ANALYZE papers;
