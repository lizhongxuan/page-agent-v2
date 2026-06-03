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
	"time"

	"github.com/page-agent/workflow-backend/internal/registry"
)

type PageObservationRequest struct {
	ProjectID      string                            `json:"projectId"`
	Task           string                            `json:"task,omitempty"`
	URL            string                            `json:"url"`
	Title          string                            `json:"title,omitempty"`
	VisibleText    []string                          `json:"visibleText,omitempty"`
	Controls       []PageObservationControl          `json:"controls,omitempty"`
	Links          []PageObservationLink             `json:"links,omitempty"`
	Breadcrumbs    []string                          `json:"breadcrumbs,omitempty"`
	ActiveTabs     []string                          `json:"activeTabs,omitempty"`
	Tables         []registry.ObservationTableSignal `json:"tables,omitempty"`
	ActiveSurfaces []registry.ActiveSurfaceSignal    `json:"activeSurfaces,omitempty"`
	Source         string                            `json:"source,omitempty"`
}

type PageObservationControl struct {
	Role     string `json:"role"`
	Name     string `json:"name"`
	Selected *bool  `json:"selected,omitempty"`
	Enabled  *bool  `json:"enabled,omitempty"`
}

type PageObservationLink struct {
	Text        string `json:"text"`
	Href        string `json:"href,omitempty"`
	HrefPattern string `json:"hrefPattern,omitempty"`
}

type NormalizedPageObservation struct {
	// Optional action-step page alias used for attribution and guide guard lookup.
	PageStateID       string
	Overlay           *NormalizedOverlaySignal
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

type NormalizedOverlaySignal struct {
	Type     registry.SurfaceType
	Title    string
	Controls []registry.ControlSignature
	Text     []string
}

type PageObservationResponse struct {
	ObservationID     string  `json:"observationId,omitempty"`
	PageStateID       string  `json:"pageStateId"`
	Matched           bool    `json:"matched"`
	Confidence        float64 `json:"confidence"`
	ActiveOverlayHint string  `json:"activeOverlayHint,omitempty"`
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
	baseControls, overlay := splitOverlayControls(controls, request.VisibleText)
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
		Overlay:           overlay,
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
		result = append(result, registry.ControlSignature{
			Role:     role,
			Name:     name,
			Selected: control.Selected,
			Enabled:  control.Enabled,
		})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Role == result[j].Role {
			return result[i].Name < result[j].Name
		}
		return result[i].Role < result[j].Role
	})
	return result, nil
}

func splitOverlayControls(controls []registry.ControlSignature, visibleText []string) ([]registry.ControlSignature, *NormalizedOverlaySignal) {
	overlayControls := []registry.ControlSignature{}
	baseControls := []registry.ControlSignature{}
	overlayType := registry.SurfaceType("")
	title := ""
	for _, control := range controls {
		if detected := detectOverlayType(control); detected != "" {
			if overlayType == "" {
				overlayType = detected
				title = control.Name
			}
			overlayControls = append(overlayControls, control)
			continue
		}
		if overlayType != "" && isLikelyOverlayControl(control) {
			overlayControls = append(overlayControls, control)
			continue
		}
		baseControls = append(baseControls, control)
	}
	if overlayType == "" {
		return baseControls, nil
	}
	overlay := &NormalizedOverlaySignal{
		Type:     overlayType,
		Title:    title,
		Controls: overlayControls,
		Text:     overlayText(visibleText, title),
	}
	return baseControls, overlay
}

func detectOverlayType(control registry.ControlSignature) registry.SurfaceType {
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

func isLikelyOverlayControl(control registry.ControlSignature) bool {
	name := strings.ToLower(strings.TrimSpace(control.Name))
	return strings.Contains(name, "确认") ||
		strings.Contains(name, "取消") ||
		strings.Contains(name, "关闭") ||
		strings.Contains(name, "close") ||
		strings.Contains(name, "cancel") ||
		strings.Contains(name, "confirm")
}

func overlayText(visibleText []string, title string) []string {
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
	eventID := stablePageObservationEventID(normalized)
	existing, _ := service.repo.ListPageObservationEvents(ctx, registry.PageObservationEventListQuery{
		ProjectID:  normalized.ProjectID,
		Site:       normalized.Site,
		URLPattern: normalized.URLPattern,
	})
	matched := false
	for _, item := range existing {
		if item.ID == eventID {
			matched = true
			break
		}
	}
	activeOverlayHint := ""
	if normalized.Overlay != nil {
		activeOverlayHint = strings.TrimSpace(strings.Join([]string{string(normalized.Overlay.Type), normalized.Overlay.Title}, ":"))
	}
	event := registry.PageObservationEvent{
		ID:                eventID,
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
			"source":            request.Source,
			"task":              request.Task,
			"activeOverlayHint": activeOverlayHint,
		},
	}
	if err := service.repo.SavePageObservationEvent(ctx, event); err != nil {
		return PageObservationResponse{}, err
	}
	if err := service.saveObservedPageState(ctx, event.ID, normalized); err != nil {
		return PageObservationResponse{}, err
	}
	return PageObservationResponse{
		ObservationID:     event.ID,
		PageStateID:       event.ID,
		Matched:           matched,
		Confidence:        pageMatchConfidence(matched),
		ActiveOverlayHint: activeOverlayHint,
	}, nil
}

