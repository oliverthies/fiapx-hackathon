package framezip

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"time"

	"github.com/oliverthies/fiapx-video-processor/internal/application"
	"github.com/oliverthies/fiapx-video-processor/internal/domain"
)

type RunFunc func(ctx context.Context, inputFile, frameDir string) error

type Options struct {
	Store   application.Storage
	WorkDir string
	Timeout time.Duration
	Run     RunFunc
	Observe func(seconds float64)
}

func Extract(ctx context.Context, job *domain.VideoJob, opt Options) (application.ProcessResult, error) {
	empty := application.ProcessResult{}
	if opt.Timeout == 0 {
		opt.Timeout = 15 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, opt.Timeout)
	defer cancel()

	tempDir := filepath.Join(opt.WorkDir, job.ID.String())
	if err := os.MkdirAll(tempDir, 0o755); err != nil {
		return empty, err
	}
	defer os.RemoveAll(tempDir)

	input, err := stageOriginal(ctx, opt.Store, job, tempDir)
	if err != nil {
		return empty, err
	}

	frameDir := filepath.Join(tempDir, "frames")
	if err := os.MkdirAll(frameDir, 0o755); err != nil {
		return empty, err
	}

	start := time.Now()
	runErr := opt.Run(ctx, input, frameDir)
	dur := time.Since(start)
	if opt.Observe != nil {
		opt.Observe(dur.Seconds())
	}
	if runErr != nil {
		return empty, runErr
	}

	frames, err := filepath.Glob(filepath.Join(frameDir, "*.png"))
	if err != nil || len(frames) == 0 {
		return empty, fmt.Errorf("no frames extracted")
	}
	sort.Strings(frames)

	thumbKey := path.Join("thumbs", job.ID.String()+".png")
	first := firstFrame(frameDir, frames)
	if err := putFile(ctx, opt.Store, thumbKey, first); err != nil {
		thumbKey = ""
	}

	zipLocal := filepath.Join(tempDir, "frames.zip")
	if err := zipFiles(zipLocal, frames); err != nil {
		return empty, err
	}
	zipKey := path.Join("outputs", job.ID.String()+".zip")
	n, err := putFileN(ctx, opt.Store, zipKey, zipLocal)
	if err != nil {
		return empty, err
	}
	return application.ProcessResult{
		ZipRelPath:      zipKey,
		ThumbRelPath:    thumbKey,
		Frames:          len(frames),
		ProcessDuration: dur,
		ZipBytes:        n,
	}, nil
}

func firstFrame(dir string, frames []string) string {
	for _, name := range []string{"frame_0001.png", "frame_0000.png"} {
		p := filepath.Join(dir, name)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return frames[0]
}

func stageOriginal(ctx context.Context, store application.Storage, job *domain.VideoJob, dir string) (string, error) {
	src, err := store.Open(ctx, job.OriginalPath)
	if err != nil {
		return "", fmt.Errorf("open original: %w", err)
	}
	defer src.Close()
	ext := filepath.Ext(job.OriginalFilename)
	if ext == "" {
		ext = filepath.Ext(job.OriginalPath)
	}
	dest := filepath.Join(dir, "input"+ext)
	f, err := os.Create(dest)
	if err != nil {
		return "", err
	}
	_, copyErr := io.Copy(f, src)
	closeErr := f.Close()
	if copyErr != nil {
		return "", copyErr
	}
	return dest, closeErr
}

func putFile(ctx context.Context, store application.Storage, key, local string) error {
	_, err := putFileN(ctx, store, key, local)
	return err
}

func putFileN(ctx context.Context, store application.Storage, key, local string) (int64, error) {
	f, err := os.Open(local)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	return store.Put(ctx, key, f)
}

func zipFiles(dest string, files []string) error {
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()
	w := zip.NewWriter(f)
	for _, p := range files {
		if err := addFile(w, p); err != nil {
			_ = w.Close()
			return err
		}
	}
	return w.Close()
}

func addFile(w *zip.Writer, path string) error {
	in, err := os.Open(path)
	if err != nil {
		return err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return err
	}
	h, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	h.Name = filepath.Base(path)
	h.Method = zip.Deflate
	out, err := w.CreateHeader(h)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, in)
	return err
}
