package memory

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"net/url"
	"regexp"
	"sort"
	"strings"

	"github.com/page-agent/workflow-backend/internal/registry"
)

type PageObservationRequest struct {
	ProjectID           string                   `json:"projectId"`
	Task                string                   `json:"task,omitempty"`
	URL                 string                   `json:"url"`
	Title               string                   `json:"title,omitempty"`
	VisibleText         []string                 `json:"visibleText,omitempty"`
	Controls            []PageObservationControl `json:"controls,omitempty"`
	Links               []PageObservationLink    `json:"links,omitempty"`
	Source              string                   `json:"source,omitempty"`
	PreviousPageStateID string                   `json:"previousPageStateId,omitempty"`
	TransitionAction    string                   `json:"transitionAction,omitempty"`
	TransitionTarget    string                   `json:"transitionTarget,omitempty"`
}

type PageObservationControl struct {
	Role string `json:"role"`
	Name string `json:"name"`
}

type PageObservationLink struct {
	Text        string `json:"text"`
	Href        string `json:"href,omitempty"`
	HrefPattern string `json:"hrefPattern,omitempty"`
}

type NormalizedPageObservation struct {
	PageStateID       string
	Surface           *NormalizedPageSurface
	ProjectID         string
	Site              string
	URL               string
	URLPattern        string
	Title             string
	VisibleTextSample string
	Controls          []registry.ControlSignature
	Links             []registry.PageLinkSummary
	Fingerprint       string
	SearchableText    string
	HardRules         registry.HardRules
}

type NormalizedPageSurface struct {
	ID              string
	Type            registry.SurfaceType
	Title           string
	Fingerprint     string
	Controls        []registry.ControlSignature
	Text            []string
	VisibilityRules registry.HardRules
}

type PageObservationResponse struct {
	PageStateID      string                    `json:"pageStateId"`
	SurfaceID        string                    `json:"surfaceId,omitempty"`
	SurfaceType      registry.SurfaceType      `json:"surfaceType,omitempty"`
	Matched          bool                      `json:"matched"`
	Confidence       float64                   `json:"confidence"`
	PageSummary      string                    `json:"pageSummary"`
	KnownTransitions []KnownTransitionResponse `json:"knownTransitions,omitempty"`
}

type KnownTransitionResponse struct {
	ToPageStateID string  `json:"toPageStateId"`
	ActionName    string  `json:"actionName,omitempty"`
	TargetName    string  `json:"targetName,omitempty"`
	Confidence    float64 `json:"confidence,omitempty"`
}

type PageObservationService struct {
	repo registry.Repository
}

func NewPageObservationService(repo registry.Repository) *PageObservationService {
	return &PageObservationService{repo: repo}
}

func NormalizePageObservation(request PageObservationRequest) (NormalizedPageObservation, error) {
	projectID := strings.TrimSpace(request.ProjectID)
	if projectID == "" {
		return NormalizedPageObservation{}, errors.New("projectId is required")
	}
	urlPattern, site := normalizeObservationURL(request.URL)
	if site == "" {
		return NormalizedPageObservation{}, errors.New("url is required")
	}
	controls, err := ExtractControlSignatures(request.Controls)
	if err != nil {
		return NormalizedPageObservation{}, err
	}
	baseControls, surface := splitSurfaceControls(controls, request.VisibleText)
	links := normalizeObservationLinks(request.Links)
	visibleText := TruncateSummary(strings.TrimSpace(strings.Join(request.VisibleText, "\n")))
	if ContainsSensitiveMaterial(visibleText) {
		return NormalizedPageObservation{}, errors.New("visible text contains sensitive material")
	}
	normalized := NormalizedPageObservation{
		ProjectID:         projectID,
		Site:              site,
		URL:               strings.TrimSpace(request.URL),
		URLPattern:        urlPattern,
		Title:             strings.TrimSpace(request.Title),
		VisibleTextSample: visibleText,
		Controls:          baseControls,
		Links:             links,
		Surface:           surface,
	}
	normalized.HardRules = BuildPageHardRules(normalized)
	normalized.Fingerprint = BuildPageFingerprint(normalized)
	normalized.SearchableText = strings.TrimSpace(strings.Join([]string{
		normalized.URLPattern,
		normalized.Title,
		normalized.VisibleTextSample,
		controlText(normalized.Controls),
		linkText(normalized.Links),
	}, "\n"))
	if err := ValidateSearchableText(normalized.SearchableText); err != nil {
		return NormalizedPageObservation{}, err
	}
	return normalized, nil
}

