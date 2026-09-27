package fs

import (
	"bytes"
	"context"
	"io"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
)

func TestPutOpenRemove(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	payload := []byte("frame-bytes")
	n, err := s.Put(ctx, "thumbs/a.png", bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	if n != int64(len(payload)) {
		t.Fatalf("wrote %d", n)
	}
	f, err := s.Open(ctx, "thumbs/a.png")
	if err != nil {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(f)
	_ = f.Close()
	if !bytes.Equal(got, payload) {
		t.Fatalf("got %q", got)
	}
	if err := s.Remove(ctx, "thumbs/a.png"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Open(ctx, "thumbs/a.png"); err == nil {
		t.Fatal("expected missing after remove")
	}
}

func TestSaveOriginalUsesObjectKey(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	key, err := s.SaveOriginal(context.Background(), uuid.MustParse("11111111-1111-1111-1111-111111111111"), "clip.MP4", bytes.NewReader([]byte("mp4")))
	if err != nil {
		t.Fatal(err)
	}
	if filepath.ToSlash(key) != key {
		t.Fatalf("storage key must use slashes: %q", key)
	}
	if key == "" || filepath.Ext(key) != ".MP4" {
		t.Fatalf("key %q", key)
	}
}
