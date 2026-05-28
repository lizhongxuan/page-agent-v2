package httpapi

import (
	"context"
	"net/http"

	"github.com/page-agent/workflow-backend/internal/artifact"
	"github.com/page-agent/workflow-backend/internal/auth"
	"github.com/page-agent/workflow-backend/internal/knowledge"
	"github.com/page-agent/workflow-backend/internal/recipe"
	"github.com/page-agent/workflow-backend/internal/selector"
	"github.com/page-agent/workflow-backend/internal/workflow"
)

type Pinger interface {
	PingContext(context.Context) error
}

type Dependencies struct {
	DB        Pinger
	Auth      *auth.Middleware
	Knowledge *knowledge.Service
	Workflow  *workflow.Service
	Candidate *recipe.CandidateService
	Selector  *selector.Service
	Artifact  *artifact.Service
}

func NewRouter(deps Dependencies) http.Handler {
	mux := http.NewServeMux()
	registerHealthRoutes(mux, deps)
	registerKnowledgeRoutes(mux, deps.Knowledge)
	registerWorkflowRoutes(mux, deps.Workflow)
	registerCandidateRoutes(mux, deps.Candidate)
	registerRunRoutes(mux, deps.Selector)
	registerArtifactRoutes(mux, deps.Artifact)
	if deps.Auth != nil {
		return deps.Auth.Wrap(mux)
	}
	return mux
}
