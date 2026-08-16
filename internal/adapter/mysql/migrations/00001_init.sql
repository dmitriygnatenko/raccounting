-- +goose Up
CREATE TABLE users
(
    id            INTEGER UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    username      VARCHAR(255) NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    settings      JSON         NOT NULL DEFAULT '{}',
    created_at    TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY uq_users_username (username)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE sessions
(
    token      CHAR(64)  NOT NULL PRIMARY KEY,
    user_id    INTEGER UNSIGNED NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    KEY        idx_sessions_user_id (user_id),
    CONSTRAINT fk_sessions_user FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE currencies
(
    code       CHAR(3)        NOT NULL PRIMARY KEY,
    symbol     VARCHAR(10)    NOT NULL,
    name       VARCHAR(100)   NOT NULL,
    rate       DECIMAL(18, 6) NOT NULL DEFAULT 1,
    is_default TINYINT(1) NOT NULL DEFAULT 0,
    status     TINYINT UNSIGNED NOT NULL DEFAULT 1, -- 1 - active, 2 - archived
    created_at TIMESTAMP      NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP      NOT NULL DEFAULT CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE categories
(
    id         INTEGER UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    name       VARCHAR(255) NOT NULL,
    color      VARCHAR(10)  NOT NULL,
    type       TINYINT UNSIGNED NOT NULL,           -- 1 - income, 2 - expense
    status     TINYINT UNSIGNED NOT NULL DEFAULT 1, -- 1 - active, 2 - archived
    created_at TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE accounts
(
    id         INTEGER UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    name       VARCHAR(255) NOT NULL,
    type       TINYINT UNSIGNED NOT NULL,           -- 1 - cash, 2 - card, 3 - account, 4 - savings, 5 - credit card, 6 - debt, 7 virtual
    currency   CHAR(3)      NOT NULL,
    balance    INTEGER      NOT NULL DEFAULT 0,
    status     TINYINT UNSIGNED NOT NULL DEFAULT 1, -- 1 - active, 2 - archived
    created_at TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    KEY        idx_accounts_currency (currency),
    CONSTRAINT fk_accounts_currency FOREIGN KEY (currency) REFERENCES currencies (code) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE transactions
(
    id                      BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    category_id             INTEGER UNSIGNED NULL,
    type                    TINYINT UNSIGNED NOT NULL, -- 1 - income, 2 - expense, 3 - transfer
    account_id              INTEGER UNSIGNED NOT NULL,
    currency                CHAR(3)   NOT NULL,
    amount                  INTEGER   NOT NULL,
    transfer_transaction_id BIGINT UNSIGNED NULL,
    transfer_currency       CHAR(3) NULL,
    transfer_amount         INTEGER NULL,
    transfer_rate           DECIMAL(18, 6) NULL,
    transfer_account_id     INTEGER UNSIGNED NULL,
    memo                    TEXT      NOT NULL DEFAULT '',
    operation_at            TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_at              TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at              TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    KEY                     idx_transactions_category (category_id),
    KEY                     idx_transactions_account_id (account_id),
    KEY                     idx_transactions_currency (currency),
    KEY                     idx_transactions_transfer_transaction_id (transfer_transaction_id),
    KEY                     idx_transactions_transfer_account_id (transfer_account_id),
    KEY                     idx_transactions_transfer_currency (transfer_currency),
    CONSTRAINT fk_transactions_category FOREIGN KEY (category_id) REFERENCES categories (id) ON DELETE RESTRICT,
    CONSTRAINT fk_transactions_account FOREIGN KEY (account_id) REFERENCES accounts (id) ON DELETE RESTRICT,
    CONSTRAINT fk_transactions_transfer_account FOREIGN KEY (transfer_account_id) REFERENCES accounts (id) ON DELETE RESTRICT,
    CONSTRAINT fk_transactions_currency FOREIGN KEY (currency) REFERENCES currencies (code) ON DELETE RESTRICT,
    CONSTRAINT fk_transactions_transfer_currency FOREIGN KEY (transfer_currency) REFERENCES currencies (code) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE budgets
(
    id          INTEGER UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    month_key   CHAR(7)   NOT NULL,
    category_id INTEGER UNSIGNED NOT NULL,
    amount      INTEGER UNSIGNED NOT NULL,
    created_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    KEY         idx_budgets_category_id (category_id),
    UNIQUE KEY uq_budgets_month_category (month_key, category_id),
    CONSTRAINT fk_budgets_category FOREIGN KEY (category_id) REFERENCES categories (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- +goose Down
DROP TABLE budgets;
DROP TABLE transactions;
DROP TABLE categories;
DROP TABLE accounts;
DROP TABLE currencies;
DROP TABLE sessions;
DROP TABLE users;
