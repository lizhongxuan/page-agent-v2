package memory

import (
	"context"
	"testing"

	"github.com/page-agent/workflow-backend/internal/registry"
)

func TestAttributionServiceLabelsHelpfulWhenEvidenceGuidesPathAndTarget(t *testing.T) {
	service := NewAttributionService(nil)
	run := attributionRun(registry.TaskRunSuccess)
	refs := []registry.MemoryEvidenceRef{
		{
			ID:          "exp_service_status",
			Source:      registry.MemoryEvidenceSourceExperience,
			Title:       "Open service detail",
			Rank:        1,
			Score:       0.91,
			PageStateID: "page_service_list",
			Payload: map[string]any{
				"optimizedPath": []any{"page_service_list", "page_service_detail"},
				"targetNames":   []any{"Service name"},
			},
		},
	}

	events := service.Evaluate(AttributionInput{TaskRun: run, EvidenceRefs: refs})

	if len(events) != 1 {
		t.Fatalf("expected one attribution event, got %#v", events)
	}
	if events[0].Label != registry.MemoryAttributionHelpful {
		t.Fatalf("expected helpful attribution, got %#v", events[0])
	}
	if !containsSignal(events[0].Signals, SignalPathAligned) || !containsSignal(events[0].Signals, SignalTargetAligned) {
		t.Fatalf("expected alignment signals, got %#v", events[0].Signals)
	}
}

func TestAttributionServiceMatchesTargetWhenEvidenceNameIsMoreSpecific(t *testing.T) {
	service := NewAttributionService(nil)
	run := attributionRun(registry.TaskRunSuccess)
	run.OriginalPath = []string{"page_service_list"}
	run.OptimizedPath = []string{"page_service_list"}
	run.ActionSteps = []registry.ActionStep{
		{
			ID:            "step_search_service",
			PageStateID:   "page_service_list",
			StepIndex:     1,
			ActionType:    registry.StepFill,
			TargetName:    "Service name",
			ValueTemplate: "{{service_name}}",
			ResultSummary: "Service list filtered.",
		},
	}
	refs := []registry.MemoryEvidenceRef{
		{
			ID:     "nav_specific_target",
			Source: registry.MemoryEvidenceSourceNavigation,
			Payload: map[string]any{
				"targetNames": []any{"Service name search input"},
			},
		},
	}

	events := service.Evaluate(AttributionInput{TaskRun: run, EvidenceRefs: refs})

	if len(events) != 1 {
		t.Fatalf("expected one attribution event, got %#v", events)
	}
	if events[0].Label != registry.MemoryAttributionHelpful {
		t.Fatalf("expected helpful attribution for semantically same target, got %#v", events[0])
	}
	if !containsSignal(events[0].Signals, SignalTargetAligned) {
		t.Fatalf("expected target-aligned signal, got %#v", events[0].Signals)
	}
}

func TestAttributionServiceDoesNotRewardUnusedEvidenceOnSuccessfulTask(t *testing.T) {
	service := NewAttributionService(nil)
	run := attributionRun(registry.TaskRunSuccess)
	refs := []registry.MemoryEvidenceRef{
		{
			ID:          "exp_wrong_branch",
			Source:      registry.MemoryEvidenceSourceExperience,
			Title:       "Open billing",
			Rank:        1,
			Score:       0.88,
			PageStateID: "page_billing",
			Payload: map[string]any{
				"optimizedPath": []any{"page_home", "page_billing"},
				"targetNames":   []any{"Billing"},
			},
		},
	}

	events := service.Evaluate(AttributionInput{TaskRun: run, EvidenceRefs: refs})

	if len(events) != 1 {
		t.Fatalf("expected one attribution event, got %#v", events)
	}
	if events[0].Label != registry.MemoryAttributionUnused {
		t.Fatalf("successful task must not reward unused evidence, got %#v", events[0])
	}
}