func BuildPageFingerprint(observation NormalizedPageObservation) string {
	controls := append([]registry.ControlSignature(nil), observation.Controls...)
	sort.Slice(controls, func(i, j int) bool {
		if controls[i].Role == controls[j].Role {
			return controls[i].Name < controls[j].Name
		}
		return controls[i].Role < controls[j].Role
	})
	hash := sha1.Sum([]byte(strings.Join([]string{
		observation.ProjectID,
		observation.Site,
		observation.URLPattern,
		observation.Title,
		controlText(controls),
	}, "\x00")))
	return "fp_" + hex.EncodeToString(hash[:8])
}

func BuildSurfaceFingerprint(observation NormalizedPageObservation, surface NormalizedPageSurface) string {
	controls := append([]registry.ControlSignature(nil), surface.Controls...)
	sort.Slice(controls, func(i, j int) bool {
		if controls[i].Role == controls[j].Role {
			return controls[i].Name < controls[j].Name
		}
		return controls[i].Role < controls[j].Role
	})
	hash := sha1.Sum([]byte(strings.Join([]string{
		observation.ProjectID,
		observation.Site,
		observation.URLPattern,
		string(surface.Type),
		surface.Title,
		controlText(controls),
	}, "\x00")))
	return "surf_fp_" + hex.EncodeToString(hash[:8])
}

func ExtractControlSignatures(controls []PageObservationControl) ([]registry.ControlSignature, error) {
	seen := map[string]bool{}
	result := []registry.ControlSignature{}
	for _, control := range controls {
		role := strings.TrimSpace(control.Role)
		name := strings.TrimSpace(control.Name)
		if role == "" && name == "" {
			continue
		}
		if ContainsSensitiveMaterial(role) || ContainsSensitiveMaterial(name) {
			return nil, errors.New("control contains sensitive material")
		}
		key := strings.ToLower(role + "\x00" + name)
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, registry.ControlSignature{Role: role, Name: name})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Role == result[j].Role {
			return result[i].Name < result[j].Name
		}
		return result[i].Role < result[j].Role
	})
	return result, nil
}

func splitSurfaceControls(controls []registry.ControlSignature, visibleText []string) ([]registry.ControlSignature, *NormalizedPageSurface) {
	surfaceControls := []registry.ControlSignature{}
	baseControls := []registry.ControlSignature{}
	surfaceType := registry.SurfaceType("")
	title := ""
	for _, control := range controls {
		if detected := detectSurfaceType(control); detected != "" {
			if surfaceType == "" {
				surfaceType = detected
				title = control.Name
			}
			surfaceControls = append(surfaceControls, control)
			continue
		}
		if surfaceType != "" && isLikelySurfaceControl(control) {
			surfaceControls = append(surfaceControls, control)
			continue
		}
		baseControls = append(baseControls, control)
	}
	if surfaceType == "" {
		return baseControls, nil
	}
	surface := &NormalizedPageSurface{
		Type:     surfaceType,
		Title:    title,
		Controls: surfaceControls,
		Text:     surfaceText(visibleText, title),
	}
	surface.VisibilityRules = registry.HardRules{
		TextAny:     surface.Text,
		ControlsAny: append([]registry.ControlSignature(nil), surfaceControls...),
	}
	return baseControls, surface
}

func detectSurfaceType(control registry.ControlSignature) registry.SurfaceType {
	role := strings.ToLower(strings.TrimSpace(control.Role))
	name := strings.ToLower(strings.TrimSpace(control.Name))
	switch {
	case strings.Contains(role, "dialog") || strings.Contains(name, "弹窗") || strings.Contains(name, "确认"):
		return registry.SurfaceModal
	case strings.Contains(role, "drawer") || strings.Contains(name, "抽屉"):
		return registry.SurfaceDrawer
	case strings.Contains(role, "menu") || strings.Contains(role, "popover"):
		return registry.SurfacePopover
	case strings.Contains(role, "listbox") || strings.Contains(role, "option"):
		return registry.SurfaceDropdown
	default:
		return ""
	}
}

func isLikelySurfaceControl(control registry.ControlSignature) bool {
	name := strings.ToLower(strings.TrimSpace(control.Name))
	return strings.Contains(name, "确认") ||
		strings.Contains(name, "取消") ||
		strings.Contains(name, "关闭") ||
		strings.Contains(name, "close") ||
		strings.Contains(name, "cancel") ||
		strings.Contains(name, "confirm")
}

func surfaceText(visibleText []string, title string) []string {
	result := []string{}
	if strings.TrimSpace(title) != "" {
		result = append(result, title)
	}
	for _, text := range visibleText {
		text = strings.TrimSpace(text)
		if text == "" || containsString(result, text) {
			continue
		}
		if strings.Contains(text, title) || strings.Contains(text, "确认") || strings.Contains(strings.ToLower(text), "confirm") {
			result = append(result, text)
		}
		if len(result) >= 5 {
			break
		}
	}
	return result
}

