package main

import (
	"context"
	"encoding/json"
	"log"
	"os"

	"github.com/page-agent/workflow-backend/internal/artifact"
	"github.com/page-agent/workflow-backend/internal/config"
)

func main() {
	cfg := config.Load()
	service := artifact.NewService(artifact.NewLocalStore(cfg.DataDir), artifact.NewMemoryRepository())
	report, err := artifact.VerifyArtifacts(context.Background(), service)
	if err != nil {
		log.Fatal(err)
	}
	if err := json.NewEncoder(os.Stdout).Encode(report); err != nil {
		log.Fatal(err)
	}
}