func TestAttributionServiceDoesNotPunishUnusedEvidenceOnFailedTask(t *testing.T) {
	service := NewAttributionService(nil)
	run := attributionRun(registry.TaskRunFailed)
	run.Summary = "The target service detail panel timed out."
	refs := []registry.MemoryEvidenceRef{
		{
			ID:          "exp_wrong_branch",
			Source:      registry.MemoryEvidenceSourceExperience,
			Title:       "Open billing",
			Rank:        1,
			Score:       0.88,
			PageStateID: "page_billing",
			Payload: map[string]any{
				"optimizedPath": []any{"page_home", "page_billing"},
				"targetNames":   []any{"Billing"},
			},
		},
	}

	events := service.Evaluate(AttributionInput{TaskRun: run, EvidenceRefs: refs})

	if len(events) != 1 {
		t.Fatalf("expected one attribution event, got %#v", events)
	}
	if events[0].Label == registry.MemoryAttributionMisleading {
		t.Fatalf("failed task must not punish unused evidence, got %#v", events[0])
	}
	if events[0].Label != registry.MemoryAttributionUnused {
		t.Fatalf("expected unused attribution, got %#v", events[0])
	}
}

func TestAttributionServiceLabelsMisleadingWhenAdoptedEvidenceCausesBranchNoise(t *testing.T) {
	service := NewAttributionService(nil)
	run := attributionRun(registry.TaskRunSuccess)
	run.OriginalPath = []string{"page_service_list", "page_billing", "page_service_list", "page_service_detail"}
	run.OptimizedPath = []string{"page_service_list", "page_service_detail"}
	run.ActionSteps = append([]registry.ActionStep{
		{
			ID:            "step_wrong_billing",
			PageStateID:   "page_service_list",
			StepIndex:     1,
			ActionType:    registry.StepClick,
			TargetName:    "Billing",
			ResultSummary: "Clicked billing.",
			IsBranchNoise: true,
		},
	}, run.ActionSteps...)
	refs := []registry.MemoryEvidenceRef{
		{
			ID:          "exp_wrong_branch",
			Source:      registry.MemoryEvidenceSourceExperience,
			Title:       "Open billing",
			Rank:        1,
			Score:       0.88,
			PageStateID: "page_service_list",
			Payload: map[string]any{
				"optimizedPath": []any{"page_service_list", "page_billing"},
				"targetNames":   []any{"Billing"},
			},
		},
	}

	events := service.Evaluate(AttributionInput{TaskRun: run, EvidenceRefs: refs})

	if len(events) != 1 {
		t.Fatalf("expected one attribution event, got %#v", events)
	}
	if events[0].Label != registry.MemoryAttributionMisleading {
		t.Fatalf("expected misleading attribution, got %#v", events[0])
	}
	if !containsSignal(events[0].Signals, SignalBranchNoise) {
		t.Fatalf("expected branch-noise signal, got %#v", events[0].Signals)
	}
}

func TestAttributionServiceLabelsStaleWhenSuggestedTargetIsMissing(t *testing.T) {
	service := NewAttributionService(nil)
	run := attributionRun(registry.TaskRunSuccess)
	refs := []registry.MemoryEvidenceRef{
		{
			ID:          "exp_old_selector",
			Source:      registry.MemoryEvidenceSourceExperience,
			Title:       "Old service search",
			Rank:        1,
			Score:       0.88,
			PageStateID: "page_service_list",
			Payload: map[string]any{
				"targetNames": []any{"Legacy Search"},
			},
		},
	}
	observation := NormalizedPageObservation{
		Controls: []registry.ControlSignature{
			{Role: "textbox", Name: "Service name"},
			{Role: "button", Name: "Search"},
		},
	}

	events := service.Evaluate(AttributionInput{
		TaskRun:         run,
		EvidenceRefs:    refs,
		PageObservation: &observation,
	})

	if len(events) != 1 {
		t.Fatalf("expected one attribution event, got %#v", events)
	}
	if events[0].Label != registry.MemoryAttributionStale {
		t.Fatalf("expected stale attribution, got %#v", events[0])
	}
	if !containsSignal(events[0].Signals, SignalPageRuleMismatch) {
		t.Fatalf("expected page-rule mismatch signal, got %#v", events[0].Signals)
	}
}

