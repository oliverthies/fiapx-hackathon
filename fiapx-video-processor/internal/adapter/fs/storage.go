package fs

import (
	"context"
	"io"
	"os"
	"path/filepath"

	"github.com/google/uuid"
)

type Storage struct {
	root string
}

func New(root string) (*Storage, error) {
	for _, d := range []string{"uploads", "outputs", "temp"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			return nil, err
		}
	}
	return &Storage{root: root}, nil
}

func (s *Storage) SaveOriginal(_ context.Context, jobID uuid.UUID, filename string, r io.Reader) (string, error) {
	rel := filepath.Join("uploads", jobID.String()+filepath.Ext(filename))
	abs := filepath.Join(s.root, rel)
	f, err := os.Create(abs)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := io.Copy(f, r); err != nil {
		return "", err
	}
	return rel, nil
}

func (s *Storage) Open(_ context.Context, path string) (io.ReadCloser, error) {
	return os.Open(s.Absolute(path))
}

func (s *Storage) Absolute(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(s.root, path)
}
