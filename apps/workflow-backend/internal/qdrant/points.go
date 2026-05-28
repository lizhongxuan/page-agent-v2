package qdrant

import (
	"crypto/sha1"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/page-agent/workflow-backend/internal/registry"
)

func WorkflowCardFromRecipe(recipe registry.WorkflowRecipe, examples []string) registry.WorkflowCard {
	card := registry.WorkflowCard{
		WorkflowID:           recipe.ID,
		Version:              recipe.Version,
		ProjectID:            recipe.ProjectID,
		TenantID:             recipe.TenantID,
		Status:               recipe.Status,
		Searchable:           recipe.Searchable,
		Site:                 recipe.Site,
		App:                  recipe.App,
		Name:                 recipe.Name,
		Intent:               recipe.Intent,
		Description:          recipe.Description,
		Examples:             sanitizeList(examples),
		Tags:                 append([]string(nil), recipe.Tags...),
		StartPageStates:      append([]string(nil), recipe.StartPageStates...),
		EndPageStates:        append([]string(nil), recipe.EndPageStates...),
		VariableNames:        nonSensitiveVariableNames(recipe.Variables),
		ActionTypes:          actionTypes(recipe.Chunks),
		RiskLevel:            recipe.RiskLevel,
		RequiresConfirmation: recipe.RequiresConfirmation,
		UpdatedAt:            recipe.UpdatedAt,
	}
	if len(recipe.Chunks) > 0 {
		card.SuccessRate = averageSuccessRate(recipe.Chunks)
		card.RecentFailureCount = totalRecentFailureCount(recipe.Chunks)
		card.LastFailureAt = latestFailureAt(recipe.Chunks)
	}
	return card
}

func BuildWorkflowCardPoint(card registry.WorkflowCard) (Point, error) {
	if card.WorkflowID == "" || card.Version <= 0 {
		return Point{}, errors.New("workflow card point requires workflow id and version")
	}
	embeddingText := sanitizeText(card.EmbeddingText())
	payload := map[string]any{
		"doc_type":              "workflow_card",
		"project_id":            card.ProjectID,
		"tenant_id":             card.TenantID,
		"workflow_id":           card.WorkflowID,
		"version":               card.Version,
		"status":                string(card.Status),
		"searchable":            card.Searchable,
		"site":                  card.Site,
		"app":                   card.App,
		"risk_level":            string(card.RiskLevel),
		"requires_confirmation": card.RequiresConfirmation,
		"name":                  card.Name,
		"intent":                sanitizeText(card.Intent),
		"description":           sanitizeText(card.Description),
		"examples_text":         strings.Join(sanitizeList(card.Examples), "\n"),
		"tags":                  card.Tags,
		"start_page_states":     card.StartPageStates,
		"end_page_states":       card.EndPageStates,
		"variable_names":        card.VariableNames,
		"action_types":          card.ActionTypes,
		"success_rate":          card.SuccessRate,
		"recent_failure_count":  card.RecentFailureCount,
		"last_failure_at":       formatTime(card.LastFailureAt),
		"updated_at":            formatTime(card.UpdatedAt),
		"embedding_text":        embeddingText,
	}
	return Point{ID: stablePointID(fmt.Sprintf("workflow:%s:v%d", card.WorkflowID, card.Version)), Payload: payload}, nil
}

func BuildWorkflowChunkPoints(recipe registry.WorkflowRecipe) ([]Point, error) {
	points := make([]Point, 0, len(recipe.Chunks))
	sensitiveVars := sensitiveVariableNames(recipe.Variables)
	for _, chunk := range recipe.Chunks {
		if chunk.ID == "" {
			return nil, errors.New("workflow chunk point requires chunk id")
		}
		targetRoles, targetNames := targetFields(chunk.Steps)
		variableNames := filterSensitiveNames(chunk.VariableNames, sensitiveVars)
		stepSummary := sanitizeText(chunk.StepSummary)
		embeddingText := sanitizeText(strings.Join([]string{
			"Workflow: " + recipe.Name,
			"Chunk: " + chunk.Name,
			"From page: " + chunk.FromPageState,
			"To page: " + chunk.ToPageState,
			"Precondition: " + chunk.PreconditionText,
			"Postcondition: " + chunk.PostconditionText,
			"Steps: " + chunk.StepSummary,
			"Targets: " + strings.Join(targetNames, ", "),
			"Variables: " + strings.Join(variableNames, ", "),
			"Risk: " + string(chunk.RiskLevel),
		}, "\n"))
		payload := map[string]any{
			"doc_type":             "workflow_chunk",
			"project_id":           recipe.ProjectID,
			"tenant_id":            recipe.TenantID,
			"workflow_id":          recipe.ID,
			"version":              recipe.Version,
			"chunk_id":             chunk.ID,
			"status":               string(recipe.Status),
			"site":                 recipe.Site,
			"app":                  recipe.App,
			"risk_level":           string(chunk.RiskLevel),
			"from_page_state":      chunk.FromPageState,
			"to_page_state":        chunk.ToPageState,
			"precondition_text":    sanitizeText(chunk.PreconditionText),
			"postcondition_text":   sanitizeText(chunk.PostconditionText),
			"step_summary":         stepSummary,
			"target_roles":         targetRoles,
			"target_names":         targetNames,
			"variable_names":       variableNames,
			"success_rate":         chunk.SuccessRate,
			"selector_health":      chunk.SelectorHealth,
			"recent_failure_count": chunk.RecentFailureCount,
			"last_failure_at":      formatTime(chunk.LastFailureAt),
			"updated_at":           formatTime(recipe.UpdatedAt),
			"embedding_text":       embeddingText,
		}
		points = append(points, Point{ID: stablePointID(fmt.Sprintf("chunk:%s:v%d:%s", recipe.ID, recipe.Version, chunk.ID)), Payload: payload})
	}
	return points, nil
}

