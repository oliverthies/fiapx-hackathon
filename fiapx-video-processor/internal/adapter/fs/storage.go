package fs

import (
	"context"
	"io"
	"os"
	"path"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/oliverthies/fiapx-video-processor/internal/application"
)

var _ application.Storage = (*Storage)(nil)


type Storage struct {
	root string
}

func New(root string) (*Storage, error) {
	for _, d := range []string{"uploads", "outputs", "temp", "thumbs"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			return nil, err
		}
	}
	return &Storage{root: root}, nil
}

func (s *Storage) SaveOriginal(ctx context.Context, jobID uuid.UUID, filename string, r io.Reader) (string, error) {
	key := path.Join("uploads", jobID.String()+filepath.Ext(filename))
	if _, err := s.Put(ctx, key, r); err != nil {
		return "", err
	}
	return key, nil
}

func (s *Storage) Put(_ context.Context, key string, r io.Reader) (int64, error) {
	abs := s.abs(key)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return 0, err
	}
	f, err := os.Create(abs)
	if err != nil {
		return 0, err
	}
	n, err := io.Copy(f, r)
	if err != nil {
		_ = f.Close()
		return n, err
	}
	return n, f.Close()
}

func (s *Storage) Open(_ context.Context, key string) (io.ReadCloser, error) {
	return os.Open(s.abs(key))
}

func (s *Storage) Remove(_ context.Context, key string) error {
	if key == "" {
		return nil
	}
	err := os.Remove(s.abs(key))
	if err != nil && os.IsNotExist(err) {
		return nil
	}
	return err
}

func (s *Storage) abs(key string) string {
	p := filepath.FromSlash(key)
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(s.root, p)
}
