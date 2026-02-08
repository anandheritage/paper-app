-- Login events table for monitoring usage
CREATE TABLE IF NOT EXISTS login_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    auth_method VARCHAR(20) NOT NULL,  -- 'email', 'google', 'token_refresh'
    ip_address VARCHAR(45),
    user_agent TEXT,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_login_events_user_id ON login_events(user_id);
CREATE INDEX IF NOT EXISTS idx_login_events_created_at ON login_events(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_login_events_auth_method ON login_events(auth_method);

-- Add last_login_at to users for quick access
ALTER TABLE users ADD COLUMN IF NOT EXISTS last_login_at TIMESTAMP;