func BuildPageStatePoint(pageState registry.PageState) (Point, error) {
	if pageState.ID == "" || pageState.Site == "" {
		return Point{}, errors.New("page state point requires id and site")
	}
	controlText, roles, names := controlFields(pageState.RequiredControls)
	payload := map[string]any{
		"doc_type":          "page_state",
		"project_id":        pageState.ProjectID,
		"page_state_id":     pageState.ID,
		"status":            string(pageState.Status),
		"site":              pageState.Site,
		"app":               pageState.App,
		"url_pattern":       pageState.URLPattern,
		"required_text":     sanitizeList(pageState.RequiredText),
		"required_controls": pageState.RequiredControls,
		"target_roles":      roles,
		"target_names":      names,
		"canonical_title":   sanitizeText(pageState.CanonicalTitle),
		"updated_at":        formatTime(pageState.UpdatedAt),
		"embedding_text": sanitizeText(strings.Join([]string{
			pageState.CanonicalTitle,
			"URL pattern: " + pageState.URLPattern,
			"Visible text: " + strings.Join(pageState.RequiredText, ", "),
			"Controls: " + controlText,
		}, "\n")),
	}
	return Point{ID: stablePointID(fmt.Sprintf("page:%s:%s", pageState.Site, pageState.ID)), Payload: payload}, nil
}

func BuildInterruptHandlerPoint(handler registry.InterruptHandler) (Point, error) {
	if handler.ID == "" {
		return Point{}, errors.New("interrupt handler point requires id")
	}
	_, roles, names := controlFields(handler.TargetControls)
	payload := map[string]any{
		"doc_type":               "interrupt_handler",
		"project_id":             handler.ProjectID,
		"handler_id":             handler.ID,
		"workflow_id":            handler.WorkflowID,
		"version":                handler.Version,
		"status":                 string(handler.Status),
		"site":                   handler.Site,
		"app":                    handler.App,
		"applies_to_page_states": handler.AppliesToPageStates,
		"interrupt_type":         handler.InterruptType,
		"fingerprint_text":       sanitizeText(handler.FingerprintText),
		"required_text":          sanitizeList(handler.RequiredText),
		"target_roles":           roles,
		"target_names":           names,
		"risk_level":             string(handler.RiskLevel),
		"success_rate":           handler.SuccessRate,
		"updated_at":             formatTime(handler.UpdatedAt),
		"embedding_text": sanitizeText(strings.Join([]string{
			handler.FingerprintText,
			"Interrupt type: " + handler.InterruptType,
			"Targets: " + strings.Join(names, ", "),
			"Pages: " + strings.Join(handler.AppliesToPageStates, ", "),
		}, "\n")),
	}
	return Point{ID: stablePointID(fmt.Sprintf("interrupt:%s:v%d", handler.ID, handler.Version)), Payload: payload}, nil
}

func BuildRepairPatchPoint(patch registry.RepairPatch) (Point, error) {
	if patch.ID == "" {
		return Point{}, errors.New("repair patch point requires id")
	}
	payload := map[string]any{
		"doc_type":               "repair_patch",
		"project_id":             patch.ProjectID,
		"patch_id":               patch.ID,
		"status":                 string(patch.Status),
		"workflow_id":            patch.WorkflowID,
		"workflow_version":       patch.WorkflowVersion,
		"chunk_id":               patch.ChunkID,
		"step_id":                patch.StepID,
		"site":                   patch.Site,
		"app":                    patch.App,
		"risk_level":             string(patch.RiskLevel),
		"failure_type":           patch.FailureType,
		"failure_signature":      sanitizeText(patch.FailureSignature),
		"old_target":             sanitizeText(patch.OldTarget),
		"new_target_summary":     sanitizeText(patch.NewTargetSummary),
		"applies_to_page_states": patch.AppliesToPageStates,
		"success_rate":           patch.SuccessRate,
		"updated_at":             formatTime(patch.UpdatedAt),
		"embedding_text": sanitizeText(strings.Join([]string{
			"Failure type: " + patch.FailureType,
			"Failure signature: " + patch.FailureSignature,
			"Old target: " + patch.OldTarget,
			"New target: " + patch.NewTargetSummary,
			"Pages: " + strings.Join(patch.AppliesToPageStates, ", "),
		}, "\n")),
	}
	return Point{ID: stablePointID("repair:" + patch.ID), Payload: payload}, nil
}

