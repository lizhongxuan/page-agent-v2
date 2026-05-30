package knowledge

import (
	"strings"
	"testing"
)

func TestChunkDocumentSplitsChineseTextAtFiveHundredCharacters(t *testing.T) {
	content := strings.Repeat("服务管理页可通过服务名称搜索框定位服务。", 80)

	chunks, err := ChunkDocument(DocumentInput{
		ID:        "doc_service",
		ProjectID: "default",
		Title:     "服务管理手册",
		Source:    "manual",
		Content:   content,
		Tags:      []string{"service", "status"},
	})

	if err != nil {
		t.Fatalf("ChunkDocument failed: %v", err)
	}
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks, got %#v", chunks)
	}
	for _, chunk := range chunks {
		if len([]rune(chunk.ChunkText)) > 500 {
			t.Fatalf("chunk exceeds 500 chars: %d", len([]rune(chunk.ChunkText)))
		}
	}
}

func TestChunkDocumentRejectsSensitiveMaterial(t *testing.T) {
	_, err := ChunkDocument(DocumentInput{
		ID:        "doc_secret",
		ProjectID: "default",
		Title:     "secret",
		Source:    "manual",
		Content:   "api_key=secret-value",
	})

	if err == nil {
		t.Fatal("expected sensitive document to be rejected")
	}
}

func TestChunkDocumentKeepsMarkdownHeadingsInMetadata(t *testing.T) {
	chunks, err := ChunkDocument(DocumentInput{
		ID:        "doc_service",
		ProjectID: "default",
		Title:     "服务管理手册",
		Source:    "manual",
		URL:       "https://docs.example.com/service",
		Content:   "# 服务管理\n服务管理页可通过服务名称搜索框定位服务。",
	})
	if err != nil {
		t.Fatalf("ChunkDocument failed: %v", err)
	}
	headings, ok := chunks[0].Metadata["headings"].([]string)
	if !ok || len(headings) != 1 || headings[0] != "服务管理" {
		t.Fatalf("expected markdown headings metadata, got %#v", chunks[0].Metadata)
	}
}

func TestChunkDocumentRejectsEmptyContent(t *testing.T) {
	_, err := ChunkDocument(DocumentInput{
		ID:        "doc_empty",
		ProjectID: "default",
		Title:     "Empty",
		Source:    "manual",
		Content:   "   ",
	})

	if err == nil {
		t.Fatal("expected empty content to be rejected")
	}
}