func TestAttributionServicePersistsManualStaleWhenManualConflictsWithDOM(t *testing.T) {
	ctx := context.Background()
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	run := attributionRun(registry.TaskRunSuccess)
	ref := registry.MemoryEvidenceRef{
		ID:          "manual_restore_legacy",
		Source:      registry.MemoryEvidenceSourceManual,
		PageStateID: "page_service_list",
		Payload: map[string]any{
			"targetNames": []any{"Legacy Restore"},
		},
	}
	observation := NormalizedPageObservation{
		PageStateID: "page_service_list",
		Controls: []registry.ControlSignature{
			{Role: "button", Name: "Data Restore"},
			{Role: "button", Name: "Start Restore"},
		},
	}

	result, err := NewAttributionService(repo).EvaluateAndPersist(ctx, AttributionInput{
		TaskRun:         run,
		EvidenceRefs:    []registry.MemoryEvidenceRef{ref},
		PageObservation: &observation,
	})
	if err != nil {
		t.Fatalf("EvaluateAndPersist failed: %v", err)
	}
	if len(result.Events) != 1 || result.Events[0].EvidenceSource != registry.MemoryEvidenceSourceManual || result.Events[0].Label != registry.MemoryAttributionStale {
		t.Fatalf("manual/DOM conflict should be recorded as stale manual evidence: %#v", result.Events)
	}
	stats, err := repo.GetMemoryEvidenceStats(ctx, "default", registry.MemoryEvidenceSourceManual, ref.ID)
	if err != nil {
		t.Fatalf("GetMemoryEvidenceStats failed: %v", err)
	}
	if stats.StaleCount != 1 || stats.UtilityScore >= 0 {
		t.Fatalf("expected stale manual evidence penalty, got %#v", stats)
	}
}

func TestAttributionServicePersistsEventsAndUpdatesStats(t *testing.T) {
	ctx := context.Background()
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	service := NewAttributionService(repo)
	run := attributionRun(registry.TaskRunSuccess)
	refs := []registry.MemoryEvidenceRef{
		{
			ID:          "exp_service_status",
			Source:      registry.MemoryEvidenceSourceExperience,
			Title:       "Open service detail",
			Rank:        1,
			Score:       0.91,
			PageStateID: "page_service_list",
			Payload: map[string]any{
				"optimizedPath": []any{"page_service_list", "page_service_detail"},
				"targetNames":   []any{"Service name"},
			},
		},
	}

	result, err := service.EvaluateAndPersist(ctx, AttributionInput{TaskRun: run, EvidenceRefs: refs})
	if err != nil {
		t.Fatalf("EvaluateAndPersist failed: %v", err)
	}
	if len(result.Events) != 1 || result.Events[0].Label != registry.MemoryAttributionHelpful {
		t.Fatalf("unexpected attribution result: %#v", result)
	}

	stats, err := repo.GetMemoryEvidenceStats(ctx, "default", registry.MemoryEvidenceSourceExperience, "exp_service_status")
	if err != nil {
		t.Fatalf("GetMemoryEvidenceStats failed: %v", err)
	}
	if stats.HelpfulCount != 1 || stats.UtilityScore <= 0 {
		t.Fatalf("expected helpful stats update, got %#v", stats)
	}
}