func BuildPageHardRules(observation NormalizedPageObservation) registry.HardRules {
	requiredText := []string{}
	if observation.Title != "" {
		requiredText = append(requiredText, observation.Title)
	}
	controls := observation.Controls
	if len(controls) > 5 {
		controls = controls[:5]
	}
	return registry.HardRules{
		URLPattern:  observation.URLPattern,
		TextAll:     requiredText,
		ControlsAll: append([]registry.ControlSignature(nil), controls...),
	}
}

func (service *PageObservationService) ObservePage(ctx context.Context, request PageObservationRequest) (PageObservationResponse, error) {
	if service.repo == nil {
		return PageObservationResponse{}, errors.New("workflow registry is not configured")
	}
	normalized, err := NormalizePageObservation(request)
	if err != nil {
		return PageObservationResponse{}, err
	}
	event := registry.PageObservationEvent{
		ID:                stablePageObservationEventID(normalized),
		ProjectID:         normalized.ProjectID,
		Site:              normalized.Site,
		URL:               normalized.URL,
		URLPattern:        normalized.URLPattern,
		Title:             normalized.Title,
		VisibleTextSample: normalized.VisibleTextSample,
		Controls:          normalized.Controls,
		Links:             normalized.Links,
		Fingerprint:       normalized.Fingerprint,
		Payload: map[string]any{
			"source": request.Source,
			"task":   request.Task,
		},
	}
	if err := service.repo.SavePageObservationEvent(ctx, event); err != nil {
		return PageObservationResponse{}, err
	}
	pageState, matched, err := service.findOrCreatePageState(ctx, normalized)
	if err != nil {
		return PageObservationResponse{}, err
	}
	surfaceID := ""
	surfaceType := registry.SurfaceType("")
	if normalized.Surface != nil {
		surface, err := service.findOrCreatePageSurface(ctx, normalized, pageState.ID)
		if err != nil {
			return PageObservationResponse{}, err
		}
		surfaceID = surface.ID
		surfaceType = surface.SurfaceType
		pageState = attachSurfaceID(pageState, surface.ID)
		if err := service.repo.SavePageState(ctx, pageState); err != nil {
			return PageObservationResponse{}, err
		}
	}
	if request.PreviousPageStateID != "" && request.PreviousPageStateID != pageState.ID {
		if err := service.saveTransition(ctx, normalized, request, pageState.ID); err != nil {
			return PageObservationResponse{}, err
		}
	}
	transitions, err := service.repo.ListPageTransitions(ctx, registry.PageTransitionListQuery{
		ProjectID:       normalized.ProjectID,
		Site:            normalized.Site,
		FromPageStateID: pageState.ID,
	})
	if err != nil {
		return PageObservationResponse{}, err
	}
	return PageObservationResponse{
		PageStateID:      pageState.ID,
		SurfaceID:        surfaceID,
		SurfaceType:      surfaceType,
		Matched:          matched,
		Confidence:       pageMatchConfidence(matched),
		PageSummary:      summarizePage(normalized),
		KnownTransitions: knownTransitionResponses(transitions),
	}, nil
}

func (service *PageObservationService) findOrCreatePageSurface(ctx context.Context, observation NormalizedPageObservation, parentPageStateID string) (registry.PageSurface, error) {
	surface := observation.Surface
	if surface == nil {
		return registry.PageSurface{}, errors.New("surface observation is required")
	}
	surface.Fingerprint = BuildSurfaceFingerprint(observation, *surface)
	surface.ID = stableSurfaceID(observation, parentPageStateID, *surface)
	existing, err := service.repo.ListPageSurfaces(ctx, registry.PageSurfaceListQuery{
		ProjectID:         observation.ProjectID,
		Site:              observation.Site,
		ParentPageStateID: parentPageStateID,
	})
	if err == nil {
		for _, item := range existing {
			if item.SurfaceFingerprint == surface.Fingerprint {
				item.RequiredControls = mergeControls(item.RequiredControls, surface.Controls)
				item.RequiredText = mergeStrings(item.RequiredText, surface.Text)
				item.VisibilityRules = surface.VisibilityRules
				if err := service.repo.SavePageSurface(ctx, item); err != nil {
					return registry.PageSurface{}, err
				}
				return item, nil
			}
		}
	}
	value := registry.PageSurface{
		ID:                 surface.ID,
		ProjectID:          observation.ProjectID,
		Site:               observation.Site,
		ParentPageStateID:  parentPageStateID,
		SurfaceType:        surface.Type,
		SurfaceFingerprint: surface.Fingerprint,
		Title:              surface.Title,
		RequiredText:       surface.Text,
		RequiredControls:   surface.Controls,
		VisibilityRules:    surface.VisibilityRules,
		Status:             registry.StatusActive,
	}
	if err := service.repo.SavePageSurface(ctx, value); err != nil {
		return registry.PageSurface{}, err
	}
	return value, nil
}

