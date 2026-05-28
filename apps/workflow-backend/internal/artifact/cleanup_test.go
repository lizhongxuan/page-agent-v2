package artifact

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCleanupExpiredArtifactsDeletesLocalFilesAndMetadata(t *testing.T) {
	store := NewLocalStore(t.TempDir())
	repo := NewMemoryRepository()
	service := NewService(store, repo)

	expired, err := service.Store(context.Background(), PutRequest{
		ProjectID:   "project_a",
		OwnerType:   OwnerWorkflowRun,
		OwnerID:     "run_1",
		Kind:        KindDebugBundle,
		ContentType: "application/json",
		Reader:      bytes.NewBufferString(`{"debug":true}`),
	})
	if err != nil {
		t.Fatalf("Store expired artifact: %v", err)
	}
	past := time.Now().Add(-time.Hour)
	expired.ExpiresAt = &past
	if err := repo.Save(context.Background(), expired); err != nil {
		t.Fatalf("Save expired metadata: %v", err)
	}

	active, err := service.Store(context.Background(), PutRequest{
		ProjectID:   "project_a",
		OwnerType:   OwnerWorkflowRun,
		OwnerID:     "run_1",
		Kind:        KindSession,
		ContentType: "application/json",
		Reader:      bytes.NewBufferString(`{"active":true}`),
	})
	if err != nil {
		t.Fatalf("Store active artifact: %v", err)
	}

	result, err := CleanupExpired(context.Background(), service, time.Now())
	if err != nil {
		t.Fatalf("CleanupExpired returned error: %v", err)
	}
	if result.Deleted != 1 {
		t.Fatalf("expected one artifact deleted, got %#v", result)
	}
	if _, err := repo.Get(context.Background(), expired.ID); err == nil {
		t.Fatal("expected expired metadata to be deleted")
	}
	if _, err := os.Stat(filepath.Join(store.RootDir(), expired.RelativePath)); !os.IsNotExist(err) {
		t.Fatalf("expected expired file deleted, err=%v", err)
	}
	if _, err := repo.Get(context.Background(), active.ID); err != nil {
		t.Fatalf("expected active metadata preserved: %v", err)
	}
}

func TestVerifyArtifactsReportsMissingFile(t *testing.T) {
	store := NewLocalStore(t.TempDir())
	repo := NewMemoryRepository()
	service := NewService(store, repo)

	artifact, err := service.Store(context.Background(), PutRequest{
		ProjectID:   "project_a",
		OwnerType:   OwnerWorkflowRun,
		OwnerID:     "run_1",
		Kind:        KindSession,
		ContentType: "application/json",
		Reader:      bytes.NewBufferString(`{"ok":true}`),
	})
	if err != nil {
		t.Fatalf("Store returned error: %v", err)
	}
	if err := os.Remove(filepath.Join(store.RootDir(), artifact.RelativePath)); err != nil {
		t.Fatalf("remove artifact file: %v", err)
	}

	report, err := VerifyArtifacts(context.Background(), service)
	if err != nil {
		t.Fatalf("VerifyArtifacts returned error: %v", err)
	}
	if len(report.MissingFiles) != 1 || report.MissingFiles[0] != artifact.ID {
		t.Fatalf("expected missing file report for %q, got %#v", artifact.ID, report)
	}
}