func TestAttributionServiceUpdatesSiteTaskGuideCounters(t *testing.T) {
	ctx := context.Background()
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	service := NewAttributionService(repo)
	seedAttributionGuide(t, ctx, repo, "guide_unused")
	seedAttributionGuide(t, ctx, repo, "guide_misleading")
	seedAttributionGuide(t, ctx, repo, "guide_mismatch")

	unusedRun := attributionRun(registry.TaskRunSuccess)
	unusedRun.ActionSteps = []registry.ActionStep{{ID: "step_search", PageStateID: "page_service_list", StepIndex: 1, ActionType: registry.StepFill, TargetName: "Service name"}}
	if _, err := service.EvaluateAndPersist(ctx, AttributionInput{
		TaskRun: unusedRun,
		EvidenceRefs: []registry.MemoryEvidenceRef{{
			ID:          "guide_unused",
			Source:      registry.MemoryEvidenceSourceGuide,
			PageStateID: "page_billing",
			Payload:     map[string]any{"targetNames": []any{"Billing"}},
		}},
	}); err != nil {
		t.Fatalf("EvaluateAndPersist unused failed: %v", err)
	}
	unusedGuide, err := repo.GetSiteTaskGuide(ctx, "guide_unused")
	if err != nil {
		t.Fatalf("GetSiteTaskGuide unused failed: %v", err)
	}
	if unusedGuide.UnusedCount != 1 || unusedGuide.SuccessCount != 1 || unusedGuide.MisleadingCount != 0 {
		t.Fatalf("unused guide should not be rewarded or marked misleading: %#v", unusedGuide)
	}

	misleadingRun := attributionRun(registry.TaskRunSuccess)
	misleadingRun.ActionSteps = []registry.ActionStep{{
		ID:            "step_broken",
		PageStateID:   "page_service_list",
		StepIndex:     1,
		ActionType:    registry.StepClick,
		TargetName:    "Broken button",
		ResultSummary: "failed to click: not visible",
	}}
	if _, err := service.EvaluateAndPersist(ctx, AttributionInput{
		TaskRun: misleadingRun,
		EvidenceRefs: []registry.MemoryEvidenceRef{{
			ID:          "guide_misleading",
			Source:      registry.MemoryEvidenceSourceGuide,
			PageStateID: "page_service_list",
			Payload:     map[string]any{"targetNames": []any{"Broken button"}},
		}},
	}); err != nil {
		t.Fatalf("EvaluateAndPersist misleading failed: %v", err)
	}
	misleadingGuide, err := repo.GetSiteTaskGuide(ctx, "guide_misleading")
	if err != nil {
		t.Fatalf("GetSiteTaskGuide misleading failed: %v", err)
	}
	if misleadingGuide.MisleadingCount != 1 {
		t.Fatalf("adopted broken target should mark guide misleading: %#v", misleadingGuide)
	}

	mismatchRun := attributionRun(registry.TaskRunSuccess)
	mismatchRun.ActionSteps = []registry.ActionStep{{ID: "step_search", PageStateID: "page_service_list", StepIndex: 1, ActionType: registry.StepFill, TargetName: "Service name"}}
	if _, err := service.EvaluateAndPersist(ctx, AttributionInput{
		TaskRun: mismatchRun,
		EvidenceRefs: []registry.MemoryEvidenceRef{{
			ID:          "guide_mismatch",
			Source:      registry.MemoryEvidenceSourceGuide,
			PageStateID: "page_service_list",
			Payload:     map[string]any{"targetNames": []any{"Legacy Search"}},
		}},
		PageObservation: &NormalizedPageObservation{
			PageStateID: "page_service_list",
			Controls: []registry.ControlSignature{
				{Role: "textbox", Name: "Service name"},
				{Role: "button", Name: "Search"},
			},
		},
	}); err != nil {
		t.Fatalf("EvaluateAndPersist mismatch failed: %v", err)
	}
	mismatchGuide, err := repo.GetSiteTaskGuide(ctx, "guide_mismatch")
	if err != nil {
		t.Fatalf("GetSiteTaskGuide mismatch failed: %v", err)
	}
	if mismatchGuide.AbandonedCount != 1 || mismatchGuide.StaleCount != 0 {
		t.Fatalf("page mismatch should mark guide abandoned_mismatch: %#v", mismatchGuide)
	}
}

