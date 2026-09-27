package processor

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/oliverthies/fiapx-video-processor/internal/application"
	"github.com/oliverthies/fiapx-video-processor/internal/domain"
)

type stub struct{ name string }

func (s stub) ExtractFrames(_ context.Context, _ *domain.VideoJob) (application.ProcessResult, error) {
	return application.ProcessResult{Frames: 1, ZipRelPath: s.name}, nil
}

func TestRouterUsesJobProcessor(t *testing.T) {
	r := NewRouter(domain.ProcessorFFmpeg)
	r.Register(domain.ProcessorFFmpeg, stub{name: "ffmpeg"})
	r.Register(domain.ProcessorGStreamer, stub{name: "gstreamer"})
	job := &domain.VideoJob{ID: uuid.New(), Processor: domain.ProcessorGStreamer}
	res, err := r.ExtractFrames(context.Background(), job)
	if err != nil {
		t.Fatal(err)
	}
	if res.ZipRelPath != "gstreamer" {
		t.Fatalf("got %s", res.ZipRelPath)
	}
}

func TestRouterFallsBackWhenEmpty(t *testing.T) {
	r := NewRouter(domain.ProcessorFFmpeg)
	r.Register(domain.ProcessorFFmpeg, stub{name: "ffmpeg"})
	res, err := r.ExtractFrames(context.Background(), &domain.VideoJob{})
	if err != nil {
		t.Fatal(err)
	}
	if res.ZipRelPath != "ffmpeg" {
		t.Fatalf("got %s", res.ZipRelPath)
	}
}

func TestRouterUnknown(t *testing.T) {
	r := NewRouter(domain.ProcessorFFmpeg)
	r.Register(domain.ProcessorFFmpeg, stub{name: "ffmpeg"})
	_, err := r.ExtractFrames(context.Background(), &domain.VideoJob{Processor: "handbrake"})
	if err == nil {
		t.Fatal("expected error")
	}
}
