-- +goose Up

-- Conversations now carry a work type (design, discovery, ...) defined in
-- yaml instead of a DB-backed role.
ALTER TABLE conversations ADD COLUMN work_type TEXT NOT NULL DEFAULT '';
ALTER TABLE conversations DROP COLUMN role_uuid;
DROP TABLE IF EXISTS roles;

-- +goose Down

CREATE TABLE roles (
    uuid TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    system_prompt TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL
);
CREATE INDEX idx_roles_created_at ON roles(created_at);
ALTER TABLE conversations ADD COLUMN role_uuid TEXT NOT NULL DEFAULT '';
ALTER TABLE conversations DROP COLUMN work_type;
