-- Migration 002: Add citation_count column for better search sorting
-- This avoids having to extract citation counts from JSONB metadata at query time

ALTER TABLE papers ADD COLUMN IF NOT EXISTS citation_count INTEGER DEFAULT 0;

-- Index for sorting by citations (descending — most cited first)
CREATE INDEX IF NOT EXISTS idx_papers_citation_count ON papers(citation_count DESC);
