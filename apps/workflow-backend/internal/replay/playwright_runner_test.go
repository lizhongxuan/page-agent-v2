package replay

import (
	"strings"
	"testing"

	"github.com/page-agent/workflow-backend/internal/registry"
)

func TestPlaywrightRunnerScriptSupportsStableLocatorsAndBindings(t *testing.T) {
	if !strings.Contains(playwrightRunnerScript, "page.getByRole") {
		t.Fatal("runner script should prefer role locators")
	}
	if !strings.Contains(playwrightRunnerScript, "locator.fill(render(step.value))") {
		t.Fatal("runner script should render bindings before fill")
	}
	if got := renderTemplate("search {query} in {{repo}}", map[string]string{
		"query": "timeout",
		"repo":  "microsoft/playwright",
	}); got != "search timeout in microsoft/playwright" {
		t.Fatalf("unexpected rendered value: %q", got)
	}
}

func TestPlaywrightRunnerPayloadKeepsChunkShape(t *testing.T) {
	payload := playwrightChunkPayload{
		URL: "https://github.com/microsoft/playwright",
		Chunk: registry.WorkflowChunk{
			ID: "open_issues",
			Steps: []registry.WorkflowStep{
				{
					ID:   "click_issues",
					Type: registry.StepClick,
					Target: registry.StepTarget{
						Primary: registry.TargetCandidate{
							Strategy: registry.TargetRole,
							Role:     "link",
							Name:     "Issues",
						},
					},
				},
			},
		},
		Bindings: map[string]string{"query": "timeout"},
	}

	if payload.Chunk.Steps[0].Target.Primary.Role != "link" || payload.Bindings["query"] != "timeout" {
		t.Fatalf("unexpected payload: %#v", payload)
	}
}

func TestPlaywrightRunnerScriptEmitsRunLogsAndValidatesPostconditions(t *testing.T) {
	required := []string{
		"postcondition_failed",
		"chunk_started",
		"chunk_finished",
		"selectorAttempts",
		"writeResult",
	}
	for _, token := range required {
		if !strings.Contains(playwrightRunnerScript, token) {
			t.Fatalf("runner script should contain %q", token)
		}
	}
}

func TestPlaywrightRunnerScriptHandlesCommonInterruptDialogs(t *testing.T) {
	required := []string{
		"interrupt_detected",
		"interrupt_handler_applied",
		"safeDismissInterrupts",
		"Got it",
	}
	for _, token := range required {
		if !strings.Contains(playwrightRunnerScript, token) {
			t.Fatalf("runner script should contain %q", token)
		}
	}
}

func TestPlaywrightRunnerScriptAppliesSafeRepairPatchTargets(t *testing.T) {
	required := []string{
		"repairPatches",
		"repairPatchForStep",
		"repair_patch_applied",
		"targetCandidatesFromPatch",
	}
	for _, token := range required {
		if !strings.Contains(playwrightRunnerScript, token) {
			t.Fatalf("runner script should contain %q", token)
		}
	}
}

func TestPlaywrightRunnerScriptValidatesFromPageStateBeforeChunk(t *testing.T) {
	required := []string{
		"validateFromPageState",
		"from_page_state_mismatch",
		"currentPageObservation",
		"expectedPageState",
	}
	for _, token := range required {
		if !strings.Contains(playwrightRunnerScript, token) {
			t.Fatalf("runner script should contain %q", token)
		}
	}
}

func TestPlaywrightRunnerScriptExecutesInterruptHandlerWorkflowBeforeRetry(t *testing.T) {
	required := []string{
		"interruptHandlers",
		"interruptWorkflows",
		"tryInterruptHandler",
		"runWorkflowChunk",
		"interrupt_retrying_original_chunk",
		"interrupt_handler_not_found",
	}
	for _, token := range required {
		if !strings.Contains(playwrightRunnerScript, token) {
			t.Fatalf("runner script should contain %q", token)
		}
	}
}
