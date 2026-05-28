package main

import (
	"errors"
	"log"
	"net/http"
	"path/filepath"

	"github.com/page-agent/workflow-backend/internal/artifact"
	"github.com/page-agent/workflow-backend/internal/config"
	"github.com/page-agent/workflow-backend/internal/httpapi"
	"github.com/page-agent/workflow-backend/internal/knowledge"
	"github.com/page-agent/workflow-backend/internal/recipe"
	"github.com/page-agent/workflow-backend/internal/selector"
	"github.com/page-agent/workflow-backend/internal/workflow"
)

func main() {
	cfg := config.Load()
	workflowRepo, err := workflow.NewFileRepository(filepath.Join(cfg.DataDir, "workflows.json"))
	if err != nil {
		log.Fatal(err)
	}
	candidateRepo, err := recipe.NewFileCandidateRepository(filepath.Join(cfg.DataDir, "workflow_candidates.json"))
	if err != nil {
		log.Fatal(err)
	}
	workflowService := workflow.NewService(workflowRepo)
	server := &http.Server{
		Addr: cfg.Addr,
		Handler: httpapi.NewRouter(httpapi.Dependencies{
			Knowledge: knowledge.NewService(knowledge.NewMemoryRepository(), knowledge.DeterministicEmbedder{}),
			Workflow:  workflowService,
			Candidate: recipe.NewCandidateService(candidateRepo, workflowService),
			Selector:  selector.NewService(selector.NewMemoryRepository()),
			Artifact:  artifact.NewService(artifact.NewLocalStore(cfg.DataDir), artifact.NewMemoryRepository()),
		}),
	}

	log.Printf("workflow backend listening on http://%s", cfg.Addr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
