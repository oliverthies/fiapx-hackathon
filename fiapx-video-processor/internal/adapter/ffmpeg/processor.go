package ffmpeg

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/oliverthies/fiapx-video-processor/internal/adapter/metrics"
	"github.com/oliverthies/fiapx-video-processor/internal/domain"
)

type Processor struct {
	root    string
	timeout time.Duration
}

func New(root string, timeout time.Duration) *Processor {
	if timeout == 0 {
		timeout = 15 * time.Minute
	}
	return &Processor{root: root, timeout: timeout}
}

func (p *Processor) ExtractFrames(ctx context.Context, job *domain.VideoJob) (string, int, error) {
	ctx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	tempDir := filepath.Join(p.root, "temp", job.ID.String())
	if err := os.MkdirAll(tempDir, 0o755); err != nil {
		return "", 0, err
	}
	defer os.RemoveAll(tempDir)

	input := job.OriginalPath
	if !filepath.IsAbs(input) {
		input = filepath.Join(p.root, input)
	}
	pattern := filepath.Join(tempDir, "frame_%04d.png")
	start := time.Now()
	cmd := exec.CommandContext(ctx, "ffmpeg", "-i", input, "-vf", "fps=1", "-y", pattern)
	out, err := cmd.CombinedOutput()
	metrics.FFmpegDuration.Observe(time.Since(start).Seconds())
	if err != nil {
		return "", 0, fmt.Errorf("ffmpeg: %w: %s", err, truncate(string(out), 400))
	}

	frames, err := filepath.Glob(filepath.Join(tempDir, "*.png"))
	if err != nil || len(frames) == 0 {
		return "", 0, fmt.Errorf("no frames extracted")
	}

	rel := filepath.Join("outputs", job.ID.String()+".zip")
	abs := filepath.Join(p.root, rel)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return "", 0, err
	}
	if err := zipFiles(abs, frames); err != nil {
		return "", 0, err
	}
	return rel, len(frames), nil
}

func zipFiles(dest string, files []string) error {
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()
	w := zip.NewWriter(f)
	defer w.Close()
	for _, path := range files {
		if err := addFile(w, path); err != nil {
			return err
		}
	}
	return nil
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

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
