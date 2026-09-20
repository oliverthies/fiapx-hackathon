package domain

import (
	"testing"

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
	if err := job.Queue(); err != nil {
		t.Fatal(err)
	}
	if err := job.StartProcessing(); err != nil {
		t.Fatal(err)
	}
	if err := job.Complete("out/"+job.ID.String()+".zip", 12); err != nil {
		t.Fatal(err)
	}
	if job.Status != StatusReady || job.FrameCount != 12 {
		t.Fatalf("complete: %+v", job)
	}
}

func TestCannotCompleteFromQueued(t *testing.T) {
	job, _ := NewVideoJob(uuid.New(), "a.mp4", "a.mp4", "c")
	_ = job.Queue()
	if err := job.Complete("z.zip", 1); err != ErrInvalidTransition {
		t.Fatalf("expected invalid transition, got %v", err)
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
