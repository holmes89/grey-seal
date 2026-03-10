-- +goose Up


-- Role table
CREATE TABLE roles (
    uuid TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    system_prompt TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL
);

-- Indexes for common query patterns
CREATE INDEX idx_roles_created_at ON roles(created_at);


-- +goose Down

DROP TABLE IF EXISTS roles;


