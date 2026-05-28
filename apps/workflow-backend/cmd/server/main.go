package main

import (
	"context"
	"log"
	"net/http"

	"github.com/page-agent/workflow-backend/internal/config"
	"github.com/page-agent/workflow-backend/internal/embedding"
	"github.com/page-agent/workflow-backend/internal/httpapi"
	"github.com/page-agent/workflow-backend/internal/indexing"
	"github.com/page-agent/workflow-backend/internal/qdrant"
	"github.com/page-agent/workflow-backend/internal/registry"
	"github.com/page-agent/workflow-backend/internal/replay"
	"github.com/page-agent/workflow-backend/internal/retrieval"
	"github.com/page-agent/workflow-backend/internal/selector"
)

func main() {
	cfg := config.Load()
	repo, err := registry.NewFileRepository(cfg.DataDir)
	if err != nil {
		log.Fatal(err)
	}
	qdrantClient := qdrant.NewClient(qdrant.ClientConfig{
		BaseURL: cfg.QdrantURL,
		APIKey:  cfg.QdrantAPIKey,
	})
	collections := qdrant.NewCollectionManager(qdrantClient, cfg.QdrantCollectionPrefix)
	if err := collections.EnsureCollections(context.Background()); err != nil {
		log.Printf("qdrant collection setup failed: %v", err)
	}
	vectorizer := buildVectorizer(cfg)
	indexer := indexing.NewSyncServiceWithVectorizer(
		repo,
		qdrantClient,
		collections.CollectionNames(),
		vectorizer,
	)
	server := &http.Server{
		Addr: cfg.BackendAddr,
		Handler: httpapi.NewRouterWithServices(cfg, httpapi.Services{
			Candidates:    registry.NewCandidateService(repo),
			RepairPatches: registry.NewRepairPatchService(repo),
			Registry:      repo,
			WorkflowSearch: retrieval.NewSearchService(retrieval.SearchServiceConfig{
				Repository:  repo,
				Qdrant:      qdrantClient,
				Collections: collections.CollectionNames(),
				Vectorizer:  vectorizer,
			}),
			Selector: selector.NewService(buildSelector(cfg)),
			Indexer:  indexer,
			Runs: replay.NewService(repo, replay.NewPlaywrightRunner(replay.PlaywrightRunnerConfig{
				Headless: true,
			})),
		}),
	}
	log.Printf("page-agent workflow backend listening on %s", cfg.BackendAddr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func buildVectorizer(cfg config.Config) retrieval.Vectorizer {
	if cfg.EmbeddingBaseURL == "" {
		return retrieval.DeterministicVectorizer{}
	}
	dense := embedding.NewOpenAICompatibleClient(
		cfg.EmbeddingBaseURL,
		cfg.EmbeddingAPIKey,
		cfg.EmbeddingModel,
		nil,
	)
	sparse := embedding.DeterministicSparseProvider{Model: cfg.SparseEmbeddingModel}
	return retrieval.NewEmbeddingVectorizer(
		dense,
		sparse,
		embedding.NewCache(),
		cfg.EmbeddingModel,
		cfg.SparseEmbeddingModel,
	)
}

func buildSelector(cfg config.Config) selector.LLMSelector {
	if cfg.LLMBaseURL == "" {
		return selector.HeuristicSelector{}
	}
	return selector.NewOpenAICompatibleSelector(cfg.LLMBaseURL, cfg.LLMAPIKey, cfg.LLMModel, nil)
}
