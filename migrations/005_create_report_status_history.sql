CREATE TABLE report_status_history (
    id         BIGSERIAL   PRIMARY KEY,
    report_id  BIGINT      NOT NULL REFERENCES reports (id) ON DELETE CASCADE,
    status     VARCHAR(20) NOT NULL,
    note       TEXT        NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX report_status_history_report_id_idx ON report_status_history (report_id, created_at);

-- Reports created before this migration get their first timeline entry.
INSERT INTO report_status_history (report_id, status, created_at)
SELECT id, status, created_at FROM reports;

-- The "my reports" list filters by user and sorts by newest.
DROP INDEX reports_user_id_idx;
CREATE INDEX reports_user_id_created_at_idx ON reports (user_id, created_at DESC);
