-- +goose Up


-- AgentRun table
CREATE TABLE agent_runs (
    uuid TEXT PRIMARY KEY,
    provider TEXT NOT NULL,
    repo_url TEXT NOT NULL,
    status TEXT NOT NULL,
    session_id TEXT NOT NULL,
    pr_url TEXT NOT NULL DEFAULT '',
    error TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL
);

-- Indexes for common query patterns
CREATE INDEX idx_agent_runs_created_at ON agent_runs(created_at);


-- +goose Down

DROP TABLE IF EXISTS agent_runs;

