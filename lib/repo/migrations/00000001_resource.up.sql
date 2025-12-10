-- +goose Up

CREATE EXTENSION IF NOT EXISTS vector;

-- Define the Source enum type
CREATE TYPE source_enum AS ENUM (
    'SOURCE_UNSPECIFIED',
    'SOURCE_WEBSITE',
    'SOURCE_PDF'
);


-- Resource table
CREATE TABLE resources (
    uuid TEXT PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL,
    service TEXT NOT NULL,
    entity TEXT NOT NULL,
    source source_enum NOT NULL,
    path TEXT
);

create TABLE resource_embeddings (
    resource_uuid TEXT REFERENCES resources (uuid) ON DELETE CASCADE,
    chunk_index INT NOT NULL,
    content TEXT NOT NULL,
    embedding VECTOR(384) NOT NULL
);

-- +goose Down

DROP TABLE IF EXISTS resources;
DROP TABLE IF EXISTS resource_embeddings;


DROP TYPE IF EXISTS source_enum;

