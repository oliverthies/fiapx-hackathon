package processor

import (
	"context"
	"fmt"

	"github.com/oliverthies/fiapx-video-processor/internal/application"
	"github.com/oliverthies/fiapx-video-processor/internal/domain"
)

var _ application.VideoProcessor = (*Router)(nil)

type Router struct {
	fallback string
	tools    map[string]application.VideoProcessor
}

func NewRouter(fallback string) *Router {
	if fallback == "" {
		fallback = domain.ProcessorFFmpeg
	}
	return &Router{fallback: fallback, tools: map[string]application.VideoProcessor{}}
}

func (r *Router) Register(name string, p application.VideoProcessor) {
	r.tools[name] = p
}

func (r *Router) ExtractFrames(ctx context.Context, job *domain.VideoJob) (application.ProcessResult, error) {
	name := job.Processor
	if name == "" {
		name = r.fallback
	}
	p, ok := r.tools[name]
	if !ok {
		return application.ProcessResult{}, fmt.Errorf("processor %q is not installed", name)
	}
	return p.ExtractFrames(ctx, job)
}
