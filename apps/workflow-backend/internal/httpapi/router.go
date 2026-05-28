package httpapi

import (
	"context"
	"net/http"

	"github.com/page-agent/workflow-backend/internal/config"
	"github.com/page-agent/workflow-backend/internal/registry"
	"github.com/page-agent/workflow-backend/internal/replay"
	"github.com/page-agent/workflow-backend/internal/retrieval"
	"github.com/page-agent/workflow-backend/internal/selector"
)

type WorkflowSearchService interface {
	Search(context.Context, retrieval.SearchRequest) ([]retrieval.WorkflowCandidate, error)
}

type WorkflowSelectorService interface {
	Select(context.Context, selector.SelectionInput) (selector.SelectionResult, error)
}

type WorkflowIndexer interface {
	IndexWorkflow(context.Context, string) error
	Rebuild(context.Context, string) (int, error)
}

type Services struct {
	Candidates     *registry.CandidateService
	RepairPatches  *registry.RepairPatchService
	Registry       registry.Repository
	WorkflowSearch WorkflowSearchService
	Selector       WorkflowSelectorService
	Indexer        WorkflowIndexer
	Runs           *replay.Service
}

func NewRouter(cfg config.Config) http.Handler {
	return NewRouterWithServices(cfg, Services{})
}

func NewRouterWithServices(cfg config.Config, services Services) http.Handler {
	mux := http.NewServeMux()
	registerHealthRoutes(mux, cfg)
	if services.Candidates != nil {
		registerCandidateRoutes(mux, services.Candidates, services.Indexer)
	}
	if services.RepairPatches != nil {
		registerRepairPatchRoutes(mux, services.RepairPatches)
	}
	registerRetrievalRoutes(mux, retrievalRouteServices{
		repo:     services.Registry,
		search:   services.WorkflowSearch,
		selector: services.Selector,
		indexer:  services.Indexer,
	})
	if services.Runs != nil {
		registerRunRoutes(mux, services.Runs, services.Registry)
	}
	return mux
}
