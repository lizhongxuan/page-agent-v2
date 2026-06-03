package memory

import (
	"testing"

	"github.com/page-agent/workflow-backend/internal/registry"
)

func TestMatchUIStateEntryRequiresStableControl(t *testing.T) {
	entry := registry.SiteTaskGuideUIStateEntry{
		ID:         "state_full_backup_tab",
		Name:       "Full Backup tab",
		StateType:  registry.SiteTaskGuideUIStateTab,
		StepOffset: 3,
		Evidence: registry.SiteTaskGuideUIStateEvidence{
			ActiveTabAny: []string{"Full Backup"},
			ControlsAll:  []registry.ControlSignature{{Role: "button", Name: "Data Restore"}},
		},
		MinimumScore: 4,
	}

	pass := matchUIStateEntry(entry, &registry.PageObservationSignal{
		Title:             "Data Backup",
		VisibleTextSample: "Full Backup Data Restore",
		ActiveTabs:        []string{"Full Backup"},
		ControlSignatures: []registry.ControlSignature{{Role: "button", Name: "Data Restore"}},
	}, 6)
	if !pass.Passed {
		t.Fatalf("expected full backup state to match, got %#v", pass)
	}

	fail := matchUIStateEntry(entry, &registry.PageObservationSignal{
		Title:             "Audit Settings",
		VisibleTextSample: "Retention days Save",
		ControlSignatures: []registry.ControlSignature{{Role: "button", Name: "Save"}},
	}, 6)
	if fail.Passed || len(fail.Missing) == 0 {
		t.Fatalf("expected unrelated page to fail preflight, got %#v", fail)
	}
}

func TestMatchUIStateEntryAllowsPartialGeneratedControlSet(t *testing.T) {
	entry := registry.SiteTaskGuideUIStateEntry{
		ID:         "state_review_tab",
		Name:       "Review tab",
		StateType:  registry.SiteTaskGuideUIStateTab,
		StepOffset: 2,
		Evidence: registry.SiteTaskGuideUIStateEvidence{
			TitleAny:     []string{"Review Center"},
			TextAny:      []string{"Pending Reviews"},
			ActiveTabAny: []string{"Pending"},
			ControlsAll: []registry.ControlSignature{
				{Role: "button", Name: "Submit Review"},
				{Role: "button", Name: "Refresh"},
			},
		},
		MinimumScore: 5,
	}

	result := matchUIStateEntry(entry, &registry.PageObservationSignal{
		Title:             "Review Center",
		VisibleTextSample: "Pending Reviews",
		ActiveTabs:        []string{"Pending"},
		ControlSignatures: []registry.ControlSignature{{Role: "button", Name: "Submit Review"}},
	}, 6)

	if !result.Passed || !containsString(result.Matched, "controls_any") {
		t.Fatalf("expected partial generated control set to pass, got %#v", result)
	}
}

func TestMatchUIStateEntryPrefersActiveModalSurface(t *testing.T) {
	entry := registry.SiteTaskGuideUIStateEntry{
		ID:         "state_restore_modal",
		Name:       "Restore modal",
		StateType:  registry.SiteTaskGuideUIStateModal,
		StepOffset: 5,
		Evidence: registry.SiteTaskGuideUIStateEvidence{
			ActiveSurfacesAny: []registry.ActiveSurfaceSignal{{
				SurfaceType: registry.SurfaceModal,
				Title:       "Data Restore",
				Controls:    []registry.ControlSignature{{Role: "button", Name: "Start Restore"}},
			}},
		},
		MinimumScore: 3,
	}

	result := matchUIStateEntry(entry, &registry.PageObservationSignal{
		Title: "Data Backup",
		ActiveSurfaces: []registry.ActiveSurfaceSignal{{
			SurfaceType: registry.SurfaceModal,
			Title:       "Data Restore",
			Controls:    []registry.ControlSignature{{Role: "button", Name: "Start Restore"}},
		}},
	}, 7)

	if !result.Passed || !containsString(result.Matched, "active_surface") {
		t.Fatalf("expected modal surface to pass, got %#v", result)
	}
}

func TestGeneratedUIStateRouteUsesURLFamilyForDynamicPaths(t *testing.T) {
	entry := uiStateEntryFromObservation(registry.PageObservationSignal{
		Site:              "k8s.example.com",
		URL:               "https://k8s.example.com/workloads/deployment-a/operations",
		URLPattern:        "https://k8s.example.com/workloads/deployment-a/operations",
		Title:             "Deployment Operations",
		VisibleTextSample: "Deployment Operations Restart Deployment Confirm Restart",
		ActiveTabs:        []string{"Operations"},
		ControlSignatures: []registry.ControlSignature{{Role: "button", Name: "Restart Deployment"}},
	}, 3, "Restart Deployment", registry.StepClick)

	result := matchUIStateEntry(entry, &registry.PageObservationSignal{
		Site:              "k8s.example.com",
		URL:               "https://k8s.example.com/workloads/deployment-b/operations",
		URLPattern:        "https://k8s.example.com/workloads/deployment-b/operations",
		URLFamily:         "/workloads/operations",
		Title:             "Deployment Operations",
		VisibleTextSample: "Deployment Operations Restart Deployment Confirm Restart",
		ActiveTabs:        []string{"Operations"},
		ControlSignatures: []registry.ControlSignature{{Role: "button", Name: "Restart Deployment"}},
	}, 5)
	if !result.Passed {
		t.Fatalf("expected same URL family to match, got %#v entry=%#v", result, entry)
	}
}
