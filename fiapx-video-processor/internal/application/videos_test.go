package application

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/oliverthies/fiapx-video-processor/internal/domain"
)

type memJobs struct {
	byID      map[uuid.UUID]*domain.VideoJob
	createErr error
	updateErr error
	deleteErr error
}

func (m *memJobs) Create(_ context.Context, job *domain.VideoJob) error {
	if m.createErr != nil {
		return m.createErr
	}
	m.byID[job.ID] = job
	return nil
}
func (m *memJobs) Update(_ context.Context, job *domain.VideoJob) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	m.byID[job.ID] = job
	return nil
}
func (m *memJobs) FindByID(_ context.Context, id uuid.UUID) (*domain.VideoJob, error) {
	j := m.byID[id]
	if j == nil {
		return nil, domain.ErrJobNotFound
	}
	return j, nil
}
func (m *memJobs) ListByUser(_ context.Context, userID uuid.UUID) ([]*domain.VideoJob, error) {
	var out []*domain.VideoJob
	for _, j := range m.byID {
		if j.UserID == userID {
			out = append(out, j)
		}
	}
	return out, nil
}
func (m *memJobs) Delete(_ context.Context, id uuid.UUID) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	if m.byID[id] == nil {
		return domain.ErrJobNotFound
	}
	delete(m.byID, id)
	return nil
}

type memStore struct {
	removed []string
	saveErr error
}

func (m *memStore) SaveOriginal(_ context.Context, jobID uuid.UUID, filename string, r io.Reader) (string, error) {
	if m.saveErr != nil {
		return "", m.saveErr
	}
	return "uploads/" + jobID.String() + ".mp4", nil
}
func (m *memStore) Put(_ context.Context, key string, r io.Reader) (int64, error) {
	n, _ := io.Copy(io.Discard, r)
	return n, nil
}
func (m *memStore) Open(_ context.Context, path string) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(nil)), nil
}
func (m *memStore) Remove(_ context.Context, path string) error {
	if path != "" {
		m.removed = append(m.removed, path)
	}
	return nil
}

type noopQueue struct{}

func (noopQueue) PublishProcess(context.Context, uuid.UUID, string) error { return nil }

type failQueue struct{ err error }

func (f failQueue) PublishProcess(context.Context, uuid.UUID, string) error { return f.err }

type stubProc struct {
	res    ProcessResult
	err    error
	mutate func(*domain.VideoJob)
}

func (s stubProc) ExtractFrames(_ context.Context, job *domain.VideoJob) (ProcessResult, error) {
	if s.mutate != nil {
		s.mutate(job)
	}
	return s.res, s.err
}

type recNotify struct{ n int }

func (r *recNotify) JobFailed(context.Context, *domain.User, *domain.VideoJob) error {
	r.n++
	return nil
}

func readyJob(owner uuid.UUID) *domain.VideoJob {
	job, _ := domain.NewVideoJob(owner, "clip.mp4", "uploads/a.mp4", "c")
	_ = job.Queue()
	_ = job.StartProcessing()
	_ = job.Complete("outputs/a.zip", "thumbs/a.png", 3, 100, 10)
	return job
}

