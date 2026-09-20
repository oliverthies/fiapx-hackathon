package domain

import (
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

type VideoJob struct {
	ID               uuid.UUID
	UserID           uuid.UUID
	Status           JobStatus
	OriginalFilename string
	OriginalPath     string
	ZipPath          string
	FrameCount       int
	ErrorMessage     string
	CorrelationID    string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func NewVideoJob(userID uuid.UUID, originalFilename, originalPath, correlationID string) (*VideoJob, error) {
	if !isSupportedVideo(originalFilename) {
		return nil, ErrUnsupportedMedia
	}
	now := time.Now().UTC()
	return &VideoJob{
		ID:               uuid.New(),
		UserID:           userID,
		Status:           StatusUploaded,
		OriginalFilename: filepath.Base(originalFilename),
		OriginalPath:     originalPath,
		CorrelationID:    correlationID,
		CreatedAt:        now,
		UpdatedAt:        now,
	}, nil
}

func (j *VideoJob) Queue() error { return j.transition(StatusQueued) }

func (j *VideoJob) StartProcessing() error { return j.transition(StatusProcessing) }

func (j *VideoJob) Complete(zipPath string, frames int) error {
	if err := j.transition(StatusReady); err != nil {
		return err
	}
	j.ZipPath = zipPath
	j.FrameCount = frames
	j.ErrorMessage = ""
	return nil
}

func (j *VideoJob) Fail(message string) error {
	if err := j.transition(StatusFailed); err != nil {
		return err
	}
	j.ErrorMessage = message
	return nil
}

func (j *VideoJob) OwnedBy(userID uuid.UUID) error {
	if j.UserID != userID {
		return ErrForbidden
	}
	return nil
}

func (j *VideoJob) EnsureReady() error {
	if j.Status != StatusReady {
		return ErrJobNotReady
	}
	return nil
}

func (j *VideoJob) transition(next JobStatus) error {
	if !j.Status.CanTransitionTo(next) {
		return ErrInvalidTransition
	}
	j.Status = next
	j.UpdatedAt = time.Now().UTC()
	return nil
}

func isSupportedVideo(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".mp4", ".avi", ".mov", ".mkv", ".wmv", ".flv", ".webm":
		return true
	default:
		return false
	}
}
