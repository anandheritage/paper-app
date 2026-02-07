-- Add admin flag to users table
ALTER TABLE users ADD COLUMN IF NOT EXISTS is_admin BOOLEAN NOT NULL DEFAULT false;

-- Set the first admin
UPDATE users SET is_admin = true WHERE email = 'anandkrshawheritage@gmail.com';
