-- +goose Up

-- Conversations now carry a work type (design, discovery, ...) defined in
-- yaml instead of a DB-backed role.
ALTER TABLE conversations ADD COLUMN work_type TEXT NOT NULL DEFAULT '';
ALTER TABLE conversations DROP COLUMN role_uuid;
DROP TABLE IF EXISTS roles;

-- Assistant messages can be drafts (discovery, design, tickets) that the user
-- keeps by writing them to Remora; kept_ref records where they went.
ALTER TABLE messages ADD COLUMN draft_kind TEXT NOT NULL DEFAULT '';
ALTER TABLE messages ADD COLUMN kept_ref TEXT NOT NULL DEFAULT '';

-- +goose Down

ALTER TABLE messages DROP COLUMN kept_ref;
ALTER TABLE messages DROP COLUMN draft_kind;

CREATE TABLE roles (
    uuid TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    system_prompt TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL
);
CREATE INDEX idx_roles_created_at ON roles(created_at);
ALTER TABLE conversations ADD COLUMN role_uuid TEXT NOT NULL DEFAULT '';
ALTER TABLE conversations DROP COLUMN work_type;
