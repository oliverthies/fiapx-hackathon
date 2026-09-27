package application

import (
	"context"
	"io"
	"time"

	"github.com/google/uuid"
	"github.com/oliverthies/fiapx-video-processor/internal/domain"
)

type ProcessResult struct {
	ZipRelPath      string
	ThumbRelPath    string
	Frames          int
	ProcessDuration time.Duration
	ZipBytes        int64
}

type UserRepository interface {
	Create(ctx context.Context, user *domain.User) error
	FindByEmail(ctx context.Context, email string) (*domain.User, error)
	FindByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
}

type JobRepository interface {
	Create(ctx context.Context, job *domain.VideoJob) error
	Update(ctx context.Context, job *domain.VideoJob) error
	FindByID(ctx context.Context, id uuid.UUID) (*domain.VideoJob, error)
	ListByUser(ctx context.Context, userID uuid.UUID) ([]*domain.VideoJob, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type PasswordHasher interface {
	Hash(password string) (string, error)
	Match(hash, password string) bool
}

type TokenIssuer interface {
	Issue(userID uuid.UUID, email string) (string, error)
}

type Storage interface {
	SaveOriginal(ctx context.Context, jobID uuid.UUID, filename string, r io.Reader) (key string, err error)
	Put(ctx context.Context, key string, r io.Reader) (n int64, err error)
	Open(ctx context.Context, key string) (io.ReadCloser, error)
	Remove(ctx context.Context, key string) error
}

type Queue interface {
	PublishProcess(ctx context.Context, jobID uuid.UUID, correlationID string) error
}

type VideoProcessor interface {
	ExtractFrames(ctx context.Context, job *domain.VideoJob) (ProcessResult, error)
}

type Notifier interface {
	JobFailed(ctx context.Context, user *domain.User, job *domain.VideoJob) error
}

type Clock interface {
	NowUnix() int64
}