func TestAttributionServiceUpdatesOnlyMatchedSiteTaskGuideState(t *testing.T) {
	ctx := context.Background()
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	service := NewAttributionService(repo)
	if err := repo.SaveSiteTaskGuide(ctx, registry.SiteTaskGuide{
		ID:            "guide_restore",
		ProjectID:     "default",
		Site:          "ops.example.com",
		TaskIntentKey: "restore_backup",
		Summary:       "Restore data from a backup.",
		Steps:         []registry.SiteTaskGuideStep{{Text: "Click Data Restore.", Target: "Data Restore"}},
		UIStateEntries: []registry.SiteTaskGuideUIStateEntry{
			{
				ID:         "state_full_backup_tab",
				Name:       "Full Backup tab",
				StateType:  registry.SiteTaskGuideUIStatePage,
				Confidence: 0.8,
				Evidence: registry.SiteTaskGuideUIStateEvidence{
					ControlsAll: []registry.ControlSignature{{Role: "button", Name: "Data Restore"}},
				},
				MinimumScore: 1,
			},
			{
				ID:         "state_restore_modal",
				Name:       "Restore modal",
				StateType:  registry.SiteTaskGuideUIStateModal,
				Confidence: 0.8,
				Evidence: registry.SiteTaskGuideUIStateEvidence{
					ControlsAll: []registry.ControlSignature{{Role: "button", Name: "Start Restore"}},
				},
				MinimumScore: 1,
			},
		},
		Status:       registry.StatusActive,
		SuccessCount: 1,
	}); err != nil {
		t.Fatalf("SaveSiteTaskGuide failed: %v", err)
	}
	run := attributionRun(registry.TaskRunSuccess)
	run.ActionSteps = []registry.ActionStep{{
		ID:            "step_restore_broken",
		PageStateID:   "page_backups",
		StepIndex:     1,
		ActionType:    registry.StepClick,
		TargetName:    "Data Restore",
		ResultSummary: "failed to click: not visible",
	}}

	if _, err := service.EvaluateAndPersist(ctx, AttributionInput{
		TaskRun: run,
		EvidenceRefs: []registry.MemoryEvidenceRef{{
			ID:          "guide_restore",
			Source:      registry.MemoryEvidenceSourceGuide,
			PageStateID: "page_backups",
			Payload: map[string]any{
				"matchedStateId": "state_full_backup_tab",
				"targetNames":    []any{"Data Restore"},
			},
		}},
	}); err != nil {
		t.Fatalf("EvaluateAndPersist failed: %v", err)
	}
	guide, err := repo.GetSiteTaskGuide(ctx, "guide_restore")
	if err != nil {
		t.Fatalf("GetSiteTaskGuide failed: %v", err)
	}
	states := map[string]registry.SiteTaskGuideUIStateEntry{}
	for _, state := range guide.UIStateEntries {
		states[state.ID] = state
	}
	if states["state_full_backup_tab"].Confidence >= 0.8 {
		t.Fatalf("matched state should be penalized, got %#v", states["state_full_backup_tab"])
	}
	if states["state_restore_modal"].Confidence != 0.8 {
		t.Fatalf("unmatched state should not change, got %#v", states["state_restore_modal"])
	}
}

