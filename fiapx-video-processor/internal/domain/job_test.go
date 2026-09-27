package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNewVideoJobRejectsTxt(t *testing.T) {
	_, err := NewVideoJob(uuid.New(), "notes.txt", "x", "c1")
	if err != ErrUnsupportedMedia {
		t.Fatalf("expected unsupported media, got %v", err)
	}
}

func TestJobLifecycle(t *testing.T) {
	job, err := NewVideoJob(uuid.New(), "race.mp4", "in/race.mp4", "corr-1")
	if err != nil {
		t.Fatal(err)
	}
	if job.Processor != ProcessorFFmpeg {
		t.Fatalf("default processor: %s", job.Processor)
	}
	if err := job.Queue(); err != nil {
		t.Fatal(err)
	}
	if err := job.StartProcessing(); err != nil {
		t.Fatal(err)
	}
	if job.StartedAt.IsZero() {
		t.Fatal("StartProcessing must set StartedAt")
	}
	if err := job.Complete("out/"+job.ID.String()+".zip", "thumbs/"+job.ID.String()+".png", 12, 4096, 1500*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if job.Status != StatusReady || job.FrameCount != 12 {
		t.Fatalf("complete: %+v", job)
	}
	if job.ThumbPath == "" {
		t.Fatal("Complete must set ThumbPath")
	}
	if job.ZipBytes != 4096 || job.ProcessDuration != 1500*time.Millisecond {
		t.Fatalf("perf fields: %+v", job)
	}
	if job.FinishedAt.IsZero() {
		t.Fatal("Complete must set FinishedAt")
	}
}

func TestFailSetsFinishedAt(t *testing.T) {
	job, _ := NewVideoJob(uuid.New(), "a.mp4", "a.mp4", "c")
	_ = job.Queue()
	_ = job.StartProcessing()
	if err := job.Fail("ffmpeg boom"); err != nil {
		t.Fatal(err)
	}
	if job.FinishedAt.IsZero() || job.ErrorMessage != "ffmpeg boom" {
		t.Fatalf("fail: %+v", job)
	}
	if err := job.EnsureDeletable(); err != nil {
		t.Fatal(err)
	}
}

func TestCannotCompleteFromQueued(t *testing.T) {
	job, _ := NewVideoJob(uuid.New(), "a.mp4", "a.mp4", "c")
	_ = job.Queue()
	if err := job.Complete("z.zip", "", 1, 0, 0); err != ErrInvalidTransition {
		t.Fatalf("expected invalid transition, got %v", err)
	}
}

func TestEnsureDeletableAfterProcessing(t *testing.T) {
	job, _ := NewVideoJob(uuid.New(), "a.mp4", "a.mp4", "c")
	_ = job.Queue()
	if err := job.EnsureDeletable(); err != ErrJobNotDeletable {
		t.Fatalf("queued: %v", err)
	}
	_ = job.StartProcessing()
	if err := job.EnsureDeletable(); err != ErrJobNotDeletable {
		t.Fatalf("processing: %v", err)
	}
	_ = job.Complete("z.zip", "t.png", 1, 0, 0)
	if err := job.EnsureDeletable(); err != nil {
		t.Fatal(err)
	}
}

func TestOwnedBy(t *testing.T) {
	owner := uuid.New()
	job, _ := NewVideoJob(owner, "a.mp4", "a.mp4", "c")
	if err := job.OwnedBy(owner); err != nil {
		t.Fatal(err)
	}
	if err := job.OwnedBy(uuid.New()); err != ErrForbidden {
		t.Fatalf("expected forbidden, got %v", err)
	}
}

func TestJobIDsAreUnique(t *testing.T) {
	u := uuid.New()
	a, _ := NewVideoJob(u, "a.mp4", "a.mp4", "c")
	b, _ := NewVideoJob(u, "a.mp4", "a.mp4", "c")
	if a.ID == b.ID {
		t.Fatal("jobs must not share identity")
	}
}

func TestParseProcessor(t *testing.T) {
	got, err := ParseProcessor("")
	if err != nil || got != ProcessorFFmpeg {
		t.Fatalf("empty: %s %v", got, err)
	}
	got, err = ParseProcessor("  FFMPEG ")
	if err != nil || got != ProcessorFFmpeg {
		t.Fatalf("ffmpeg: %s %v", got, err)
	}
	got, err = ParseProcessor("GST")
	if err != nil || got != ProcessorGStreamer {
		t.Fatalf("gst: %s %v", got, err)
	}
	got, err = ParseProcessor(" gstreamer ")
	if err != nil || got != ProcessorGStreamer {
		t.Fatalf("gstreamer: %s %v", got, err)
	}
	if _, err := ParseProcessor("handbrake"); err != ErrUnknownProcessor {
		t.Fatalf("got %v", err)
	}
}

func TestNewVideoJobNormalizesNameAndExtensions(t *testing.T) {
	for _, name := range []string{"dir/clip.MP4", "a.avi", "b.mov", "c.mkv", "d.wmv", "e.flv", "f.webm"} {
		job, err := NewVideoJob(uuid.New(), name, "in", "c")
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if name == "dir/clip.MP4" && job.OriginalFilename != "clip.MP4" {
			t.Fatalf("base name: %s", job.OriginalFilename)
		}
	}
	if _, err := NewVideoJob(uuid.New(), "noext", "in", "c"); err != ErrUnsupportedMedia {
		t.Fatalf("got %v", err)
	}
}

func TestInvalidTransitionsAndReadyGate(t *testing.T) {
	job, err := NewVideoJob(uuid.New(), "a.mp4", "a.mp4", "c")
	if err != nil {
		t.Fatal(err)
	}
	if err := job.StartProcessing(); err != ErrInvalidTransition {
		t.Fatalf("start from uploaded: %v", err)
	}
	if err := job.Fail("x"); err != ErrInvalidTransition {
		t.Fatalf("fail from uploaded: %v", err)
	}
	if err := job.EnsureReady(); err != ErrJobNotReady {
		t.Fatalf("ready gate: %v", err)
	}
	if err := job.Queue(); err != nil {
		t.Fatal(err)
	}
	if err := job.Queue(); err != ErrInvalidTransition {
		t.Fatalf("double queue: %v", err)
	}
	if err := job.StartProcessing(); err != nil {
		t.Fatal(err)
	}
	if err := job.Complete("z.zip", "t.png", 1, 1, time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if err := job.EnsureReady(); err != nil {
		t.Fatal(err)
	}
	if err := job.Fail("late"); err != ErrInvalidTransition {
		t.Fatalf("fail from ready: %v", err)
	}
}

func TestStatusTransitions(t *testing.T) {
	cases := []struct {
		from, to JobStatus
		ok       bool
	}{
		{StatusUploaded, StatusQueued, true},
		{StatusUploaded, StatusProcessing, false},
		{StatusQueued, StatusProcessing, true},
		{StatusQueued, StatusReady, false},
		{StatusProcessing, StatusReady, true},
		{StatusProcessing, StatusFailed, true},
		{StatusProcessing, StatusQueued, false},
		{StatusReady, StatusFailed, false},
		{StatusFailed, StatusReady, false},
	}
	for _, tc := range cases {
		if tc.from.CanTransitionTo(tc.to) != tc.ok {
			t.Fatalf("%s -> %s want %v", tc.from, tc.to, tc.ok)
		}
	}
}
