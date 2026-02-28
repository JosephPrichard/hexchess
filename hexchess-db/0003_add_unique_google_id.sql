-- +goose up
CREATE UNIQUE INDEX idx_google_account_id
    ON users (google_account_id);

-- +goose Down
DROP INDEX IF EXISTS idx_google_account_id;