func TestDeleteRemovesReadyJobAndFiles(t *testing.T) {
	owner := uuid.New()
	job := readyJob(owner)
	jobs := &memJobs{byID: map[uuid.UUID]*domain.VideoJob{job.ID: job}}
	store := &memStore{}
	svc := NewVideoService(jobs, nil, store, noopQueue{}, nil, nil)
	if err := svc.Delete(context.Background(), owner, job.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := jobs.FindByID(context.Background(), job.ID); err != domain.ErrJobNotFound {
		t.Fatalf("job still in repo: %v", err)
	}
	if len(store.removed) != 3 {
		t.Fatalf("removed files: %v", store.removed)
	}
}

func TestDeleteRejectsQueuedJob(t *testing.T) {
	owner := uuid.New()
	job, _ := domain.NewVideoJob(owner, "clip.mp4", "uploads/a.mp4", "c")
	_ = job.Queue()
	jobs := &memJobs{byID: map[uuid.UUID]*domain.VideoJob{job.ID: job}}
	svc := NewVideoService(jobs, nil, &memStore{}, noopQueue{}, nil, nil)
	if err := svc.Delete(context.Background(), owner, job.ID); err != domain.ErrJobNotDeletable {
		t.Fatalf("got %v", err)
	}
}

func TestUploadStoresSelectedProcessor(t *testing.T) {
	owner := uuid.New()
	jobs := &memJobs{byID: map[uuid.UUID]*domain.VideoJob{}}
	svc := NewVideoService(jobs, nil, &memStore{}, noopQueue{}, nil, nil)
	job, err := svc.Upload(context.Background(), owner, "clip.mp4", bytes.NewReader([]byte("x")), "c", "gstreamer")
	if err != nil {
		t.Fatal(err)
	}
	if job.Processor != domain.ProcessorGStreamer {
		t.Fatalf("processor %q", job.Processor)
	}
}

func TestUploadRejectsUnknownProcessor(t *testing.T) {
	owner := uuid.New()
	svc := NewVideoService(&memJobs{byID: map[uuid.UUID]*domain.VideoJob{}}, nil, &memStore{}, noopQueue{}, nil, nil)
	if _, err := svc.Upload(context.Background(), owner, "clip.mp4", bytes.NewReader([]byte("x")), "c", "handbrake"); err != domain.ErrUnknownProcessor {
		t.Fatalf("got %v", err)
	}
}

func TestUploadRejectsBadMediaAndStoreOrQueueErrors(t *testing.T) {
	owner := uuid.New()
	jobs := &memJobs{byID: map[uuid.UUID]*domain.VideoJob{}}
	svc := NewVideoService(jobs, nil, &memStore{saveErr: errors.New("disk")}, noopQueue{}, nil, nil)
	if _, err := svc.Upload(context.Background(), owner, "notes.txt", bytes.NewReader([]byte("x")), "c", ""); err != domain.ErrUnsupportedMedia {
		t.Fatalf("media: %v", err)
	}
	if _, err := svc.Upload(context.Background(), owner, "clip.mp4", bytes.NewReader([]byte("x")), "c", ""); err == nil {
		t.Fatal("expected store error")
	}
	jobs.createErr = errors.New("db")
	svc = NewVideoService(jobs, nil, &memStore{}, noopQueue{}, nil, nil)
	if _, err := svc.Upload(context.Background(), owner, "clip.mp4", bytes.NewReader([]byte("x")), "c", "ffmpeg"); err == nil {
		t.Fatal("expected create error")
	}
	jobs.createErr = nil
	svc = NewVideoService(jobs, nil, &memStore{}, failQueue{err: errors.New("amqp")}, nil, nil)
	if _, err := svc.Upload(context.Background(), owner, "clip.mp4", bytes.NewReader([]byte("x")), "c", ""); err == nil {
		t.Fatal("expected publish error")
	}
}

func TestListGetAndDownload(t *testing.T) {
	owner := uuid.New()
	job := readyJob(owner)
	other, _ := domain.NewVideoJob(uuid.New(), "b.mp4", "b.mp4", "c")
	jobs := &memJobs{byID: map[uuid.UUID]*domain.VideoJob{job.ID: job, other.ID: other}}
	svc := NewVideoService(jobs, nil, &memStore{}, noopQueue{}, nil, nil)

	listed, err := svc.List(context.Background(), owner)
	if err != nil || len(listed) != 1 || listed[0].ID != job.ID {
		t.Fatalf("list: %+v %v", listed, err)
	}
	got, err := svc.Get(context.Background(), owner, job.ID)
	if err != nil || got.ID != job.ID {
		t.Fatalf("get: %v", err)
	}
	if _, err := svc.Get(context.Background(), owner, uuid.New()); err != domain.ErrJobNotFound {
		t.Fatalf("missing: %v", err)
	}
	if _, err := svc.Get(context.Background(), uuid.New(), job.ID); err != domain.ErrForbidden {
		t.Fatalf("forbidden: %v", err)
	}
	if _, err := svc.Download(context.Background(), owner, job.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Download(context.Background(), owner, uuid.New()); err != domain.ErrJobNotFound {
		t.Fatalf("download missing: %v", err)
	}
	queued, _ := domain.NewVideoJob(owner, "c.mp4", "c.mp4", "c")
	_ = queued.Queue()
	jobs.byID[queued.ID] = queued
	if _, err := svc.Download(context.Background(), owner, queued.ID); err != domain.ErrJobNotReady {
		t.Fatalf("not ready: %v", err)
	}
}

func TestProcessSuccessAndFailure(t *testing.T) {
	owner := uuid.New()
	user := &domain.User{ID: owner, Email: "ana@fiapx.local"}
	users := &memUsers{byEmail: map[string]*domain.User{}, byID: map[uuid.UUID]*domain.User{owner: user}}
	job, _ := domain.NewVideoJob(owner, "clip.mp4", "uploads/a.mp4", "corr")
	_ = job.Queue()
	jobs := &memJobs{byID: map[uuid.UUID]*domain.VideoJob{job.ID: job}}
	notify := &recNotify{}
	svc := NewVideoService(jobs, users, &memStore{}, noopQueue{}, stubProc{
		res: ProcessResult{ZipRelPath: "outputs/a.zip", ThumbRelPath: "thumbs/a.png", Frames: 4, ZipBytes: 20, ProcessDuration: time.Second},
	}, notify)
	if err := svc.Process(context.Background(), job.ID); err != nil {
		t.Fatal(err)
	}
	if jobs.byID[job.ID].Status != domain.StatusReady || jobs.byID[job.ID].FrameCount != 4 {
		t.Fatalf("processed: %+v", jobs.byID[job.ID])
	}

	failed, _ := domain.NewVideoJob(owner, "clip.mp4", "uploads/b.mp4", "corr")
	_ = failed.Queue()
	jobs.byID[failed.ID] = failed
	svc = NewVideoService(jobs, users, &memStore{}, noopQueue{}, stubProc{err: errors.New("ffmpeg boom")}, notify)
	if err := svc.Process(context.Background(), failed.ID); err == nil {
		t.Fatal("expected process error")
	}
	if jobs.byID[failed.ID].Status != domain.StatusFailed || notify.n != 1 {
		t.Fatalf("failed job: %+v notify=%d", jobs.byID[failed.ID], notify.n)
	}

	silent, _ := domain.NewVideoJob(owner, "clip.mp4", "uploads/c.mp4", "corr")
	_ = silent.Queue()
	jobs.byID[silent.ID] = silent
	users.byID = map[uuid.UUID]*domain.User{}
	svc = NewVideoService(jobs, users, &memStore{}, noopQueue{}, stubProc{err: errors.New("boom")}, notify)
	if err := svc.Process(context.Background(), silent.ID); err == nil {
		t.Fatal("expected process error")
	}
	if notify.n != 1 {
		t.Fatalf("notify without user: %d", notify.n)
	}

	lookup, _ := domain.NewVideoJob(owner, "clip.mp4", "uploads/d.mp4", "corr")
	_ = lookup.Queue()
	jobs.byID[lookup.ID] = lookup
	users.findErr = errors.New("db")
	svc = NewVideoService(jobs, users, &memStore{}, noopQueue{}, stubProc{err: errors.New("boom")}, notify)
	if err := svc.Process(context.Background(), lookup.ID); err == nil {
		t.Fatal("expected process error")
	}
	if notify.n != 1 {
		t.Fatalf("notify on lookup error: %d", notify.n)
	}
}

func TestProcessRejectsBadStateAndUpdateErrors(t *testing.T) {
	owner := uuid.New()
	missing := NewVideoService(&memJobs{byID: map[uuid.UUID]*domain.VideoJob{}}, nil, nil, noopQueue{}, stubProc{}, nil)
	if err := missing.Process(context.Background(), uuid.New()); err != domain.ErrJobNotFound {
		t.Fatalf("missing: %v", err)
	}

	ready := readyJob(owner)
	jobs := &memJobs{byID: map[uuid.UUID]*domain.VideoJob{ready.ID: ready}}
	svc := NewVideoService(jobs, nil, nil, noopQueue{}, stubProc{}, nil)
	if err := svc.Process(context.Background(), ready.ID); err != domain.ErrInvalidTransition {
		t.Fatalf("start: %v", err)
	}

	job, _ := domain.NewVideoJob(owner, "clip.mp4", "a.mp4", "c")
	_ = job.Queue()
	jobs = &memJobs{byID: map[uuid.UUID]*domain.VideoJob{job.ID: job}, updateErr: errors.New("db")}
	svc = NewVideoService(jobs, nil, nil, noopQueue{}, stubProc{}, nil)
	if err := svc.Process(context.Background(), job.ID); err == nil {
		t.Fatal("expected update error")
	}

	job, _ = domain.NewVideoJob(owner, "clip.mp4", "a.mp4", "c")
	_ = job.Queue()
	jobs = &memJobs{byID: map[uuid.UUID]*domain.VideoJob{job.ID: job}}
	svc = NewVideoService(jobs, nil, nil, noopQueue{}, stubProc{mutate: func(j *domain.VideoJob) { j.Status = domain.StatusUploaded }}, nil)
	if err := svc.Process(context.Background(), job.ID); err != domain.ErrInvalidTransition {
		t.Fatalf("complete: %v", err)
	}
}

func TestDeleteGuards(t *testing.T) {
	owner := uuid.New()
	job := readyJob(owner)
	jobs := &memJobs{byID: map[uuid.UUID]*domain.VideoJob{job.ID: job}}
	svc := NewVideoService(jobs, nil, nil, noopQueue{}, nil, nil)
	if err := svc.Delete(context.Background(), uuid.New(), job.ID); err != domain.ErrForbidden {
		t.Fatalf("forbidden: %v", err)
	}
	if err := svc.Delete(context.Background(), owner, uuid.New()); err != domain.ErrJobNotFound {
		t.Fatalf("missing: %v", err)
	}
	jobs.deleteErr = errors.New("db")
	svc = NewVideoService(jobs, nil, &memStore{}, noopQueue{}, nil, nil)
	if err := svc.Delete(context.Background(), owner, job.ID); err == nil {
		t.Fatal("expected delete error")
	}
	jobs.deleteErr = nil
	if err := svc.Delete(context.Background(), owner, job.ID); err != nil {
		t.Fatal(err)
	}
}