func (service *PageObservationService) saveObservedPageState(ctx context.Context, pageStateID string, observation NormalizedPageObservation) error {
	now := time.Now().UTC()
	surfaceIDs := []string{}
	if observation.Overlay != nil {
		surface := observedPageSurface(pageStateID, observation, *observation.Overlay, now)
		if err := service.repo.SavePageSurface(ctx, surface); err != nil {
			return err
		}
		surfaceIDs = append(surfaceIDs, surface.ID)
	}
	page := registry.PageState{
		ID:                pageStateID,
		ProjectID:         observation.ProjectID,
		Site:              observation.Site,
		URLPattern:        observation.URLPattern,
		HardRules:         observation.HardRules,
		RequiredText:      observedPageRequiredText(observation),
		RequiredControls:  append([]registry.ControlSignature(nil), observation.Controls...),
		StableControls:    append([]registry.ControlSignature(nil), observation.Controls...),
		TransientControls: []registry.ControlSignature{},
		SurfaceIDs:        surfaceIDs,
		CanonicalTitle:    observation.Title,
		BaseFingerprint:   observation.Fingerprint,
		Confidence:        0.72,
		UpdatePolicy:      "merge_observation",
		LastStableSeenAt:  now,
		Status:            registry.StatusActive,
		UpdatedAt:         now,
	}
	return service.repo.SavePageState(ctx, page)
}

func observedPageSurface(pageStateID string, observation NormalizedPageObservation, overlay NormalizedOverlaySignal, now time.Time) registry.PageSurface {
	return registry.PageSurface{
		ID:                 stablePageSurfaceID(pageStateID, overlay),
		ProjectID:          observation.ProjectID,
		Site:               observation.Site,
		ParentPageStateID:  pageStateID,
		SurfaceType:        overlay.Type,
		SurfaceFingerprint: stablePageSurfaceFingerprint(pageStateID, overlay),
		Title:              overlay.Title,
		RequiredText:       append([]string(nil), overlay.Text...),
		RequiredControls:   append([]registry.ControlSignature(nil), overlay.Controls...),
		VisibilityRules: registry.HardRules{
			URLPattern:  observation.URLPattern,
			TextAny:     append([]string(nil), overlay.Text...),
			ControlsAll: append([]registry.ControlSignature(nil), overlay.Controls...),
		},
		ObservationCount: 1,
		Status:           registry.StatusActive,
		FirstSeenAt:      now,
		LastSeenAt:       now,
	}
}

func observedPageRequiredText(observation NormalizedPageObservation) []string {
	if strings.TrimSpace(observation.Title) == "" {
		return nil
	}
	return []string{strings.TrimSpace(observation.Title)}
}

func stablePageSurfaceID(pageStateID string, overlay NormalizedOverlaySignal) string {
	return "surface_" + stablePageSurfaceHash(pageStateID, overlay)
}

func stablePageSurfaceFingerprint(pageStateID string, overlay NormalizedOverlaySignal) string {
	return "surface_fp_" + stablePageSurfaceHash(pageStateID, overlay)
}

func stablePageSurfaceHash(pageStateID string, overlay NormalizedOverlaySignal) string {
	hash := sha1.Sum([]byte(strings.Join([]string{
		pageStateID,
		string(overlay.Type),
		overlay.Title,
		controlText(overlay.Controls),
		strings.Join(overlay.Text, "\n"),
	}, "\x00")))
	return hex.EncodeToString(hash[:8])
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

func stablePageObservationEventID(observation NormalizedPageObservation) string {
	hash := sha1.Sum([]byte(strings.Join([]string{
		observation.ProjectID,
		observation.Site,
		observation.URLPattern,
		observation.Title,
	}, "\x00")))
	return "obs_" + hex.EncodeToString(hash[:8])
}

func pageMatchConfidence(matched bool) float64 {
	if matched {
		return 0.91
	}
	return 0.62
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
