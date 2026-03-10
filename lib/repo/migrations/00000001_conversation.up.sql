-- +goose Up

CREATE TABLE conversations (
    uuid TEXT PRIMARY KEY,
    title TEXT NOT NULL DEFAULT '',
    role_uuid TEXT NOT NULL DEFAULT '',
    resource_uuids TEXT[] NOT NULL DEFAULT '{}',
    summary TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL
);

CREATE INDEX idx_conversations_updated_at ON conversations(updated_at);
CREATE INDEX idx_conversations_created_at ON conversations(created_at);

-- role stored as integer (proto enum value: 0=unspecified, 1=user, 2=assistant)
CREATE TABLE messages (
    uuid TEXT PRIMARY KEY,
    conversation_uuid TEXT NOT NULL REFERENCES conversations(uuid) ON DELETE CASCADE,
    role INTEGER NOT NULL DEFAULT 0,
    content TEXT NOT NULL,
    resource_uuids TEXT[] NOT NULL DEFAULT '{}',
    feedback INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL
);

CREATE INDEX idx_messages_conversation_uuid ON messages(conversation_uuid);
CREATE INDEX idx_messages_created_at ON messages(created_at);


-- +goose Down

DROP TABLE IF EXISTS messages;
DROP TABLE IF EXISTS conversations;
