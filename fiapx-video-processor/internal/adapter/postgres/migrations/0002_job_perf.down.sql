ALTER TABLE video_jobs
    DROP COLUMN IF EXISTS started_at,
    DROP COLUMN IF EXISTS finished_at,
    DROP COLUMN IF EXISTS ffmpeg_ms,
    DROP COLUMN IF EXISTS zip_bytes;
