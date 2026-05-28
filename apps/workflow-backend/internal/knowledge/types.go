package knowledge

import "time"

type Status string

const (
	StatusActive  Status = "active"
	StatusDeleted Status = "deleted"
)

type Document struct {
	ID         string         `json:"id"`
	ProjectID  string         `json:"projectId"`
	Type       string         `json:"type"`
	Title      string         `json:"title"`
	Source     string         `json:"source"`
	URL        string         `json:"url,omitempty"`
	Tags       []string       `json:"tags,omitempty"`
	Content    string         `json:"content"`
	Scope      map[string]any `json:"scope,omitempty"`
	Confidence float64        `json:"confidence"`
	Status     Status         `json:"status"`
	CreatedAt  time.Time      `json:"createdAt,omitempty"`
	UpdatedAt  time.Time      `json:"updatedAt,omitempty"`
}

type SearchRequest struct {
	ProjectID   string   `json:"projectId,omitempty"`
	ProjectKey  string   `json:"projectKey,omitempty"`
	Task        string   `json:"task"`
	URL         string   `json:"url"`
	Title       string   `json:"title"`
	VisibleText string   `json:"visibleText,omitempty"`
	Hints       []string `json:"hints,omitempty"`
	Limit       int      `json:"limit"`
}

type Hit struct {
	ID      string   `json:"id"`
	Title   string   `json:"title"`
	Source  string   `json:"source"`
	Snippet string   `json:"snippet"`
	URL     string   `json:"url,omitempty"`
	Score   float64  `json:"score"`
	Tags    []string `json:"tags,omitempty"`
}

type IngestResult struct {
	Accepted int `json:"accepted"`
	Rejected int `json:"rejected"`
}
