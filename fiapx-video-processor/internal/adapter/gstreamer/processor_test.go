package gstreamer

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/oliverthies/fiapx-video-processor/internal/domain"
)

type mapStore struct{ files map[string][]byte }

func (m *mapStore) SaveOriginal(_ context.Context, jobID uuid.UUID, filename string, r io.Reader) (string, error) {
	return "", nil
}
func (m *mapStore) Put(_ context.Context, key string, r io.Reader) (int64, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return 0, err
	}
	if m.files == nil {
		m.files = map[string][]byte{}
	}
	m.files[key] = b
	return int64(len(b)), nil
}
func (m *mapStore) Open(_ context.Context, key string) (io.ReadCloser, error) {
	b, ok := m.files[key]
	if !ok {
		return nil, osError("missing " + key)
	}
	return io.NopCloser(bytes.NewReader(b)), nil
}
func (m *mapStore) Remove(_ context.Context, key string) error {
	delete(m.files, key)
	return nil
}

type osError string

func (e osError) Error() string { return string(e) }

func TestExtractFramesRequiresOriginalInStorage(t *testing.T) {
	store := &mapStore{files: map[string][]byte{}}
	p := New(store, Options{WorkDir: t.TempDir(), Timeout: time.Second})
	job, err := domain.NewVideoJob(uuid.New(), "clip.mp4", "uploads/missing.mp4", "c")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.ExtractFrames(context.Background(), job); err == nil {
		t.Fatal("expected open original to fail")
	}
}
