-- Add bookmarked_at timestamp for proper bookmark sorting
ALTER TABLE user_papers ADD COLUMN IF NOT EXISTS bookmarked_at TIMESTAMPTZ;

-- Backfill: set bookmarked_at = saved_at for existing bookmarks
UPDATE user_papers SET bookmarked_at = saved_at WHERE is_bookmarked = TRUE AND bookmarked_at IS NULL;
