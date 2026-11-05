-- We store only the SHA-256 of each token, so a leaked dump can't be replayed.
CREATE TABLE sessions (
    id          BIGSERIAL   PRIMARY KEY,
    user_id     BIGINT      NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash  CHAR(64)    NOT NULL UNIQUE,
    ip_address  VARCHAR(64) NOT NULL DEFAULT '',
    user_agent  TEXT        NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at  TIMESTAMPTZ NOT NULL
);

CREATE INDEX sessions_user_id_idx    ON sessions (user_id);
CREATE INDEX sessions_expires_at_idx ON sessions (expires_at);
