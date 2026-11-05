CREATE TABLE reports (
    id          BIGSERIAL    PRIMARY KEY,
    user_id     BIGINT       NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    title       VARCHAR(120) NOT NULL,
    description TEXT         NOT NULL,
    photo_key   TEXT         NOT NULL,
    status      VARCHAR(20)  NOT NULL DEFAULT 'admin_review'
                CHECK (status IN ('admin_review', 'approved', 'rejected')),
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX reports_user_id_idx ON reports (user_id);

CREATE TABLE hashtags (
    id   BIGSERIAL   PRIMARY KEY,
    name VARCHAR(30) NOT NULL UNIQUE
);

CREATE TABLE report_hashtags (
    report_id  BIGINT NOT NULL REFERENCES reports (id) ON DELETE CASCADE,
    hashtag_id BIGINT NOT NULL REFERENCES hashtags (id) ON DELETE CASCADE,
    PRIMARY KEY (report_id, hashtag_id)
);

CREATE INDEX report_hashtags_hashtag_id_idx ON report_hashtags (hashtag_id);
