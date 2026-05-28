package artifact

import (
	"context"
	"io"
	"time"
)

type ArtifactKind string

const (
	KindSession           ArtifactKind = "session"
	KindBrowserState      ArtifactKind = "browser_state"
	KindDOMSnapshot       ArtifactKind = "dom_snapshot"
	KindWorkflowUseSource ArtifactKind = "workflow_use_source"
	KindDebugBundle       ArtifactKind = "debug_bundle"
)

type OwnerType string

const (
	OwnerWorkflowRun       OwnerType = "workflow_run"
	OwnerWorkflowCandidate OwnerType = "workflow_candidate"
	OwnerWorkflowVersion   OwnerType = "workflow_version"
	OwnerImport            OwnerType = "import"
)

type PutRequest struct {
	ProjectID   string
	OwnerType   OwnerType
	OwnerID     string
	Kind        ArtifactKind
	ContentType string
	Reader      io.Reader
}

type Artifact struct {
	ID             string       `json:"id"`
	ProjectID      string       `json:"projectId"`
	OwnerType      OwnerType    `json:"ownerType"`
	OwnerID        string       `json:"ownerId"`
	Kind           ArtifactKind `json:"kind"`
	ContentType    string       `json:"contentType"`
	SizeBytes      int64        `json:"sizeBytes"`
	SHA256         string       `json:"sha256"`
	StorageBackend string       `json:"storageBackend"`
	RelativePath   string       `json:"relativePath"`
	CreatedAt      time.Time    `json:"createdAt"`
	ExpiresAt      *time.Time   `json:"expiresAt,omitempty"`
}

type Store interface {
	Put(context.Context, PutRequest) (Artifact, error)
	Get(context.Context, Artifact) (io.ReadCloser, error)
	Delete(context.Context, Artifact) error
}

type Repository interface {
	Save(context.Context, Artifact) error
	List(context.Context, ListQuery) ([]Artifact, error)
	ListAll(context.Context) ([]Artifact, error)
	Get(context.Context, string) (Artifact, error)
	Delete(context.Context, string) error
}

type ListQuery struct {
	ProjectID string
	OwnerType OwnerType
	OwnerID   string
}
