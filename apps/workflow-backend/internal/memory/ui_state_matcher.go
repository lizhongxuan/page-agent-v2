package memory

import (
	"strings"

	"github.com/page-agent/workflow-backend/internal/registry"
)

type uiStateMatchResult struct {
	Entry        registry.SiteTaskGuideUIStateEntry
	Passed       bool
	Score        float64
	MinimumScore float64
	Matched      []string
	Missing      []string
	Reason       string
}

func bestUIStateMatch(guide registry.SiteTaskGuide, signal *registry.PageObservationSignal) uiStateMatchResult {
	if len(guide.UIStateEntries) == 0 {
		return uiStateMatchResult{Passed: false, Reason: "missing_ui_state_entries"}
	}
	if signal == nil {
		return uiStateMatchResult{Passed: false, Reason: "missing_page_observation_signal"}
	}
	best := uiStateMatchResult{Reason: "ui_state_preflight_failed"}
	for _, entry := range guide.UIStateEntries {
		result := matchUIStateEntry(entry, signal, len(guide.Steps))
		if result.Passed {
			if !best.Passed || result.Score > best.Score || (result.Score == best.Score && result.Entry.StepOffset > best.Entry.StepOffset) {
				best = result
			}
			continue
		}
		if !best.Passed && result.Score > best.Score {
			best = result
		}
	}
	return best
}

func matchUIStateEntry(entry registry.SiteTaskGuideUIStateEntry, signal *registry.PageObservationSignal, stepCount int) uiStateMatchResult {
	result := uiStateMatchResult{
		Entry:        entry,
		MinimumScore: entry.MinimumScore,
		Reason:       "ui_state_preflight_failed",
	}
	if signal == nil {
		result.Missing = append(result.Missing, "page_observation_signal")
		return result
	}
	if result.MinimumScore <= 0 {
		result.MinimumScore = 2
	}
	if entry.StepOffset < 0 || (stepCount > 0 && entry.StepOffset >= stepCount) {
		result.Missing = append(result.Missing, "valid_step_offset")
		return result
	}
	if !routeScopeMatches(entry.RouteScope, signal) {
		result.Missing = append(result.Missing, "route_scope")
		return result
	}
	evidence := entry.Evidence
	pageText := strings.ToLower(strings.Join([]string{
		signal.Title,
		signal.VisibleTextSample,
		strings.Join(signal.Breadcrumbs, " "),
		strings.Join(signal.ActiveTabs, " "),
		activeSurfaceText(signal.ActiveSurfaces),
	}, "\n"))
	if anyTextPresent(pageText, evidence.NegativeTextAny) {
		result.Missing = append(result.Missing, "negative_text_absent")
		return result
	}
	if anyControlPresentExtended(allSignalControls(signal), evidence.NegativeControlsAny) {
		result.Missing = append(result.Missing, "negative_control_absent")
		return result
	}
	addTextAnyScore(&result, "title", signal.Title, evidence.TitleAny, 1)
	addTextAnyScore(&result, "breadcrumb", strings.Join(signal.Breadcrumbs, " "), evidence.BreadcrumbAny, 1)
	addTextAnyScore(&result, "active_tab", strings.Join(signal.ActiveTabs, " "), evidence.ActiveTabAny, 2)
	if !addTextAllScore(&result, "text_all", pageText, evidence.TextAll, 1) {
		return result
	}
	addTextAnyScore(&result, "text_any", pageText, evidence.TextAny, 1)
	if len(evidence.ControlsAll) > 0 {
		observedControls := allSignalControls(signal)
		if !allControlsPresentExtended(observedControls, evidence.ControlsAll) {
			if anyControlPresentExtended(observedControls, evidence.ControlsAll) {
				result.Score += 2
				result.Matched = append(result.Matched, "controls_any")
			} else {
				result.Missing = append(result.Missing, "controls_all")
				return result
			}
		} else {
			result.Score += float64(len(evidence.ControlsAll)) * 2
			result.Matched = append(result.Matched, "controls_all")
		}
	}
	if len(evidence.ControlsAny) > 0 && anyControlPresentExtended(allSignalControls(signal), evidence.ControlsAny) {
		result.Score += 1
		result.Matched = append(result.Matched, "controls_any")
	}
	if len(evidence.TableHeadersAny) > 0 {
		if tableHeadersMatch(signal.Tables, evidence.TableHeadersAny) {
			result.Score += 1
			result.Matched = append(result.Matched, "table_headers")
		} else {
			result.Missing = append(result.Missing, "table_headers")
		}
	}
	if activeSurfaceState(entry.StateType) || len(evidence.ActiveSurfacesAny) > 0 {
		if activeSurfaceMatches(signal.ActiveSurfaces, evidence.ActiveSurfacesAny, entry.StateType) {
			result.Score += 3
			result.Matched = append(result.Matched, "active_surface")
		} else {
			result.Missing = append(result.Missing, "active_surface")
			return result
		}
	}
	if result.Score >= result.MinimumScore {
		result.Passed = true
		result.Reason = "ui_state_preflight_passed"
		return result
	}
	if len(result.Missing) == 0 {
		result.Missing = append(result.Missing, "minimum_score")
	}
	return result
}

