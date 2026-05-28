package artifact

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestLocalStorePutGetDeleteJSONArtifact(t *testing.T) {
	store := NewLocalStore(t.TempDir())
	ctx := context.Background()

	stored, err := store.Put(ctx, PutRequest{
		ProjectID:   "project_a",
		OwnerType:   OwnerWorkflowRun,
		OwnerID:     "run_123",
		Kind:        KindBrowserState,
		ContentType: "application/json",
		Reader:      bytes.NewBufferString(`{"url":"https://example.test"}`),
	})
	if err != nil {
		t.Fatalf("Put returned error: %v", err)
	}

	if stored.StorageBackend != "local" {
		t.Fatalf("expected local backend, got %q", stored.StorageBackend)
	}
	if stored.SizeBytes == 0 {
		t.Fatal("expected non-zero size")
	}
	if stored.SHA256 == "" {
		t.Fatal("expected sha256")
	}
	if !filepath.IsLocal(stored.RelativePath) {
		t.Fatalf("expected local relative path, got %q", stored.RelativePath)
	}

	got, err := store.Get(ctx, stored)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	defer got.Close()
	data, err := io.ReadAll(got)
	if err != nil {
		t.Fatalf("ReadAll returned error: %v", err)
	}
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != stored.SHA256 {
		t.Fatalf("sha256 mismatch: got %s, want %s", hex.EncodeToString(sum[:]), stored.SHA256)
	}

	if err := store.Delete(ctx, stored); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(store.RootDir(), stored.RelativePath)); !os.IsNotExist(err) {
		t.Fatalf("expected local file deleted, stat err=%v", err)
	}
}

func TestLocalStoreRejectsUnsafeKindsAndContentTypes(t *testing.T) {
	store := NewLocalStore(t.TempDir())
	ctx := context.Background()

	for _, tc := range []PutRequest{
		{
			ProjectID:   "project_a",
			OwnerType:   OwnerWorkflowRun,
			OwnerID:     "run_123",
			Kind:        ArtifactKind("screenshot"),
			ContentType: "image/png",
			Reader:      bytes.NewBufferString("png"),
		},
		{
			ProjectID:   "project_a",
			OwnerType:   OwnerWorkflowRun,
			OwnerID:     "run_123",
			Kind:        KindSession,
			ContentType: "video/mp4",
			Reader:      bytes.NewBufferString("video"),
		},
	} {
		if _, err := store.Put(ctx, tc); err == nil {
			t.Fatalf("expected Put to reject kind=%q contentType=%q", tc.Kind, tc.ContentType)
		}
	}
}

func TestLocalStoreRejectsPathTraversalOnGet(t *testing.T) {
	store := NewLocalStore(t.TempDir())

	_, err := store.Get(context.Background(), Artifact{
		RelativePath: "../outside",
	})
	if err == nil {
		t.Fatal("expected path traversal to be rejected")
	}
}
