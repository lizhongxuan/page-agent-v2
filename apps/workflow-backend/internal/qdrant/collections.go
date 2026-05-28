package qdrant

import (
	"context"
	"fmt"
	"strings"
)

type CollectionNames struct {
	WorkflowCards     string
	WorkflowChunks    string
	PageStates        string
	InterruptHandlers string
	RepairPatches     string
}

type CollectionDefinition struct {
	Name           string
	PayloadIndexes []PayloadIndex
}

type CollectionManager struct {
	client      *Client
	prefix      string
	definitions []CollectionDefinition
}

func NewCollectionManager(client *Client, prefix string) *CollectionManager {
	normalized := strings.Trim(strings.TrimSpace(prefix), "_")
	if normalized == "" {
		normalized = "pa"
	}
	return &CollectionManager{
		client:      client,
		prefix:      normalized,
		definitions: DefaultCollectionDefinitions(normalized),
	}
}

func (manager *CollectionManager) CollectionNames() CollectionNames {
	return collectionNames(manager.prefix)
}

func (manager *CollectionManager) EnsureCollections(ctx context.Context) error {
	if err := ValidateCollectionDefinitions(manager.definitions); err != nil {
		return err
	}
	for _, definition := range manager.definitions {
		exists, err := manager.client.CollectionExists(ctx, definition.Name)
		if err != nil {
			return fmt.Errorf("check qdrant collection %s: %w", definition.Name, err)
		}
		if !exists {
			if err := manager.client.CreateCollection(ctx, definition.Name, collectionCreateBody()); err != nil {
				return fmt.Errorf("create qdrant collection %s: %w", definition.Name, err)
			}
		}
		for _, index := range definition.PayloadIndexes {
			if err := manager.client.CreatePayloadIndex(ctx, definition.Name, index); err != nil {
				return fmt.Errorf("create qdrant payload index %s.%s: %w", definition.Name, index.FieldName, err)
			}
		}
	}
	return nil
}

func DefaultCollectionDefinitions(prefix string) []CollectionDefinition {
	names := collectionNames(prefix)
	return []CollectionDefinition{
		{Name: names.WorkflowCards, PayloadIndexes: workflowCardPayloadIndexes()},
		{Name: names.WorkflowChunks, PayloadIndexes: workflowChunkPayloadIndexes()},
		{Name: names.PageStates, PayloadIndexes: pageStatePayloadIndexes()},
		{Name: names.InterruptHandlers, PayloadIndexes: interruptHandlerPayloadIndexes()},
		{Name: names.RepairPatches, PayloadIndexes: repairPatchPayloadIndexes()},
	}
}

func ValidateCollectionDefinitions(definitions []CollectionDefinition) error {
	for _, definition := range definitions {
		indexed := map[string]bool{}
		for _, index := range definition.PayloadIndexes {
			indexed[index.FieldName] = true
		}
		for _, field := range requiredHighFrequencyIndexes(definition.Name) {
			if !indexed[field] {
				return fmt.Errorf("collection %s missing required payload index %s", definition.Name, field)
			}
		}
	}
	return nil
}

func collectionNames(prefix string) CollectionNames {
	return CollectionNames{
		WorkflowCards:     prefix + "_workflow_cards",
		WorkflowChunks:    prefix + "_workflow_chunks",
		PageStates:        prefix + "_page_states",
		InterruptHandlers: prefix + "_interrupt_handlers",
		RepairPatches:     prefix + "_repair_patches",
	}
}

func collectionCreateBody() map[string]any {
	return map[string]any{
		"vectors": map[string]any{
			"intent_dense": map[string]any{"size": 1024, "distance": "Cosine"},
			"page_dense":   map[string]any{"size": 1024, "distance": "Cosine"},
		},
		"sparse_vectors": map[string]any{
			"lexical_sparse": map[string]any{},
		},
	}
}
