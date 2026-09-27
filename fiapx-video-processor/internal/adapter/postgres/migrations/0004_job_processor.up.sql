ALTER TABLE video_jobs
    ADD COLUMN processor TEXT NOT NULL DEFAULT 'ffmpeg';