func nonSensitiveVariableNames(variables []registry.Variable) []string {
	names := make([]string, 0, len(variables))
	for _, variable := range variables {
		if !variable.Sensitive {
			names = append(names, variable.Name)
		}
	}
	sort.Strings(names)
	return names
}

func sensitiveVariableNames(variables []registry.Variable) map[string]bool {
	names := map[string]bool{}
	for _, variable := range variables {
		if variable.Sensitive {
			names[variable.Name] = true
		}
	}
	return names
}

func filterSensitiveNames(names []string, sensitive map[string]bool) []string {
	filtered := make([]string, 0, len(names))
	for _, name := range names {
		if !sensitive[name] {
			filtered = append(filtered, name)
		}
	}
	sort.Strings(filtered)
	return filtered
}

func actionTypes(chunks []registry.WorkflowChunk) []string {
	seen := map[string]bool{}
	for _, chunk := range chunks {
		for _, step := range chunk.Steps {
			seen[string(step.Type)] = true
		}
	}
	return sortedKeys(seen)
}

func targetFields(steps []registry.WorkflowStep) ([]string, []string) {
	roles := map[string]bool{}
	names := map[string]bool{}
	for _, step := range steps {
		for _, target := range append([]registry.TargetCandidate{step.Target.Primary}, step.Target.Fallbacks...) {
			if target.Role != "" {
				roles[target.Role] = true
			}
			if target.Name != "" {
				names[sanitizeText(target.Name)] = true
			}
		}
	}
	return sortedKeys(roles), sortedKeys(names)
}

func controlFields(controls []registry.ControlSignature) (string, []string, []string) {
	roles := map[string]bool{}
	names := map[string]bool{}
	parts := make([]string, 0, len(controls))
	for _, control := range controls {
		if control.Role != "" {
			roles[control.Role] = true
		}
		if control.Name != "" {
			names[sanitizeText(control.Name)] = true
		}
		parts = append(parts, strings.TrimSpace(control.Name+" "+control.Role))
	}
	return strings.Join(parts, ", "), sortedKeys(roles), sortedKeys(names)
}

func sortedKeys(values map[string]bool) []string {
	keys := make([]string, 0, len(values))
	for value := range values {
		if value != "" {
			keys = append(keys, value)
		}
	}
	sort.Strings(keys)
	return keys
}

func stablePointID(key string) string {
	sum := sha1.Sum([]byte(key))
	bytes := sum[:16]
	bytes[6] = (bytes[6] & 0x0f) | 0x50
	bytes[8] = (bytes[8] & 0x3f) | 0x80
	return fmt.Sprintf(
		"%x-%x-%x-%x-%x",
		bytes[0:4],
		bytes[4:6],
		bytes[6:8],
		bytes[8:10],
		bytes[10:16],
	)
}

func averageSuccessRate(chunks []registry.WorkflowChunk) float64 {
	var total float64
	var count float64
	for _, chunk := range chunks {
		if chunk.SuccessRate > 0 {
			total += chunk.SuccessRate
			count++
		}
	}
	if count == 0 {
		return 0
	}
	return total / count
}

func totalRecentFailureCount(chunks []registry.WorkflowChunk) int {
	total := 0
	for _, chunk := range chunks {
		total += chunk.RecentFailureCount
	}
	return total
}

func latestFailureAt(chunks []registry.WorkflowChunk) time.Time {
	var latest time.Time
	for _, chunk := range chunks {
		if chunk.LastFailureAt.After(latest) {
			latest = chunk.LastFailureAt
		}
	}
	return latest
}

func sanitizeList(values []string) []string {
	sanitized := make([]string, 0, len(values))
	for _, value := range values {
		clean := sanitizeText(value)
		if clean != "" && clean != "[REDACTED]" {
			sanitized = append(sanitized, clean)
		}
	}
	return sanitized
}

var sensitiveTextPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\bsk-[a-z0-9_-]{8,}`),
	regexp.MustCompile(`(?i)\b(token|api[_-]?key|password|passwd|secret)\b\s*[:=]\s*\S+`),
	regexp.MustCompile(`(?i)\b\d{6}\b.*\b(mfa|otp|验证码|verification)\b`),
}

func sanitizeText(value string) string {
	clean := strings.TrimSpace(value)
	for _, pattern := range sensitiveTextPatterns {
		clean = pattern.ReplaceAllString(clean, "[REDACTED]")
	}
	return clean
}

func formatTime(value interface {
	IsZero() bool
	Format(string) string
}) string {
	if value.IsZero() {
		return ""
	}
	return value.Format("2006-01-02T15:04:05Z07:00")
}
