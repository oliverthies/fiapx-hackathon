package framezip

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/oliverthies/fiapx-video-processor/internal/domain"
)

type memStore struct {
	files     map[string][]byte
	openErr   error
	failThumb bool
	failZip   bool
}

func (m *memStore) SaveOriginal(context.Context, uuid.UUID, string, io.Reader) (string, error) {
	return "", nil
}
func (m *memStore) Put(_ context.Context, key string, r io.Reader) (int64, error) {
	if m.failThumb && strings.HasPrefix(key, "thumbs/") {
		return 0, errors.New("thumb")
	}
	if m.failZip && strings.HasPrefix(key, "outputs/") {
		return 0, errors.New("zip")
	}
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
func (m *memStore) Open(_ context.Context, key string) (io.ReadCloser, error) {
	if m.openErr != nil {
		return nil, m.openErr
	}
	b, ok := m.files[key]
	if !ok {
		return nil, errors.New("missing")
	}
	return io.NopCloser(bytes.NewReader(b)), nil
}
func (m *memStore) Remove(context.Context, string) error { return nil }

func writePNG(t *testing.T, dir, name string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte("png-"+name), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestExtractPackagesFrames(t *testing.T) {
	jobID := uuid.New()
	store := &memStore{files: map[string][]byte{"orig.mp4": []byte("video-bytes")}}
	var observed bool
	res, err := Extract(context.Background(), &domain.VideoJob{
		ID:               jobID,
		OriginalFilename: "clip",
		OriginalPath:     "orig.mp4",
	}, Options{
		Store:   store,
		WorkDir: t.TempDir(),
		Run: func(_ context.Context, inputFile, frameDir string) error {
			b, err := os.ReadFile(inputFile)
			if err != nil {
				return err
			}
			if string(b) != "video-bytes" {
				return errors.New("staged input mismatch")
			}
			if filepath.Ext(inputFile) != ".mp4" {
				return errors.New("expected extension from original path")
			}
			writePNG(t, frameDir, "frame_0001.png")
			writePNG(t, frameDir, "frame_0002.png")
			return nil
		},
		Observe: func(float64) { observed = true },
	})
	if err != nil {
		t.Fatal(err)
	}
	if !observed || res.Frames != 2 || res.ThumbRelPath == "" || res.ZipBytes == 0 || res.ProcessDuration < 0 {
		t.Fatalf("result: %+v observed=%v", res, observed)
	}
	zr, err := zip.NewReader(bytes.NewReader(store.files[res.ZipRelPath]), int64(len(store.files[res.ZipRelPath])))
	if err != nil {
		t.Fatal(err)
	}
	if len(zr.File) != 2 {
		t.Fatalf("zip entries: %d", len(zr.File))
	}
}

func TestExtractFailureModes(t *testing.T) {
	job := &domain.VideoJob{ID: uuid.New(), OriginalFilename: "clip.mp4", OriginalPath: "missing.mp4"}
	store := &memStore{openErr: errors.New("gone")}
	if _, err := Extract(context.Background(), job, Options{
		Store: store, WorkDir: t.TempDir(), Timeout: time.Second,
		Run: func(context.Context, string, string) error { return nil },
	}); err == nil {
		t.Fatal("expected open error")
	}

	store = &memStore{files: map[string][]byte{"clip.mp4": []byte("v")}}
	job.OriginalPath = "clip.mp4"
	if _, err := Extract(context.Background(), job, Options{
		Store: store, WorkDir: t.TempDir(), Timeout: time.Second,
		Run: func(context.Context, string, string) error { return errors.New("decoder") },
	}); err == nil || !strings.Contains(err.Error(), "decoder") {
		t.Fatalf("run: %v", err)
	}

	if _, err := Extract(context.Background(), job, Options{
		Store: store, WorkDir: t.TempDir(), Timeout: time.Second,
		Run: func(context.Context, string, string) error { return nil },
	}); err == nil || !strings.Contains(err.Error(), "no frames") {
		t.Fatalf("empty: %v", err)
	}

	store.failZip = true
	if _, err := Extract(context.Background(), job, Options{
		Store: store, WorkDir: t.TempDir(), Timeout: time.Second,
		Run: func(_ context.Context, _, frameDir string) error {
			writePNG(t, frameDir, "frame_0000.png")
			return nil
		},
	}); err == nil {
		t.Fatal("expected zip put error")
	}

	store.failZip = false
	store.failThumb = true
	res, err := Extract(context.Background(), job, Options{
		Store: store, WorkDir: t.TempDir(), Timeout: time.Second,
		Run: func(_ context.Context, _, frameDir string) error {
			writePNG(t, frameDir, "other.png")
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.ThumbRelPath != "" || res.Frames != 1 {
		t.Fatalf("thumb fallback: %+v", res)
	}
}
