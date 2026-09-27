package wiring

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/oliverthies/fiapx-video-processor/internal/adapter/ffmpeg"
	"github.com/oliverthies/fiapx-video-processor/internal/adapter/fs"
	"github.com/oliverthies/fiapx-video-processor/internal/adapter/gstreamer"
	"github.com/oliverthies/fiapx-video-processor/internal/adapter/processor"
	"github.com/oliverthies/fiapx-video-processor/internal/adapter/s3store"
	"github.com/oliverthies/fiapx-video-processor/internal/application"
	"github.com/oliverthies/fiapx-video-processor/internal/domain"
	"github.com/oliverthies/fiapx-video-processor/internal/platform"
)

func NewStorage(ctx context.Context) (application.Storage, string, error) {
	driver := strings.ToLower(platform.Env("STORAGE_DRIVER", "fs"))
	switch driver {
	case "fs", "filesystem", "local":
		root := platform.Env("STORAGE_ROOT", "/data")
		store, err := fs.New(root)
		return store, "fs", err
	case "s3", "minio":
		store, err := s3store.New(ctx, s3store.Config{
			Endpoint:  platform.Env("S3_ENDPOINT", ""),
			Bucket:    platform.Env("S3_BUCKET", "fiapx"),
			AccessKey: platform.Env("S3_ACCESS_KEY", platform.Env("MINIO_ROOT_USER", "")),
			SecretKey: platform.Env("S3_SECRET_KEY", platform.Env("MINIO_ROOT_PASSWORD", "")),
			Region:    platform.Env("S3_REGION", "us-east-1"),
			UseSSL:    strings.EqualFold(platform.Env("S3_USE_SSL", "false"), "true"),
		})
		return store, "s3", err
	default:
		return nil, driver, fmt.Errorf("unknown STORAGE_DRIVER %q (fs|s3)", driver)
	}
}

func NewProcessor(store application.Storage, storageDriver string) (application.VideoProcessor, string, error) {
	fallback, err := domain.ParseProcessor(platform.Env("PROCESSOR_DRIVER", domain.ProcessorFFmpeg))
	if err != nil {
		return nil, "", err
	}
	work := platform.Env("PROCESSOR_WORKDIR", "")
	if work == "" {
		if storageDriver == "fs" {
			work = filepath.Join(platform.Env("STORAGE_ROOT", "/data"), "temp")
		} else {
			work = filepath.Join(os.TempDir(), "fiapx-work")
		}
	}
	timeout := 15 * time.Minute
	if v := os.Getenv("PROCESSOR_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			timeout = d
		}
	}
	router := processor.NewRouter(fallback)
	router.Register(domain.ProcessorFFmpeg, ffmpeg.New(store, ffmpeg.Options{
		Bin:     platform.Env("PROCESSOR_BIN", "ffmpeg"),
		WorkDir: work,
		Timeout: timeout,
	}))
	router.Register(domain.ProcessorGStreamer, gstreamer.New(store, gstreamer.Options{
		Bin:     platform.Env("GSTREAMER_BIN", "gst-launch-1.0"),
		WorkDir: work,
		Timeout: timeout,
	}))
	return router, fallback, nil
}
