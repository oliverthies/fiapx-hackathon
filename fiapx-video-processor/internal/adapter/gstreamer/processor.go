package gstreamer

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/oliverthies/fiapx-video-processor/internal/adapter/framezip"
	"github.com/oliverthies/fiapx-video-processor/internal/adapter/metrics"
	"github.com/oliverthies/fiapx-video-processor/internal/application"
	"github.com/oliverthies/fiapx-video-processor/internal/domain"
)

var _ application.VideoProcessor = (*Processor)(nil)

type Options struct {
	Bin     string
	WorkDir string
	Timeout time.Duration
}

type Processor struct {
	store   application.Storage
	bin     string
	workDir string
	timeout time.Duration
}

func New(store application.Storage, opt Options) *Processor {
	if opt.Bin == "" {
		opt.Bin = "gst-launch-1.0"
	}
	if opt.WorkDir == "" {
		opt.WorkDir = filepath.Join(os.TempDir(), "fiapx-work")
	}
	if opt.Timeout == 0 {
		opt.Timeout = 15 * time.Minute
	}
	return &Processor{store: store, bin: opt.Bin, workDir: opt.WorkDir, timeout: opt.Timeout}
}

func (p *Processor) ExtractFrames(ctx context.Context, job *domain.VideoJob) (application.ProcessResult, error) {
	return framezip.Extract(ctx, job, framezip.Options{
		Store:   p.store,
		WorkDir: p.workDir,
		Timeout: p.timeout,
		Observe: metrics.GStreamerDuration.Observe,
		Run: func(ctx context.Context, input, frameDir string) error {
			pattern := filepath.Join(frameDir, "frame_%04d.png")
			cmd := exec.CommandContext(ctx, p.bin,
				"-q", "-e",
				"filesrc", "location="+input,
				"!", "decodebin",
				"!", "videoconvert",
				"!", "videorate",
				"!", "video/x-raw,framerate=1/1",
				"!", "pngenc",
				"!", "multifilesink", "location="+pattern,
			)
			out, err := cmd.CombinedOutput()
			if err != nil {
				return fmt.Errorf("%s: %w: %s", p.bin, err, truncate(string(out), 400))
			}
			return nil
		},
	})
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