func TestAttributionServiceUpdatesStatsWithBoundedUtility(t *testing.T) {
	ctx := context.Background()
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	service := NewAttributionService(repo)
	helpfulRun := attributionRun(registry.TaskRunSuccess)
	helpfulRef := registry.MemoryEvidenceRef{
		ID:     "exp_helpful_repeat",
		Source: registry.MemoryEvidenceSourceExperience,
		Payload: map[string]any{
			"optimizedPath": []string{"page_service_list", "page_service_detail"},
			"targetNames":   []string{"Service name"},
		},
	}
	for index := 0; index < 7; index++ {
		if _, err := service.EvaluateAndPersist(ctx, AttributionInput{TaskRun: helpfulRun, EvidenceRefs: []registry.MemoryEvidenceRef{helpfulRef}}); err != nil {
			t.Fatalf("EvaluateAndPersist helpful failed: %v", err)
		}
	}
	helpfulStats, err := repo.GetMemoryEvidenceStats(ctx, "default", registry.MemoryEvidenceSourceExperience, helpfulRef.ID)
	if err != nil {
		t.Fatalf("GetMemoryEvidenceStats helpful failed: %v", err)
	}
	if helpfulStats.HelpfulCount != 7 || helpfulStats.UtilityScore != 5 {
		t.Fatalf("expected helpful stats to rise and clamp at 5, got %#v", helpfulStats)
	}

	misleadingRun := attributionRun(registry.TaskRunSuccess)
	misleadingRun.OriginalPath = []string{"page_service_list", "page_billing", "page_service_list", "page_service_detail"}
	misleadingRun.OptimizedPath = []string{"page_service_list", "page_service_detail"}
	misleadingRun.ActionSteps = []registry.ActionStep{
		{ID: "step_wrong_billing", PageStateID: "page_service_list", StepIndex: 1, ActionType: registry.StepClick, TargetName: "Billing", ResultSummary: "Clicked billing.", IsBranchNoise: true},
	}
	misleadingRef := registry.MemoryEvidenceRef{
		ID:     "exp_misleading_repeat",
		Source: registry.MemoryEvidenceSourceExperience,
		Payload: map[string]any{
			"optimizedPath": []string{"page_service_list", "page_billing"},
			"targetNames":   []string{"Billing"},
		},
	}
	for index := 0; index < 5; index++ {
		if _, err := service.EvaluateAndPersist(ctx, AttributionInput{TaskRun: misleadingRun, EvidenceRefs: []registry.MemoryEvidenceRef{misleadingRef}}); err != nil {
			t.Fatalf("EvaluateAndPersist misleading failed: %v", err)
		}
	}
	misleadingStats, err := repo.GetMemoryEvidenceStats(ctx, "default", registry.MemoryEvidenceSourceExperience, misleadingRef.ID)
	if err != nil {
		t.Fatalf("GetMemoryEvidenceStats misleading failed: %v", err)
	}
	if misleadingStats.MisleadingCount != 5 || misleadingStats.UtilityScore != -5 {
		t.Fatalf("expected misleading stats to drop and clamp at -5, got %#v", misleadingStats)
	}

	unusedRef := registry.MemoryEvidenceRef{
		ID:     "exp_unused_soft_penalty",
		Source: registry.MemoryEvidenceSourceExperience,
		Payload: map[string]any{
			"optimizedPath": []string{"page_home", "page_billing"},
			"targetNames":   []string{"Billing"},
		},
	}
	if _, err := service.EvaluateAndPersist(ctx, AttributionInput{TaskRun: helpfulRun, EvidenceRefs: []registry.MemoryEvidenceRef{unusedRef}}); err != nil {
		t.Fatalf("EvaluateAndPersist unused failed: %v", err)
	}
	unusedStats, err := repo.GetMemoryEvidenceStats(ctx, "default", registry.MemoryEvidenceSourceExperience, unusedRef.ID)
	if err != nil {
		t.Fatalf("GetMemoryEvidenceStats unused failed: %v", err)
	}
	if unusedStats.UnusedCount != 1 || unusedStats.UtilityScore != -0.05 {
		t.Fatalf("expected unused to have only a soft penalty, got %#v", unusedStats)
	}
}