func attachSurfaceID(page registry.PageState, surfaceID string) registry.PageState {
	if surfaceID == "" || containsString(page.SurfaceIDs, surfaceID) {
		return page
	}
	page.SurfaceIDs = append(page.SurfaceIDs, surfaceID)
	return page
}

func (service *PageObservationService) findOrCreatePageState(ctx context.Context, observation NormalizedPageObservation) (registry.PageState, bool, error) {
	pages, err := service.repo.ListPageStates(ctx, registry.PageStateListQuery{ProjectID: observation.ProjectID, Site: observation.Site})
	if err != nil {
		return registry.PageState{}, false, err
	}
	for _, page := range pages {
		if page.URLPattern == observation.URLPattern && strings.EqualFold(page.CanonicalTitle, observation.Title) {
			page.RequiredControls = mergeControls(page.RequiredControls, observation.Controls)
			page.StableControls = mergeControls(page.StableControls, observation.Controls)
			page.RequiredText = mergeStrings(page.RequiredText, observation.HardRules.TextAll)
			page.HardRules = BuildPageHardRules(observation)
			page.BaseFingerprint = observation.Fingerprint
			page.Confidence = 1
			if page.Status == "" {
				page.Status = registry.StatusActive
			}
			if err := service.repo.SavePageState(ctx, page); err != nil {
				return registry.PageState{}, false, err
			}
			return page, true, nil
		}
	}
	page := registry.PageState{
		ID:               stablePageStateID(observation),
		ProjectID:        observation.ProjectID,
		Site:             observation.Site,
		URLPattern:       observation.URLPattern,
		HardRules:        observation.HardRules,
		RequiredText:     observation.HardRules.TextAll,
		RequiredControls: observation.Controls,
		StableControls:   observation.Controls,
		CanonicalTitle:   observation.Title,
		BaseFingerprint:  observation.Fingerprint,
		Confidence:       1,
		Status:           registry.StatusActive,
	}
	if err := service.repo.SavePageState(ctx, page); err != nil {
		return registry.PageState{}, false, err
	}
	return page, false, nil
}

func (service *PageObservationService) saveTransition(ctx context.Context, observation NormalizedPageObservation, request PageObservationRequest, toPageStateID string) error {
	actionName := strings.TrimSpace(request.TransitionAction)
	if actionName == "" {
		actionName = "Navigate to " + observation.Title
	}
	transition := registry.PageTransition{
		ID:            stableTransitionID(request.PreviousPageStateID, toPageStateID, actionName),
		ProjectID:     observation.ProjectID,
		Site:          observation.Site,
		FromPageState: request.PreviousPageStateID,
		ToPageState:   toPageStateID,
		ActionName:    actionName,
		TargetName:    strings.TrimSpace(request.TransitionTarget),
		GuardRules:    observation.HardRules,
		RiskLevel:     registry.RiskReadOrSearch,
	}
	return service.repo.SavePageTransition(ctx, transition)
}

func normalizeObservationURL(rawURL string) (string, string) {
	rawURL = strings.TrimSpace(rawURL)
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host == "" {
		return "", ""
	}
	path := normalizePathPattern(parsed.EscapedPath())
	if path == "" {
		path = "/"
	}
	return parsed.Scheme + "://" + strings.ToLower(parsed.Host) + path, strings.ToLower(parsed.Host)
}

func normalizePathPattern(path string) string {
	segments := strings.Split(path, "/")
	for index, segment := range segments {
		if looksLikePathInstance(segment) {
			segments[index] = ":id"
		}
	}
	return strings.Join(segments, "/")
}

func looksLikePathInstance(segment string) bool {
	segment, _ = url.PathUnescape(segment)
	if segment == "" {
		return false
	}
	if regexp.MustCompile(`^\d{3,}$`).MatchString(segment) {
		return true
	}
	if regexp.MustCompile(`^[A-Za-z]+-[A-Za-z0-9]+-[A-Za-z0-9]+$`).MatchString(segment) {
		return true
	}
	return regexp.MustCompile(`(?i)^[0-9a-f]{8,}$`).MatchString(segment)
}

