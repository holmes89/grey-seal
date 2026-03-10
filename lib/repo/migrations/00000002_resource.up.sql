-- +goose Up

-- source stored as integer (proto enum value: 0=unspecified, 1=website, 2=pdf, 3=text)
CREATE TABLE resources (
    uuid TEXT PRIMARY KEY,
    name TEXT NOT NULL DEFAULT '',
    service TEXT NOT NULL DEFAULT '',
    entity TEXT NOT NULL DEFAULT '',
    source INTEGER NOT NULL DEFAULT 0,
    path TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL,
    indexed_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX idx_resources_created_at ON resources(created_at);


-- +goose Down

DROP TABLE IF EXISTS resources;
