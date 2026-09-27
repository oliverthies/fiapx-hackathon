package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/oliverthies/fiapx-video-processor/internal/domain"
)

type Pool struct {
	pool *pgxpool.Pool
}

func New(ctx context.Context, url string) (*Pool, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return &Pool{pool: pool}, nil
}

func (p *Pool) Close() { p.pool.Close() }

func (p *Pool) Ping(ctx context.Context) error { return p.pool.Ping(ctx) }

func (p *Pool) Users() *UserRepo { return &UserRepo{p: p.pool} }
func (p *Pool) Jobs() *JobRepo   { return &JobRepo{p: p.pool} }

type UserRepo struct{ p *pgxpool.Pool }

func (r *UserRepo) Create(ctx context.Context, user *domain.User) error {
	_, err := r.p.Exec(ctx,
		`INSERT INTO users (id, email, password_hash, created_at) VALUES ($1,$2,$3,$4)`,
		user.ID, user.Email, user.PasswordHash, user.CreatedAt)
	return err
}

func (r *UserRepo) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	return scanUser(r.p.QueryRow(ctx,
		`SELECT id, email, password_hash, created_at FROM users WHERE email=$1`, email))
}

func (r *UserRepo) FindByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	return scanUser(r.p.QueryRow(ctx,
		`SELECT id, email, password_hash, created_at FROM users WHERE id=$1`, id))
}

func scanUser(row pgx.Row) (*domain.User, error) {
	var u domain.User
	if err := row.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrInvalidCredentials
		}
		return nil, err
	}
	return &u, nil
}

type JobRepo struct{ p *pgxpool.Pool }

func (r *JobRepo) Create(ctx context.Context, job *domain.VideoJob) error {
	_, err := r.p.Exec(ctx, `
		INSERT INTO video_jobs
		(id, user_id, status, original_filename, original_path, zip_path, frame_count, error_message, correlation_id,
		 created_at, updated_at, started_at, finished_at, ffmpeg_ms, zip_bytes, thumb_path, processor)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)`,
		job.ID, job.UserID, job.Status, job.OriginalFilename, job.OriginalPath,
		job.ZipPath, job.FrameCount, job.ErrorMessage, job.CorrelationID, job.CreatedAt, job.UpdatedAt,
		nullTime(job.StartedAt), nullTime(job.FinishedAt), job.ProcessDuration.Milliseconds(), job.ZipBytes, job.ThumbPath, job.Processor)
	return err
}

func (r *JobRepo) Update(ctx context.Context, job *domain.VideoJob) error {
	_, err := r.p.Exec(ctx, `
		UPDATE video_jobs SET status=$2, zip_path=$3, frame_count=$4, error_message=$5, updated_at=$6,
		started_at=$7, finished_at=$8, ffmpeg_ms=$9, zip_bytes=$10, thumb_path=$11
		WHERE id=$1`,
		job.ID, job.Status, job.ZipPath, job.FrameCount, job.ErrorMessage, job.UpdatedAt,
		nullTime(job.StartedAt), nullTime(job.FinishedAt), job.ProcessDuration.Milliseconds(), job.ZipBytes, job.ThumbPath)
	return err
}

func (r *JobRepo) FindByID(ctx context.Context, id uuid.UUID) (*domain.VideoJob, error) {
	return scanJob(r.p.QueryRow(ctx, jobSelect+` WHERE id=$1`, id))
}

func (r *JobRepo) ListByUser(ctx context.Context, userID uuid.UUID) ([]*domain.VideoJob, error) {
	rows, err := r.p.Query(ctx, jobSelect+` WHERE user_id=$1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*domain.VideoJob
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

func (r *JobRepo) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.p.Exec(ctx, `DELETE FROM video_jobs WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrJobNotFound
	}
	return nil
}

const jobSelect = `SELECT id, user_id, status, original_filename, original_path, zip_path, frame_count, error_message, correlation_id, created_at, updated_at, started_at, finished_at, ffmpeg_ms, zip_bytes, thumb_path, processor FROM video_jobs`

func scanJob(row pgx.Row) (*domain.VideoJob, error) {
	var j domain.VideoJob
	var startedAt, finishedAt *time.Time
	var ffmpegMs int64
	if err := row.Scan(&j.ID, &j.UserID, &j.Status, &j.OriginalFilename, &j.OriginalPath,
		&j.ZipPath, &j.FrameCount, &j.ErrorMessage, &j.CorrelationID, &j.CreatedAt, &j.UpdatedAt,
		&startedAt, &finishedAt, &ffmpegMs, &j.ZipBytes, &j.ThumbPath, &j.Processor); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrJobNotFound
		}
		return nil, err
	}
	if startedAt != nil {
		j.StartedAt = startedAt.UTC()
	}
	if finishedAt != nil {
		j.FinishedAt = finishedAt.UTC()
	}
	j.ProcessDuration = time.Duration(ffmpegMs) * time.Millisecond
	if j.Processor == "" {
		j.Processor = domain.ProcessorFFmpeg
	}
	return &j, nil
}

func nullTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t
}

// IsUndefinedColumn reports a query against a column the current schema does not have yet.
func IsUndefinedColumn(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "42703"
}
