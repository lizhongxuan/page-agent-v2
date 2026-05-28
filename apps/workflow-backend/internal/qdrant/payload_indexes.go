package qdrant

import "strings"

func commonPayloadIndexes() []PayloadIndex {
	return []PayloadIndex{
		{FieldName: "project_id", FieldSchema: "keyword"},
		{FieldName: "tenant_id", FieldSchema: "keyword"},
		{FieldName: "doc_type", FieldSchema: "keyword"},
		{FieldName: "status", FieldSchema: "keyword"},
		{FieldName: "site", FieldSchema: "keyword"},
		{FieldName: "app", FieldSchema: "keyword"},
		{FieldName: "risk_level", FieldSchema: "keyword"},
		{FieldName: "updated_at", FieldSchema: "datetime"},
		{FieldName: "success_rate", FieldSchema: "float"},
		{FieldName: "recent_failure_count", FieldSchema: "integer"},
		{FieldName: "last_failure_at", FieldSchema: "datetime"},
	}
}

func workflowCardPayloadIndexes() []PayloadIndex {
	return append(commonPayloadIndexes(),
		PayloadIndex{FieldName: "workflow_id", FieldSchema: "keyword"},
		PayloadIndex{FieldName: "version", FieldSchema: "integer"},
		PayloadIndex{FieldName: "searchable", FieldSchema: "bool"},
		PayloadIndex{FieldName: "requires_confirmation", FieldSchema: "bool"},
		PayloadIndex{FieldName: "tags", FieldSchema: "keyword"},
		PayloadIndex{FieldName: "start_page_states", FieldSchema: "keyword"},
		PayloadIndex{FieldName: "end_page_states", FieldSchema: "keyword"},
		PayloadIndex{FieldName: "variable_names", FieldSchema: "keyword"},
		PayloadIndex{FieldName: "action_types", FieldSchema: "keyword"},
		PayloadIndex{FieldName: "intent", FieldSchema: "text"},
		PayloadIndex{FieldName: "description", FieldSchema: "text"},
		PayloadIndex{FieldName: "examples_text", FieldSchema: "text"},
	)
}

func workflowChunkPayloadIndexes() []PayloadIndex {
	return append(commonPayloadIndexes(),
		PayloadIndex{FieldName: "workflow_id", FieldSchema: "keyword"},
		PayloadIndex{FieldName: "version", FieldSchema: "integer"},
		PayloadIndex{FieldName: "chunk_id", FieldSchema: "keyword"},
		PayloadIndex{FieldName: "from_page_state", FieldSchema: "keyword"},
		PayloadIndex{FieldName: "to_page_state", FieldSchema: "keyword"},
		PayloadIndex{FieldName: "target_roles", FieldSchema: "keyword"},
		PayloadIndex{FieldName: "target_names", FieldSchema: "text"},
		PayloadIndex{FieldName: "variable_names", FieldSchema: "keyword"},
		PayloadIndex{FieldName: "step_summary", FieldSchema: "text"},
		PayloadIndex{FieldName: "selector_health", FieldSchema: "float"},
	)
}

func pageStatePayloadIndexes() []PayloadIndex {
	return append(commonPayloadIndexes(),
		PayloadIndex{FieldName: "page_state_id", FieldSchema: "keyword"},
		PayloadIndex{FieldName: "url_pattern", FieldSchema: "text"},
		PayloadIndex{FieldName: "required_text", FieldSchema: "text"},
		PayloadIndex{FieldName: "canonical_title", FieldSchema: "text"},
	)
}

func interruptHandlerPayloadIndexes() []PayloadIndex {
	return append(commonPayloadIndexes(),
		PayloadIndex{FieldName: "handler_id", FieldSchema: "keyword"},
		PayloadIndex{FieldName: "workflow_id", FieldSchema: "keyword"},
		PayloadIndex{FieldName: "version", FieldSchema: "integer"},
		PayloadIndex{FieldName: "interrupt_type", FieldSchema: "keyword"},
		PayloadIndex{FieldName: "applies_to_page_states", FieldSchema: "keyword"},
		PayloadIndex{FieldName: "fingerprint_text", FieldSchema: "text"},
		PayloadIndex{FieldName: "target_names", FieldSchema: "text"},
	)
}

func repairPatchPayloadIndexes() []PayloadIndex {
	return append(commonPayloadIndexes(),
		PayloadIndex{FieldName: "patch_id", FieldSchema: "keyword"},
		PayloadIndex{FieldName: "workflow_id", FieldSchema: "keyword"},
		PayloadIndex{FieldName: "workflow_version", FieldSchema: "integer"},
		PayloadIndex{FieldName: "chunk_id", FieldSchema: "keyword"},
		PayloadIndex{FieldName: "step_id", FieldSchema: "keyword"},
		PayloadIndex{FieldName: "failure_type", FieldSchema: "keyword"},
		PayloadIndex{FieldName: "failure_signature", FieldSchema: "text"},
		PayloadIndex{FieldName: "applies_to_page_states", FieldSchema: "keyword"},
		PayloadIndex{FieldName: "target_names", FieldSchema: "text"},
	)
}

func requiredHighFrequencyIndexes(collection string) []string {
	common := []string{"project_id", "doc_type", "status", "site", "risk_level", "updated_at", "success_rate"}
	switch {
	case strings.HasSuffix(collection, "_workflow_cards"):
		return append(common, "workflow_id", "searchable", "variable_names", "action_types")
	case strings.HasSuffix(collection, "_workflow_chunks"):
		return append(common, "workflow_id", "chunk_id", "from_page_state", "to_page_state", "variable_names", "target_names")
	case strings.HasSuffix(collection, "_page_states"):
		return append(common, "page_state_id", "url_pattern")
	case strings.HasSuffix(collection, "_interrupt_handlers"):
		return append(common, "handler_id", "applies_to_page_states", "interrupt_type", "target_names")
	case strings.HasSuffix(collection, "_repair_patches"):
		return append(common, "patch_id", "workflow_id", "failure_type", "failure_signature", "applies_to_page_states")
	default:
		return common
	}
}
