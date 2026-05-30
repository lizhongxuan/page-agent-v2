package httpapi

import (
	"net/http"

	"github.com/page-agent/workflow-backend/internal/knowledge"
	"github.com/page-agent/workflow-backend/internal/registry"
)

type knowledgeDocumentRequest struct {
	ID         string   `json:"id"`
	ProjectID  string   `json:"projectId"`
	ProjectKey string   `json:"projectKey"`
	Title      string   `json:"title"`
	Source     string   `json:"source"`
	URL        string   `json:"url"`
	Content    string   `json:"content"`
	Tags       []string `json:"tags"`
	SourceType string   `json:"sourceType"`
}

type knowledgeIngestRequest struct {
	Documents []knowledgeDocumentRequest `json:"documents"`
}

type knowledgeSearchRequest struct {
	Task        string   `json:"task"`
	URL         string   `json:"url"`
	Title       string   `json:"title"`
	ProjectKey  string   `json:"projectKey"`
	ProjectID   string   `json:"projectId"`
	VisibleText string   `json:"visibleText"`
	Hints       []string `json:"hints"`
	Limit       int      `json:"limit"`
}

type knowledgeHitResponse struct {
	ID      string   `json:"id"`
	Title   string   `json:"title"`
	Source  string   `json:"source"`
	Snippet string   `json:"snippet"`
	URL     string   `json:"url,omitempty"`
	Score   float64  `json:"score"`
	Tags    []string `json:"tags,omitempty"`
}

func registerKnowledgeRoutes(mux *http.ServeMux, repo registry.Repository, vectorizer KnowledgeVectorizer) {
	handleDocument := func(w http.ResponseWriter, r *http.Request) {
		writeDeprecatedEndpoint(w, "/api/memory/documents")
	}
	handleIngest := func(w http.ResponseWriter, r *http.Request) {
		writeDeprecatedEndpoint(w, "/api/memory/documents")
	}
	handleSearch := func(w http.ResponseWriter, r *http.Request) {
		writeDeprecatedEndpoint(w, "/api/memory/context")
	}

	mux.HandleFunc("POST /documents", handleDocument)
	mux.HandleFunc("POST /api/knowledge/documents", handleDocument)
	mux.HandleFunc("POST /ingest", handleIngest)
	mux.HandleFunc("POST /api/knowledge/ingest", handleIngest)
	mux.HandleFunc("POST /search", handleSearch)
	mux.HandleFunc("POST /api/knowledge/search", handleSearch)
}

func documentInput(request knowledgeDocumentRequest) knowledge.DocumentInput {
	projectID := request.ProjectID
	if projectID == "" {
		projectID = request.ProjectKey
	}
	return knowledge.DocumentInput{
		ID:         request.ID,
		ProjectID:  projectID,
		Title:      request.Title,
		Source:     request.Source,
		URL:        request.URL,
		Content:    request.Content,
		Tags:       request.Tags,
		SourceType: request.SourceType,
	}
}
