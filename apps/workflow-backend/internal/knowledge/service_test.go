package knowledge

import (
	"context"
	"testing"
)

func TestServiceCreateSearchUpdateDeleteWithProjectIsolation(t *testing.T) {
	service := NewService(NewMemoryRepository(), DeterministicEmbedder{})
	ctx := context.Background()

	doc, err := service.Create(ctx, Document{
		ProjectID:  "project_a",
		Type:       "site_guide",
		Title:      "Service Search",
		Source:     "manual",
		URL:        "https://console.example.test/services",
		Tags:       []string{"service"},
		Content:    "Use the service name search box to find services.",
		Confidence: 0.9,
		Status:     StatusActive,
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	hits, err := service.Search(ctx, SearchRequest{
		ProjectID: "project_a",
		Task:      "find service by name",
		URL:       "https://console.example.test/services",
		Limit:     5,
	})
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(hits) != 1 || hits[0].ID != doc.ID {
		t.Fatalf("expected created doc in search results, got %#v", hits)
	}

	otherProject, err := service.Search(ctx, SearchRequest{
		ProjectID: "project_b",
		Task:      "find service by name",
		Limit:     5,
	})
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(otherProject) != 0 {
		t.Fatalf("expected project isolation, got %#v", otherProject)
	}

	doc.Title = "Updated title"
	if _, err := service.Update(ctx, doc); err != nil {
		t.Fatalf("Update returned error: %v", err)
	}
	got, err := service.Get(ctx, "project_a", doc.ID)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got.Title != "Updated title" {
		t.Fatalf("expected updated title, got %q", got.Title)
	}

	if err := service.Delete(ctx, "project_a", doc.ID); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	hits, err = service.Search(ctx, SearchRequest{ProjectID: "project_a", Task: "service", Limit: 5})
	if err != nil {
		t.Fatalf("Search after delete returned error: %v", err)
	}
	if len(hits) != 0 {
		t.Fatalf("expected deleted doc omitted from search, got %#v", hits)
	}
}

func TestServiceBulkIngestCountsAcceptedAndRejected(t *testing.T) {
	service := NewService(NewMemoryRepository(), DeterministicEmbedder{})
	result, err := service.Ingest(context.Background(), []Document{
		{ProjectID: "project_a", Type: "guide", Title: "Accepted", Source: "manual", Content: "content"},
		{ProjectID: "", Type: "guide", Title: "Rejected", Source: "manual", Content: "content"},
	})
	if err != nil {
		t.Fatalf("Ingest returned error: %v", err)
	}
	if result.Accepted != 1 || result.Rejected != 1 {
		t.Fatalf("unexpected ingest result: %#v", result)
	}
}