func normalizeObservationLinks(links []PageObservationLink) []registry.PageLinkSummary {
	seen := map[string]bool{}
	result := []registry.PageLinkSummary{}
	for _, link := range links {
		text := strings.TrimSpace(link.Text)
		pattern := strings.TrimSpace(link.HrefPattern)
		if pattern == "" && strings.TrimSpace(link.Href) != "" {
			pattern, _ = normalizeObservationURL(link.Href)
		}
		if text == "" && pattern == "" {
			continue
		}
		if ContainsSensitiveMaterial(text) || ContainsSensitiveMaterial(pattern) {
			continue
		}
		key := strings.ToLower(text + "\x00" + pattern)
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, registry.PageLinkSummary{Text: text, URLPattern: pattern})
	}
	return result
}

func stablePageStateID(observation NormalizedPageObservation) string {
	base := slugify(strings.Join([]string{observation.Title, observation.URLPattern}, " "))
	if base == "" {
		base = "page"
	}
	hash := sha1.Sum([]byte(observation.Fingerprint))
	return base + "_" + hex.EncodeToString(hash[:4])
}

func stablePageObservationEventID(observation NormalizedPageObservation) string {
	hash := sha1.Sum([]byte(strings.Join([]string{
		observation.ProjectID,
		observation.Site,
		observation.URLPattern,
		observation.Title,
	}, "\x00")))
	return "obs_" + hex.EncodeToString(hash[:8])
}

func stableSurfaceID(observation NormalizedPageObservation, parentPageStateID string, surface NormalizedPageSurface) string {
	hash := sha1.Sum([]byte(strings.Join([]string{
		observation.ProjectID,
		observation.Site,
		parentPageStateID,
		string(surface.Type),
		surface.Fingerprint,
	}, "\x00")))
	return "surface_" + hex.EncodeToString(hash[:8])
}

func stableTransitionID(fromPageStateID, toPageStateID, actionName string) string {
	hash := sha1.Sum([]byte(strings.Join([]string{fromPageStateID, toPageStateID, actionName}, "\x00")))
	return "transition_" + hex.EncodeToString(hash[:8])
}

func slugify(value string) string {
	value = strings.ToLower(value)
	parts := regexp.MustCompile(`[^a-z0-9]+`).Split(value, -1)
	kept := []string{}
	for _, part := range parts {
		if part != "" {
			kept = append(kept, part)
		}
	}
	if len(kept) == 0 {
		return "page"
	}
	result := strings.Join(kept, "_")
	if len(result) > 40 {
		result = result[:40]
	}
	return "page_" + strings.Trim(result, "_")
}

func summarizePage(observation NormalizedPageObservation) string {
	summary := strings.TrimSpace(strings.Join([]string{observation.Title, observation.VisibleTextSample}, " - "))
	if summary == "" {
		summary = observation.URLPattern
	}
	return TruncateSummary(summary)
}

func pageMatchConfidence(matched bool) float64 {
	if matched {
		return 0.91
	}
	return 0.62
}

func knownTransitionResponses(transitions []registry.PageTransition) []KnownTransitionResponse {
	result := make([]KnownTransitionResponse, 0, len(transitions))
	for _, transition := range transitions {
		result = append(result, KnownTransitionResponse{
			ToPageStateID: transition.ToPageState,
			ActionName:    transition.ActionName,
			TargetName:    transition.TargetName,
			Confidence:    0.8,
		})
	}
	return result
}

func controlText(controls []registry.ControlSignature) string {
	parts := make([]string, 0, len(controls))
	for _, control := range controls {
		parts = append(parts, strings.TrimSpace(control.Role+" "+control.Name))
	}
	return strings.Join(parts, " ")
}

func linkText(links []registry.PageLinkSummary) string {
	parts := make([]string, 0, len(links))
	for _, link := range links {
		parts = append(parts, strings.TrimSpace(link.Text+" "+link.URLPattern))
	}
	return strings.Join(parts, " ")
}

func mergeControls(left, right []registry.ControlSignature) []registry.ControlSignature {
	seen := map[string]bool{}
	result := []registry.ControlSignature{}
	for _, control := range append(append([]registry.ControlSignature{}, left...), right...) {
		key := strings.ToLower(control.Role + "\x00" + control.Name)
		if key == "\x00" || seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, control)
	}
	return result
}

func mergeStrings(left, right []string) []string {
	seen := map[string]bool{}
	result := []string{}
	for _, value := range append(append([]string{}, left...), right...) {
		value = strings.TrimSpace(value)
		key := strings.ToLower(value)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, value)
	}
	return result
}
