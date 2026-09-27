package application

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/google/uuid"
	"github.com/oliverthies/fiapx-video-processor/internal/domain"
)

type memJobs struct{ byID map[uuid.UUID]*domain.VideoJob }

func (m *memJobs) Create(_ context.Context, job *domain.VideoJob) error {
	m.byID[job.ID] = job
	return nil
}
func (m *memJobs) Update(_ context.Context, job *domain.VideoJob) error {
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
	if m.byID[id] == nil {
		return domain.ErrJobNotFound
	}
	delete(m.byID, id)
	return nil
}

type memStore struct{ removed []string }

func (m *memStore) SaveOriginal(_ context.Context, jobID uuid.UUID, filename string, r io.Reader) (string, error) {
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
