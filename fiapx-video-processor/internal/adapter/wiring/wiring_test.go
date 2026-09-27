package wiring

import (
	"context"
	"os"
	"testing"
)

func TestUnknownStorageDriver(t *testing.T) {
	t.Setenv("STORAGE_DRIVER", "gcs")
	if _, _, err := NewStorage(context.Background()); err == nil {
		t.Fatal("expected error")
	}
}

func TestUnknownProcessorDriver(t *testing.T) {
	t.Setenv("PROCESSOR_DRIVER", "handbrake")
	if _, _, err := NewProcessor(nil, "fs"); err == nil {
		t.Fatal("expected error")
	}
}

func TestFilesystemDriver(t *testing.T) {
	root := t.TempDir()
	t.Setenv("STORAGE_DRIVER", "fs")
	t.Setenv("STORAGE_ROOT", root)
	store, name, err := NewStorage(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if name != "fs" || store == nil {
		t.Fatalf("%s %#v", name, store)
	}
	t.Setenv("PROCESSOR_DRIVER", "ffmpeg")
	t.Setenv("PROCESSOR_WORKDIR", t.TempDir())
	proc, pname, err := NewProcessor(store, name)
	if err != nil {
		t.Fatal(err)
	}
	if pname != "ffmpeg" || proc == nil {
		t.Fatalf("%s %#v", pname, proc)
	}
}

func TestS3RequiresEndpoint(t *testing.T) {
	t.Setenv("STORAGE_DRIVER", "s3")
	t.Setenv("S3_ENDPOINT", "")
	t.Setenv("S3_BUCKET", "")
	if _, _, err := NewStorage(context.Background()); err == nil {
		t.Fatal("expected missing endpoint")
	}
	_ = os.Unsetenv("STORAGE_DRIVER")
}
