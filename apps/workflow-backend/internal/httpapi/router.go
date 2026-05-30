package httpapi

import (
	"context"
	"net/http"

	"github.com/page-agent/workflow-backend/internal/config"
	"github.com/page-agent/workflow-backend/internal/registry"
)

type WorkflowIndexer interface {
	IndexWorkflow(context.Context, string) error
	Rebuild(context.Context, string) (int, error)
}

type KnowledgeVectorizer interface {
	DenseQuery(context.Context, string) ([]float32, error)
}

type Services struct {
	Registry            registry.Repository
	Indexer             WorkflowIndexer
	KnowledgeVectorizer KnowledgeVectorizer
}

func NewRouter(cfg config.Config) http.Handler {
	return NewRouterWithServices(cfg, Services{})
}

func NewRouterWithServices(cfg config.Config, services Services) http.Handler {
	mux := http.NewServeMux()
	registerHealthRoutes(mux, cfg)
	registerAdminRoutes(mux, cfg, services.Indexer)
	registerMemoryRoutes(mux, cfg, services.Registry, services.KnowledgeVectorizer)
	registerTaskRunRoutes(mux, services.Registry)
	registerKnowledgeRoutes(mux, services.Registry, services.KnowledgeVectorizer)
	return mux
}
