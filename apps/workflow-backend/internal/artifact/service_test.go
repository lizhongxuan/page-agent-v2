package artifact

import (
	"bytes"
	"context"
	"testing"
)

func TestServiceStoresArtifactMetadata(t *testing.T) {
	repo := NewMemoryRepository()
	service := NewService(NewLocalStore(t.TempDir()), repo)

	artifact, err := service.Store(context.Background(), PutRequest{
		ProjectID:   "project_a",
		OwnerType:   OwnerWorkflowRun,
		OwnerID:     "run_123",
		Kind:        KindSession,
		ContentType: "application/json",
		Reader:      bytes.NewBufferString(`{"steps":[]}`),
	})
	if err != nil {
		t.Fatalf("Store returned error: %v", err)
	}

	list, err := service.List(context.Background(), ListQuery{
		ProjectID: "project_a",
		OwnerType: OwnerWorkflowRun,
		OwnerID:   "run_123",
	})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(list) != 1 || list[0].ID != artifact.ID {
		t.Fatalf("unexpected list result: %#v", list)
	}

	otherProject, err := service.List(context.Background(), ListQuery{
		ProjectID: "project_b",
		OwnerType: OwnerWorkflowRun,
		OwnerID:   "run_123",
	})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(otherProject) != 0 {
		t.Fatalf("expected project isolation, got %#v", otherProject)
	}
}
