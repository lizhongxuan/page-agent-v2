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
	"github.com/page-agent/workflow-backend/internal/retrieval"
)

func main() {
	cfg := config.Load()
	ctx := context.Background()
	repo, err := buildRepository(ctx, cfg)
	if err != nil {
		log.Fatal(err)
	}
	vectorizer := buildVectorizer(cfg)
	indexer := buildIndexer(ctx, cfg, repo, vectorizer)
	server := &http.Server{
		Addr: cfg.BackendAddr,
		Handler: httpapi.NewRouterWithServices(cfg, httpapi.Services{
			Registry:            repo,
			Indexer:             indexer,
			KnowledgeVectorizer: vectorizer,
		}),
	}
	log.Printf("page-agent workflow backend listening on %s", cfg.BackendAddr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func buildRepository(ctx context.Context, cfg config.Config) (registry.Repository, error) {
	if cfg.StorageBackend == "postgres" {
		return registry.NewPostgresRepository(ctx, cfg.PostgresURL)
	}
	return registry.NewFileRepository(cfg.DataDir)
}

func buildIndexer(
	ctx context.Context,
	cfg config.Config,
	repo registry.Repository,
	vectorizer retrieval.Vectorizer,
) httpapi.WorkflowIndexer {
	if cfg.DisableQdrant {
		log.Printf("workflow backend qdrant disabled; workflow indexer unavailable")
		return nil
	}
	qdrantClient := qdrant.NewClient(qdrant.ClientConfig{
		BaseURL: cfg.QdrantURL,
		APIKey:  cfg.QdrantAPIKey,
	})
	collections := qdrant.NewCollectionManager(qdrantClient, cfg.QdrantCollectionPrefix)
	if err := collections.EnsureCollections(ctx); err != nil {
		log.Printf("qdrant collection setup failed: %v", err)
	}
	indexer := indexing.NewSyncServiceWithVectorizer(
		repo,
		qdrantClient,
		collections.CollectionNames(),
		vectorizer,
	)
	return indexer
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
