-- Test script to verify NULL login handling in seed script
-- This creates a small test database with mixed data (valid and NULL logins)

-- Create test database file
-- Run: sqlite3 test_null_sample.db < test_null_logins.sql

DROP TABLE IF EXISTS author_login_cache;

CREATE TABLE author_login_cache (
  author_email TEXT,
  login TEXT,
  resolved_at TIMESTAMP
);

-- Insert 10 valid records
INSERT INTO author_login_cache VALUES
('valid1@example.com', 'user1', '2026-01-01T00:00:00Z'),
('valid2@example.com', 'user2', '2026-01-02T00:00:00Z'),
('valid3@example.com', 'user3', '2026-01-03T00:00:00Z'),
('valid4@example.com', 'user4', '2026-01-04T00:00:00Z'),
('valid5@example.com', 'user5', '2026-01-05T00:00:00Z'),
('valid6@example.com', 'user6', '2026-01-06T00:00:00Z'),
('valid7@example.com', 'user7', '2026-01-07T00:00:00Z'),
('valid8@example.com', 'user8', '2026-01-08T00:00:00Z'),
('valid9@example.com', 'user9', '2026-01-09T00:00:00Z'),
('valid10@example.com', 'user10', '2026-01-10T00:00:00Z');

-- Insert 5 NULL login records (should be skipped)
INSERT INTO author_login_cache VALUES
('null1@example.com', NULL, '2026-02-01T00:00:00Z'),
('null2@example.com', NULL, '2026-02-02T00:00:00Z'),
('null3@example.com', NULL, '2026-02-03T00:00:00Z'),
('null4@example.com', NULL, '2026-02-04T00:00:00Z'),
('null5@example.com', NULL, '2026-02-05T00:00:00Z');

-- Insert 5 empty string login records (should also be skipped)
INSERT INTO author_login_cache VALUES
('empty1@example.com', '', '2026-03-01T00:00:00Z'),
('empty2@example.com', '', '2026-03-02T00:00:00Z'),
('empty3@example.com', '', '2026-03-03T00:00:00Z'),
('empty4@example.com', '', '2026-03-04T00:00:00Z'),
('empty5@example.com', '', '2026-03-05T00:00:00Z');

-- Expected results:
-- Total pairs read: 20
-- Valid positive resolutions: 10 (NULL/empty logins skipped)
-- Negative-cache (skipped): 10