func TestAttributionServiceLabelsMixedEvidenceSet(t *testing.T) {
	service := NewAttributionService(nil)
	run := attributionRun(registry.TaskRunSuccess)
	run.OriginalPath = []string{"page_service_list", "page_billing", "page_service_list", "page_service_detail"}
	run.OptimizedPath = []string{"page_service_list", "page_service_detail"}
	run.ActionSteps = []registry.ActionStep{
		{ID: "step_wrong_billing", PageStateID: "page_service_list", StepIndex: 1, ActionType: registry.StepClick, TargetName: "Billing", ResultSummary: "Clicked billing.", IsBranchNoise: true},
		{ID: "step_broken", PageStateID: "page_service_list", StepIndex: 2, ActionType: registry.StepClick, TargetName: "Broken button", ResultSummary: "failed to click: not visible"},
		{ID: "step_search_service", PageStateID: "page_service_list", StepIndex: 3, ActionType: registry.StepFill, TargetName: "Service name", ResultSummary: "Service list filtered."},
	}
	refs := []registry.MemoryEvidenceRef{
		{ID: "helpful_path", Source: registry.MemoryEvidenceSourceExperience, Payload: map[string]any{"optimizedPath": []string{"page_service_list", "page_service_detail"}}},
		{ID: "helpful_target", Source: registry.MemoryEvidenceSourceNavigation, Payload: map[string]any{"optimizedPath": []string{"page_service_list", "page_service_detail"}, "targetNames": []string{"Service name"}}},
		{ID: "misleading_branch", Source: registry.MemoryEvidenceSourceExperience, Payload: map[string]any{"optimizedPath": []string{"page_service_list", "page_billing"}, "targetNames": []string{"Billing"}}},
		{ID: "misleading_selector", Source: registry.MemoryEvidenceSourceExperience, Payload: map[string]any{"targetNames": []string{"Broken button"}}},
		{ID: "stale_missing_search", Source: registry.MemoryEvidenceSourceExperience, Payload: map[string]any{"targetNames": []string{"Legacy search"}}},
		{ID: "stale_missing_filter", Source: registry.MemoryEvidenceSourceExperience, Payload: map[string]any{"targetNames": []string{"Old filter"}}},
		{ID: "unused_path", Source: registry.MemoryEvidenceSourceExperience, Payload: map[string]any{"optimizedPath": []string{"page_home", "page_billing"}}},
		{ID: "unused_target", Source: registry.MemoryEvidenceSourceExperience, Payload: map[string]any{"targetNames": []string{"Search"}}},
		{ID: "neutral_manual", Source: registry.MemoryEvidenceSourceManual},
	}
	observation := NormalizedPageObservation{
		Controls: []registry.ControlSignature{
			{Role: "textbox", Name: "Service name"},
			{Role: "button", Name: "Search"},
		},
	}

	events := service.Evaluate(AttributionInput{TaskRun: run, EvidenceRefs: refs, PageObservation: &observation})

	counts := map[registry.MemoryAttributionLabel]int{}
	for _, event := range events {
		counts[event.Label]++
	}
	expected := map[registry.MemoryAttributionLabel]int{
		registry.MemoryAttributionHelpful:    2,
		registry.MemoryAttributionMisleading: 2,
		registry.MemoryAttributionStale:      2,
		registry.MemoryAttributionUnused:     2,
		registry.MemoryAttributionNeutral:    1,
	}
	for label, count := range expected {
		if counts[label] != count {
			t.Fatalf("expected %s count %d, got counts=%#v events=%#v", label, count, counts, events)
		}
	}
}

func TestAttributionServicePersistsAdoptionSignalSeparateFromTaskOutcome(t *testing.T) {
	service := NewAttributionService(nil)
	run := attributionRun(registry.TaskRunSuccess)
	run.ActionSteps = []registry.ActionStep{
		{
			ID:            "step_open_billing",
			PageStateID:   "page_service_list",
			StepIndex:     1,
			ActionType:    registry.StepClick,
			TargetName:    "Billing",
			ResultSummary: "Clicked billing then returned.",
			IsBranchNoise: true,
		},
		{
			ID:            "step_search_service",
			PageStateID:   "page_service_list",
			StepIndex:     2,
			ActionType:    registry.StepFill,
			TargetName:    "Service name",
			ValueTemplate: "{{service_name}}",
			ResultSummary: "Service list filtered.",
		},
	}
	refs := []registry.MemoryEvidenceRef{
		{
			ID:     "exp_wrong_billing",
			Source: registry.MemoryEvidenceSourceExperience,
			Payload: map[string]any{
				"optimizedPath": []string{"page_service_list", "page_billing"},
				"targetNames":   []string{"Billing"},
				"stepTargets": []registry.MemoryStepTarget{
					{ActionType: string(registry.StepClick), TargetName: "Billing", PageStateID: "page_service_list"},
				},
			},
		},
	}

	events := service.Evaluate(AttributionInput{TaskRun: run, EvidenceRefs: refs})

	if len(events) != 1 {
		t.Fatalf("expected one attribution event, got %#v", events)
	}
	if events[0].Label != registry.MemoryAttributionMisleading {
		t.Fatalf("adopted branch-noise evidence should be misleading, got %#v", events[0])
	}
	if !events[0].Adoption.AdoptedTarget || !events[0].Adoption.CausedBacktrack {
		t.Fatalf("expected adopted target and backtrack signal, got %#v", events[0].Adoption)
	}
	if events[0].Adoption.FirstAdoptedStepIndex != 1 {
		t.Fatalf("expected first adopted step index 1, got %#v", events[0].Adoption)
	}
}

