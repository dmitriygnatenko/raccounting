-- +goose Up
CREATE TABLE users
(
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    username      VARCHAR(255) NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    settings      TEXT         NOT NULL DEFAULT '{}',
    created_at    TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (username)
);

CREATE TABLE sessions
(
    token      CHAR(64)  NOT NULL PRIMARY KEY,
    user_id    INTEGER   NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_sessions_user_id ON sessions (user_id);

CREATE TABLE currencies
(
    code       CHAR(3)        NOT NULL PRIMARY KEY,
    symbol     VARCHAR(10)    NOT NULL,
    name       VARCHAR(255)   NOT NULL,
    rate       DECIMAL(18, 6) NOT NULL DEFAULT 1,
    is_default TINYINT(1) NOT NULL DEFAULT 0,
    status     TINYINT    NOT NULL DEFAULT 1, -- 1 - active, 2 - archived
    created_at TIMESTAMP      NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP      NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE categories
(
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    name       VARCHAR(255) NOT NULL,
    color      VARCHAR(10)  NOT NULL,
    type       TINYINT      NOT NULL, -- 1 - income, 2 - expense
    status     TINYINT      NOT NULL DEFAULT 1, -- 1 - active, 2 - archived
    created_at TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE accounts
(
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    name       VARCHAR(255) NOT NULL,
    type       TINYINT      NOT NULL, -- 1 - cash, 2 - card, 3 - account, 4 - savings, 5 - credit card, 6 - debt, 7 virtual
    currency   CHAR(3)      NOT NULL REFERENCES currencies (code) ON DELETE RESTRICT,
    balance    INTEGER      NOT NULL DEFAULT 0,
    status     TINYINT      NOT NULL DEFAULT 1, -- 1 - active, 2 - archived
    created_at TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_accounts_currency ON accounts (currency);

CREATE TABLE transactions
(
    id                      INTEGER PRIMARY KEY AUTOINCREMENT,
    category_id             INTEGER NULL REFERENCES categories (id) ON DELETE RESTRICT,
    type                    TINYINT        NOT NULL, -- 1 - income, 2 - expense, 3 - transfer
    account_id              INTEGER        NOT NULL REFERENCES accounts (id) ON DELETE RESTRICT,
    currency                CHAR(3)        NOT NULL REFERENCES currencies (code) ON DELETE RESTRICT,
    amount                  INTEGER        NOT NULL,
    transfer_transaction_id INTEGER NULL,
    transfer_currency       CHAR(3) NULL REFERENCES currencies (code) ON DELETE RESTRICT,
    transfer_amount         INTEGER NULL,
    transfer_rate           DECIMAL(18, 6) NULL,
    transfer_account_id     INTEGER NULL REFERENCES accounts (id) ON DELETE RESTRICT,
    memo                    TEXT           NOT NULL DEFAULT '',
    operation_at            TIMESTAMP      NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_at              TIMESTAMP      NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at              TIMESTAMP      NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_transactions_category ON transactions (category_id);
CREATE INDEX idx_transactions_account_id ON transactions (account_id);
CREATE INDEX idx_transactions_currency ON transactions (currency);
CREATE INDEX idx_transactions_transfer_transaction_id ON transactions (transfer_transaction_id);
CREATE INDEX idx_transactions_transfer_account_id ON transactions (transfer_account_id);
CREATE INDEX idx_transactions_transfer_currency ON transactions (transfer_currency);

CREATE TABLE budgets
(
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    month_key   CHAR(7)   NOT NULL,
    category_id INTEGER   NOT NULL REFERENCES categories (id) ON DELETE CASCADE,
    amount      INTEGER   NOT NULL,
    created_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (month_key, category_id)
);
CREATE INDEX idx_budgets_category_id ON budgets (category_id);

-- +goose Down
DROP TABLE budgets;
DROP TABLE transactions;
DROP TABLE categories;
DROP TABLE accounts;
DROP TABLE currencies;
DROP TABLE sessions;
DROP TABLE users;