func routeScopeMatches(scope registry.SiteTaskGuideRouteScope, signal *registry.PageObservationSignal) bool {
	if scope.URLPattern != "" {
		pattern := strings.ToLower(strings.TrimRight(scope.URLPattern, "*"))
		url := strings.ToLower(strings.Join([]string{signal.URL, signal.URLPattern, signal.URLFamily}, "\n"))
		if !strings.Contains(url, pattern) {
			return false
		}
	}
	for _, include := range scope.URLIncludes {
		include = strings.ToLower(strings.TrimSpace(include))
		if include == "" {
			continue
		}
		url := strings.ToLower(strings.Join([]string{signal.URL, signal.URLPattern, signal.URLFamily}, "\n"))
		if !strings.Contains(url, include) {
			return false
		}
	}
	return true
}

func addTextAnyScore(result *uiStateMatchResult, label, haystack string, needles []string, score float64) {
	if len(needles) == 0 {
		return
	}
	if anyTextPresent(strings.ToLower(haystack), needles) {
		result.Score += score
		result.Matched = append(result.Matched, label)
	}
}

func addTextAllScore(result *uiStateMatchResult, label, haystack string, needles []string, score float64) bool {
	for _, needle := range needles {
		needle = strings.ToLower(strings.TrimSpace(needle))
		if needle != "" && !strings.Contains(haystack, needle) {
			result.Missing = append(result.Missing, label+":"+needle)
			return false
		}
	}
	if len(needles) > 0 {
		result.Score += float64(len(needles)) * score
		result.Matched = append(result.Matched, label)
	}
	return true
}

func anyTextPresent(haystack string, needles []string) bool {
	haystack = strings.ToLower(haystack)
	for _, needle := range needles {
		needle = strings.ToLower(strings.TrimSpace(needle))
		if needle != "" && strings.Contains(haystack, needle) {
			return true
		}
	}
	return false
}

func allSignalControls(signal *registry.PageObservationSignal) []registry.ControlSignature {
	if signal == nil {
		return nil
	}
	result := append([]registry.ControlSignature{}, signal.ControlSignatures...)
	for _, surface := range signal.ActiveSurfaces {
		result = append(result, surface.Controls...)
	}
	return result
}

func allControlsPresentExtended(observed []registry.ControlSignature, required []registry.ControlSignature) bool {
	for _, control := range required {
		if !controlPresentExtended(observed, control) {
			return false
		}
	}
	return true
}

func anyControlPresentExtended(observed []registry.ControlSignature, required []registry.ControlSignature) bool {
	for _, control := range required {
		if controlPresentExtended(observed, control) {
			return true
		}
	}
	return false
}

func controlPresentExtended(observed []registry.ControlSignature, required registry.ControlSignature) bool {
	for _, control := range observed {
		if required.Role != "" && !strings.EqualFold(control.Role, required.Role) {
			continue
		}
		if required.Name != "" && !strings.EqualFold(control.Name, required.Name) {
			continue
		}
		if required.Selected != nil && (control.Selected == nil || *control.Selected != *required.Selected) {
			continue
		}
		if required.Enabled != nil && (control.Enabled == nil || *control.Enabled != *required.Enabled) {
			continue
		}
		return true
	}
	return false
}

func tableHeadersMatch(tables []registry.ObservationTableSignal, required [][]string) bool {
	for _, expected := range required {
		for _, table := range tables {
			if stringSetContainsAll(table.Headers, expected) {
				return true
			}
		}
	}
	return false
}

func stringSetContainsAll(values []string, required []string) bool {
	text := strings.ToLower(strings.Join(values, "\n"))
	for _, value := range required {
		value = strings.ToLower(strings.TrimSpace(value))
		if value != "" && !strings.Contains(text, value) {
			return false
		}
	}
	return true
}

func activeSurfaceState(stateType registry.SiteTaskGuideUIStateType) bool {
	switch stateType {
	case registry.SiteTaskGuideUIStateModal, registry.SiteTaskGuideUIStateDrawer, registry.SiteTaskGuideUIStatePopover, registry.SiteTaskGuideUIStateWizard:
		return true
	default:
		return false
	}
}

func activeSurfaceMatches(observed []registry.ActiveSurfaceSignal, expected []registry.ActiveSurfaceSignal, stateType registry.SiteTaskGuideUIStateType) bool {
	for _, surface := range observed {
		if len(expected) == 0 {
			return surfaceTypeMatches(surface.SurfaceType, stateType)
		}
		for _, candidate := range expected {
			if candidate.SurfaceType != "" && surface.SurfaceType != candidate.SurfaceType {
				continue
			}
			if candidate.Title != "" && !strings.Contains(strings.ToLower(surface.Title), strings.ToLower(candidate.Title)) {
				continue
			}
			if len(candidate.Controls) > 0 && !allControlsPresentExtended(surface.Controls, candidate.Controls) {
				continue
			}
			return true
		}
	}
	return false
}

func surfaceTypeMatches(surfaceType registry.SurfaceType, stateType registry.SiteTaskGuideUIStateType) bool {
	switch stateType {
	case registry.SiteTaskGuideUIStateModal:
		return surfaceType == registry.SurfaceModal
	case registry.SiteTaskGuideUIStateDrawer:
		return surfaceType == registry.SurfaceDrawer
	case registry.SiteTaskGuideUIStatePopover:
		return surfaceType == registry.SurfacePopover
	case registry.SiteTaskGuideUIStateWizard:
		return surfaceType == registry.SurfaceWizard
	default:
		return true
	}
}

func activeSurfaceText(surfaces []registry.ActiveSurfaceSignal) string {
	parts := []string{}
	for _, surface := range surfaces {
		parts = append(parts, string(surface.SurfaceType), surface.Title, strings.Join(surface.Text, " "), controlTextForGuide(surface.Controls))
	}
	return strings.Join(parts, "\n")
}
