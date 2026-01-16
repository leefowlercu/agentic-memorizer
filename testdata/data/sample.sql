-- Sample SQL file for testing chunkers.

-- Create users table
CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    role VARCHAR(50) DEFAULT 'user',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create posts table
CREATE TABLE IF NOT EXISTS posts (
    id SERIAL PRIMARY KEY,
    title VARCHAR(255) NOT NULL,
    content TEXT,
    author_id INTEGER REFERENCES users(id),
    published BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create index on posts author
CREATE INDEX idx_posts_author ON posts(author_id);

-- Create index on posts published
CREATE INDEX idx_posts_published ON posts(published);

-- Insert sample data
INSERT INTO users (name, email, role) VALUES
    ('Alice', 'alice@example.com', 'admin'),
    ('Bob', 'bob@example.com', 'user'),
    ('Charlie', 'charlie@example.com', 'user');

INSERT INTO posts (title, content, author_id, published) VALUES
    ('First Post', 'This is the first post content.', 1, TRUE),
    ('Draft Post', 'This is a draft.', 1, FALSE),
    ('User Post', 'A post by a regular user.', 2, TRUE);

-- Select all published posts with authors
SELECT p.id, p.title, u.name AS author
FROM posts p
JOIN users u ON p.author_id = u.id
WHERE p.published = TRUE
ORDER BY p.created_at DESC;
