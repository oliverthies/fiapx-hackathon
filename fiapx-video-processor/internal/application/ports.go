package application

import (
	"context"
	"io"

	"github.com/google/uuid"
	"github.com/oliverthies/fiapx-video-processor/internal/domain"
)

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
}

type PasswordHasher interface {
	Hash(password string) (string, error)
	Match(hash, password string) bool
}

type TokenIssuer interface {
	Issue(userID uuid.UUID, email string) (string, error)
}

type Storage interface {
	SaveOriginal(ctx context.Context, jobID uuid.UUID, filename string, r io.Reader) (path string, err error)
	Open(ctx context.Context, path string) (io.ReadCloser, error)
	Absolute(path string) string
}

type Queue interface {
	PublishProcess(ctx context.Context, jobID uuid.UUID, correlationID string) error
}

type VideoProcessor interface {
	ExtractFrames(ctx context.Context, job *domain.VideoJob) (zipRelPath string, frames int, err error)
}

type Notifier interface {
	JobFailed(ctx context.Context, user *domain.User, job *domain.VideoJob) error
}

type Clock interface {
	NowUnix() int64
}
