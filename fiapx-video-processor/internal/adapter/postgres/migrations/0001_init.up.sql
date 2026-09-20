CREATE TABLE users (
    id UUID PRIMARY KEY,
    email TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE video_jobs (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users (id),
    status TEXT NOT NULL,
    original_filename TEXT NOT NULL,
    original_path TEXT NOT NULL,
    zip_path TEXT NOT NULL DEFAULT '',
    frame_count INTEGER NOT NULL DEFAULT 0,
    error_message TEXT NOT NULL DEFAULT '',
    correlation_id TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX video_jobs_user_id_idx ON video_jobs (user_id);
CREATE INDEX video_jobs_status_idx ON video_jobs (status);
