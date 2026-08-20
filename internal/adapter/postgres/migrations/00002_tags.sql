-- +goose Up
CREATE TABLE tags
(
    id         SERIAL       PRIMARY KEY,
    name       VARCHAR(255) NOT NULL UNIQUE,
    color      VARCHAR(10)  NOT NULL,
    created_at TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE transaction_tags
(
    transaction_id BIGINT  NOT NULL REFERENCES transactions (id) ON DELETE CASCADE,
    tag_id         INTEGER NOT NULL REFERENCES tags (id) ON DELETE CASCADE,
    PRIMARY KEY (transaction_id, tag_id)
);
CREATE INDEX idx_transaction_tags_tag_id ON transaction_tags (tag_id);

-- +goose Down
DROP TABLE transaction_tags;
DROP TABLE tags;
