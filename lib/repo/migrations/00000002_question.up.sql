-- +goose Up


-- Question table
CREATE TABLE questions (
    uuid TEXT PRIMARY KEY,
    role_description TEXT NOT NULL,
    content TEXT NOT NULL
);

CREATE TABLE question_responses (
    question_uuid TEXT NOT NULL,
    response TEXT NOT NULL,
    FOREIGN KEY (question_uuid) REFERENCES questions(uuid) ON DELETE CASCADE
);


CREATE TABLE question_references (
    question_uuid TEXT NOT NULL,
    resource_uuid TEXT NOT NULL,
    FOREIGN KEY (question_uuid) REFERENCES questions(uuid) ON DELETE CASCADE,
    FOREIGN KEY (resource_uuid) REFERENCES resources(uuid) ON DELETE CASCADE
);


-- +goose Down

DROP TABLE IF EXISTS questions;
DROP TABLE IF EXISTS question_responses;
DROP TABLE IF EXISTS question_references;

