package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"github.com/page-agent/workflow-backend/internal/artifact"
	"github.com/page-agent/workflow-backend/internal/knowledge"
	"github.com/page-agent/workflow-backend/internal/recipe"
	"github.com/page-agent/workflow-backend/internal/workflow"
)

type Dependencies struct {
	Workflow  *workflow.Service
	Knowledge *knowledge.Service
	Artifact  *artifact.Service
}

type ToolFunc func(context.Context, map[string]any) (any, error)

type Registry struct {
	deps  Dependencies
	tools map[string]ToolFunc
}

type PreparedWorkflowRun struct {
	WorkflowID       string                   `json:"workflowId"`
	WorkflowVersion  int                      `json:"workflowVersion"`
	WorkflowName     string                   `json:"workflowName"`
	Chunks           []workflow.WorkflowChunk `json:"chunks"`
	ControlsBrowser  bool                     `json:"controlsBrowser"`
	ExecutionMessage string                   `json:"executionMessage"`
}

func NewRegistry(deps Dependencies) *Registry {
	registry := &Registry{deps: deps, tools: map[string]ToolFunc{}}
	registry.tools["search_site_knowledge"] = registry.searchSiteKnowledge
	registry.tools["search_workflows"] = registry.searchWorkflows
	registry.tools["get_workflow"] = registry.getWorkflow
	registry.tools["prepare_workflow_run"] = registry.prepareWorkflowRun
	registry.tools["store_run_artifact"] = registry.storeRunArtifact
	registry.tools["get_run_artifacts"] = registry.getRunArtifacts
	registry.tools["create_workflow_candidate"] = notImplemented("create_workflow_candidate")
	registry.tools["save_workflow"] = notImplemented("save_workflow")
	registry.tools["record_workflow_run"] = notImplemented("record_workflow_run")
	registry.tools["import_workflow_use"] = registry.importWorkflowUse
	registry.tools["export_workflow_use"] = registry.exportWorkflowUse
	return registry
}

func (registry *Registry) ToolNames() []string {
	names := make([]string, 0, len(registry.tools))
	for name := range registry.tools {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (registry *Registry) Call(ctx context.Context, name string, input map[string]any) (any, error) {
	tool, ok := registry.tools[name]
	if !ok {
		return nil, fmt.Errorf("unknown tool %q", name)
	}
	return tool(ctx, input)
}

func (registry *Registry) searchSiteKnowledge(ctx context.Context, input map[string]any) (any, error) {
	if registry.deps.Knowledge == nil {
		return nil, errors.New("knowledge service is not configured")
	}
	var request knowledge.SearchRequest
	if err := decodeMap(input, &request); err != nil {
		return nil, err
	}
	return registry.deps.Knowledge.Search(ctx, request)
}

func (registry *Registry) searchWorkflows(ctx context.Context, input map[string]any) (any, error) {
	if registry.deps.Workflow == nil {
		return nil, errors.New("workflow service is not configured")
	}
	var request workflow.SearchRequest
	if err := decodeMap(input, &request); err != nil {
		return nil, err
	}
	return registry.deps.Workflow.Search(ctx, request)
}

func (registry *Registry) getWorkflow(ctx context.Context, input map[string]any) (any, error) {
	if registry.deps.Workflow == nil {
		return nil, errors.New("workflow service is not configured")
	}
	id, _ := input["id"].(string)
	if id == "" {
		id, _ = input["workflowId"].(string)
	}
	if id == "" {
		return nil, errors.New("workflow id is required")
	}
	return registry.deps.Workflow.Get(ctx, id)
}

func (registry *Registry) prepareWorkflowRun(ctx context.Context, input map[string]any) (any, error) {
	resultsAny, err := registry.searchWorkflows(ctx, input)
	if err != nil {
		return nil, err
	}
	results := resultsAny.([]workflow.SearchResult)
	if len(results) == 0 {
		return PreparedWorkflowRun{ControlsBrowser: false, ExecutionMessage: "no matching workflow"}, nil
	}
	recipe := results[0].Recipe
	return PreparedWorkflowRun{
		WorkflowID:       recipe.ID,
		WorkflowVersion:  recipe.Version,
		WorkflowName:     recipe.Name,
		Chunks:           recipe.Chunks,
		ControlsBrowser:  false,
		ExecutionMessage: "plan only; extension executes in the current Chrome tab",
	}, nil
}

func (registry *Registry) storeRunArtifact(ctx context.Context, input map[string]any) (any, error) {
	if registry.deps.Artifact == nil {
		return nil, errors.New("artifact service is not configured")
	}
	content, _ := input["content"].(string)
	request := artifact.PutRequest{
		ProjectID:   stringValue(input, "projectId"),
		OwnerType:   artifact.OwnerType(stringValue(input, "ownerType")),
		OwnerID:     stringValue(input, "ownerId"),
		Kind:        artifact.ArtifactKind(stringValue(input, "kind")),
		ContentType: stringValue(input, "contentType"),
		Reader:      bytes.NewBufferString(content),
	}
	return registry.deps.Artifact.Store(ctx, request)
}

func (registry *Registry) getRunArtifacts(ctx context.Context, input map[string]any) (any, error) {
	if registry.deps.Artifact == nil {
		return nil, errors.New("artifact service is not configured")
	}
	return registry.deps.Artifact.List(ctx, artifact.ListQuery{
		ProjectID: stringValue(input, "projectId"),
		OwnerType: artifact.OwnerType(stringValue(input, "ownerType")),
		OwnerID:   stringValue(input, "ownerId"),
	})
}

func (registry *Registry) importWorkflowUse(_ context.Context, input map[string]any) (any, error) {
	source, _ := input["source"].(string)
	if source == "" {
		source, _ = input["content"].(string)
	}
	if source == "" {
		return nil, errors.New("workflow-use source is required")
	}
	return recipe.ImportWorkflowUse([]byte(source), recipe.ImportWorkflowUseOptions{
		ProjectID: stringValue(input, "projectId"),
	})
}

func (registry *Registry) exportWorkflowUse(ctx context.Context, input map[string]any) (any, error) {
	if registry.deps.Workflow == nil {
		return nil, errors.New("workflow service is not configured")
	}
	id, _ := input["id"].(string)
	if id == "" {
		id, _ = input["workflowId"].(string)
	}
	if id == "" {
		return nil, errors.New("workflow id is required")
	}
	recipeDraft, err := registry.deps.Workflow.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return recipe.ExportWorkflowUse(recipeDraft), nil
}

func notImplemented(name string) ToolFunc {
	return func(context.Context, map[string]any) (any, error) {
		return nil, fmt.Errorf("%s is not implemented in this backend slice yet", name)
	}
}

func decodeMap(input map[string]any, output any) error {
	data, err := json.Marshal(input)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, output)
}

func stringValue(input map[string]any, key string) string {
	value, _ := input[key].(string)
	return value
}
