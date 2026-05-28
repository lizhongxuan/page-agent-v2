package main

import (
	"encoding/json"
	"log"
	"os"

	"github.com/page-agent/workflow-backend/internal/mcp"
)

func main() {
	server := mcp.NewServer(mcp.Dependencies{})
	if err := json.NewEncoder(os.Stdout).Encode(map[string]any{
		"name":  "page-agent-workflow-memory",
		"tools": server.Registry.ToolNames(),
	}); err != nil {
		log.Fatal(err)
	}
}