func TestAttributionServiceAdoptedSurfaceRequiresCurrentSurfaceMatch(t *testing.T) {
	service := NewAttributionService(nil)
	run := attributionRun(registry.TaskRunSuccess)
	ref := registry.MemoryEvidenceRef{
		ID:     "exp_modal_service_edit",
		Source: registry.MemoryEvidenceSourceExperience,
		Payload: map[string]any{
			"optimizedPath": []string{"page_service_list", "page_service_detail"},
			"stepTargets": []registry.MemoryStepTarget{
				{
					ActionType:  string(registry.StepFill),
					TargetName:  "Service name",
					PageStateID: "page_service_list",
					SurfaceID:   "surface_edit_modal",
				},
			},
		},
	}

	withoutSurface := service.Evaluate(AttributionInput{
		TaskRun:      run,
		ContextEvent: registry.MemoryContextEvent{CurrentSurface: ""},
		EvidenceRefs: []registry.MemoryEvidenceRef{ref},
	})
	if len(withoutSurface) != 1 {
		t.Fatalf("expected one attribution event, got %#v", withoutSurface)
	}
	if withoutSurface[0].Adoption.AdoptedSurface {
		t.Fatalf("surface evidence must not be marked adopted without current surface: %#v", withoutSurface[0].Adoption)
	}

	withSurface := service.Evaluate(AttributionInput{
		TaskRun:      run,
		ContextEvent: registry.MemoryContextEvent{CurrentSurface: "surface_edit_modal"},
		EvidenceRefs: []registry.MemoryEvidenceRef{ref},
	})
	if len(withSurface) != 1 {
		t.Fatalf("expected one attribution event, got %#v", withSurface)
	}
	if !withSurface[0].Adoption.AdoptedSurface {
		t.Fatalf("matching current surface should mark surface evidence adopted: %#v", withSurface[0].Adoption)
	}
}

func attributionRun(status registry.TaskRunStatus) registry.TaskRun {
	return registry.TaskRun{
		ID:              "task_run_service_status",
		ProjectID:       "default",
		Site:            "ops.example.com",
		TaskTemplate:    "Check {{service_name}} status",
		Summary:         "Checked service status.",
		MemoryContextID: "ctx_service_status",
		OriginalPath:    []string{"page_service_list", "page_service_detail"},
		OptimizedPath:   []string{"page_service_list", "page_service_detail"},
		Status:          status,
		ActionSteps: []registry.ActionStep{
			{
				ID:            "step_search_service",
				PageStateID:   "page_service_list",
				StepIndex:     1,
				ActionType:    registry.StepFill,
				TargetName:    "Service name",
				ValueTemplate: "{{service_name}}",
				ResultSummary: "Service list filtered.",
			},
			{
				ID:            "step_open_detail",
				PageStateID:   "page_service_list",
				StepIndex:     2,
				ActionType:    registry.StepClick,
				TargetName:    "Service name",
				ResultSummary: "Opened service detail.",
			},
		},
	}
}

func seedAttributionGuide(t *testing.T, ctx context.Context, repo registry.SiteTaskGuideRepository, id string) {
	t.Helper()
	if err := repo.SaveSiteTaskGuide(ctx, registry.SiteTaskGuide{
		ID:            id,
		ProjectID:     "default",
		Site:          "ops.example.com",
		TaskIntentKey: id,
		Summary:       "Check service status.",
		Steps:         []registry.SiteTaskGuideStep{{Text: "Use the recorded guide.", Target: "Service name"}},
		UIStateEntries: []registry.SiteTaskGuideUIStateEntry{{
			ID:        "state_service_list",
			Name:      "Service list",
			StateType: registry.SiteTaskGuideUIStatePage,
			Evidence: registry.SiteTaskGuideUIStateEvidence{
				ControlsAll: []registry.ControlSignature{{Role: "textbox", Name: "Service name"}},
			},
			MinimumScore: 1,
		}},
		Status:       registry.StatusActive,
		SuccessCount: 1,
	}); err != nil {
		t.Fatalf("SaveSiteTaskGuide %s failed: %v", id, err)
	}
}

func containsSignal(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}
