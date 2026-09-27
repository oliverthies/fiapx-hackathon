package application

import (
	"context"
	"io"

	"github.com/google/uuid"
	"github.com/oliverthies/fiapx-video-processor/internal/domain"
)

type VideoService struct {
	jobs    JobRepository
	users   UserRepository
	store   Storage
	queue   Queue
	process VideoProcessor
	notify  Notifier
}

func NewVideoService(jobs JobRepository, users UserRepository, store Storage, queue Queue, process VideoProcessor, notify Notifier) *VideoService {
	return &VideoService{jobs: jobs, users: users, store: store, queue: queue, process: process, notify: notify}
}

func (s *VideoService) Upload(ctx context.Context, userID uuid.UUID, filename string, r io.Reader, correlationID, processor string) (*domain.VideoJob, error) {
	job, err := domain.NewVideoJob(userID, filename, "", correlationID)
	if err != nil {
		return nil, err
	}
	engine, err := domain.ParseProcessor(processor)
	if err != nil {
		return nil, err
	}
	job.Processor = engine
	path, err := s.store.SaveOriginal(ctx, job.ID, filename, r)
	if err != nil {
		return nil, err
	}
	job.OriginalPath = path
	if err := job.Queue(); err != nil {
		return nil, err
	}
	if err := s.jobs.Create(ctx, job); err != nil {
		return nil, err
	}
	if err := s.queue.PublishProcess(ctx, job.ID, job.CorrelationID); err != nil {
		return nil, err
	}
	return job, nil
}

func (s *VideoService) List(ctx context.Context, userID uuid.UUID) ([]*domain.VideoJob, error) {
	return s.jobs.ListByUser(ctx, userID)
}

func (s *VideoService) Get(ctx context.Context, userID, jobID uuid.UUID) (*domain.VideoJob, error) {
	job, err := s.jobs.FindByID(ctx, jobID)
	if err != nil {
		return nil, err
	}
	if err := job.OwnedBy(userID); err != nil {
		return nil, err
	}
	return job, nil
}

func (s *VideoService) Download(ctx context.Context, userID, jobID uuid.UUID) (*domain.VideoJob, error) {
	job, err := s.Get(ctx, userID, jobID)
	if err != nil {
		return nil, err
	}
	if err := job.EnsureReady(); err != nil {
		return nil, err
	}
	return job, nil
}

// Process is called by the worker. Isolated from HTTP.
func (s *VideoService) Process(ctx context.Context, jobID uuid.UUID) error {
	job, err := s.jobs.FindByID(ctx, jobID)
	if err != nil {
		return err
	}
	if err := job.StartProcessing(); err != nil {
		return err
	}
	if err := s.jobs.Update(ctx, job); err != nil {
		return err
	}

	result, procErr := s.process.ExtractFrames(ctx, job)
	if procErr != nil {
		_ = job.Fail(procErr.Error())
		_ = s.jobs.Update(ctx, job)
		if user, uerr := s.users.FindByID(ctx, job.UserID); uerr == nil && user != nil {
			_ = s.notify.JobFailed(ctx, user, job)
		}
		return procErr
	}
	if err := job.Complete(result.ZipRelPath, result.ThumbRelPath, result.Frames, result.ZipBytes, result.ProcessDuration); err != nil {
		return err
	}
	return s.jobs.Update(ctx, job)
}

func (s *VideoService) Delete(ctx context.Context, userID, jobID uuid.UUID) error {
	job, err := s.Get(ctx, userID, jobID)
	if err != nil {
		return err
	}
	if err := job.EnsureDeletable(); err != nil {
		return err
	}
	if err := s.jobs.Delete(ctx, job.ID); err != nil {
		return err
	}
	if s.store != nil {
		_ = s.store.Remove(ctx, job.OriginalPath)
		_ = s.store.Remove(ctx, job.ZipPath)
		_ = s.store.Remove(ctx, job.ThumbPath)
	}
	return nil
}
