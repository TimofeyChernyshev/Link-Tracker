CREATE TABLE links (
    id SERIAL PRIMARY KEY,
    url TEXT UNIQUE NOT NULL,
    updated_at TIMESTAMP DEFAULT NOW(),
    last_checked_at TIMESTAMP
);