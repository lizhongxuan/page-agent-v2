package memory

import (
	"context"
	"errors"
	"sort"
	"strings"

	"github.com/page-agent/workflow-backend/internal/knowledge"
	"github.com/page-agent/workflow-backend/internal/registry"
)

type BusinessProfileService struct {
	repo registry.Repository
}

func NewBusinessProfileService(repo registry.Repository) *BusinessProfileService {
	return &BusinessProfileService{repo: repo}
}

func (service *BusinessProfileService) UpdateFromDocuments(ctx context.Context, documents []knowledge.DocumentInput) (registry.BusinessSystemProfile, bool, error) {
	if service.repo == nil {
		return registry.BusinessSystemProfile{}, false, errors.New("workflow registry is not configured")
	}
	if len(documents) == 0 {
		return registry.BusinessSystemProfile{}, false, nil
	}
	var last registry.BusinessSystemProfile
	updated := false
	for _, document := range documents {
		if ContainsSensitiveMaterial(document.Title) || ContainsSensitiveMaterial(document.Source) {
			return registry.BusinessSystemProfile{}, false, errors.New("document metadata contains sensitive material")
		}
		sourceType := documentSourceType(document.SourceType)
		if sourceType != registry.MemorySourceProduction {
			continue
		}
		projectID := strings.TrimSpace(document.ProjectID)
		if projectID == "" {
			projectID = "default"
		}
		_, site := normalizeObservationURL(document.URL)
		moduleName := inferModuleName(document)
		purpose := inferPurpose(document)
		if ContainsSensitiveMaterial(purpose) {
			return registry.BusinessSystemProfile{}, false, errors.New("document purpose contains sensitive material")
		}
		existing, _ := service.repo.GetBusinessSystemProfile(ctx, registry.BusinessSystemProfileQuery{
			ProjectID:  projectID,
			Site:       site,
			Module:     moduleName,
			SourceType: registry.MemorySourceProduction,
		})
		profile := existing
		profile.ProjectID = projectID
		profile.Site = site
		profile.Module = moduleName
		profile.SourceType = registry.MemorySourceProduction
		profile.Status = registry.StatusActive
		if profile.Terms == nil {
			profile.Terms = map[string]string{}
		}
		sourceRefs := sourceRefSet(profile.SourceRefs)
		modules := moduleMap(profile.Modules)
		entryPages := entryPageMap(profile.EntryPages)
		if moduleName != "" {
			module := modules[moduleName]
			module.Name = moduleName
			if purpose != "" {
				module.Purpose = TruncateSummary(purpose)
			}
			if strings.TrimSpace(document.URL) != "" {
				pattern, _ := normalizeObservationURL(document.URL)
				entryID := stableEntryPageID(pattern, moduleName)
				module.EntryPageStateID = entryID
				entryPages[entryID] = registry.EntryPageRef{
					PageStateID: entryID,
					Name:        moduleName,
					URLPattern:  pattern,
				}
			}
			modules[moduleName] = module
			profile.Terms[moduleName] = purpose
		}
		if document.ID != "" {
			sourceRefs["document:"+document.ID] = registry.MemorySourceRef{Type: "document", ID: document.ID}
		}
		profile.Modules = sortedModules(modules)
		profile.EntryPages = sortedEntryPages(entryPages)
		profile.SourceRefs = sortedSourceRefs(sourceRefs)
		if moduleName != "" && purpose != "" {
			profile.Summary = TruncateSummary(moduleName + "：" + purpose)
		}
		if profile.Confidence == 0 {
			profile.Confidence = 0.7
		}
		if err := service.repo.SaveBusinessSystemProfile(ctx, profile); err != nil {
			return registry.BusinessSystemProfile{}, false, err
		}
		last = profile
		updated = true
	}
	return last, updated, nil
}

func documentSourceType(value string) registry.MemorySourceType {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case string(registry.MemorySourceTest):
		return registry.MemorySourceTest
	case string(registry.MemorySourceSeed):
		return registry.MemorySourceSeed
	case string(registry.MemorySourceExample):
		return registry.MemorySourceExample
	default:
		return registry.MemorySourceProduction
	}
}

func inferModuleName(document knowledge.DocumentInput) string {
	for _, tag := range document.Tags {
		tag = strings.TrimSpace(tag)
		if tag != "" && !ContainsSensitiveMaterial(tag) {
			return cleanModuleName(tag)
		}
	}
	title := strings.TrimSpace(document.Title)
	if title == "" {
		return "Business System"
	}
	return cleanModuleName(title)
}

func cleanModuleName(value string) string {
	replacers := []string{"手册", "说明", "SOP", "Manual", "manual", "Guide", "guide"}
	value = strings.TrimSpace(value)
	for _, item := range replacers {
		value = strings.TrimSpace(strings.ReplaceAll(value, item, ""))
	}
	if value == "" {
		return "Business System"
	}
	return value
}

func inferPurpose(document knowledge.DocumentInput) string {
	content := strings.TrimSpace(document.Content)
	lines := strings.Split(content, "\n")
	candidates := []string{}
	for _, line := range lines {
		line = strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(line), "#"))
		if line != "" {
			candidates = append(candidates, line)
		}
	}
	if len(candidates) == 0 {
		return strings.TrimSpace(document.Title)
	}
	if len(candidates) == 1 {
		return TruncateSummary(candidates[0])
	}
	return TruncateSummary(candidates[0] + " " + candidates[1])
}

func stableEntryPageID(urlPattern, moduleName string) string {
	normalized := NormalizedPageObservation{
		ProjectID:  "profile",
		Site:       "",
		URLPattern: urlPattern,
		Title:      moduleName,
	}
	normalized.Fingerprint = BuildPageFingerprint(normalized)
	return stablePageStateID(normalized)
}

func moduleMap(values []registry.BusinessModule) map[string]registry.BusinessModule {
	result := map[string]registry.BusinessModule{}
	for _, value := range values {
		if value.Name != "" {
			result[value.Name] = value
		}
	}
	return result
}

func entryPageMap(values []registry.EntryPageRef) map[string]registry.EntryPageRef {
	result := map[string]registry.EntryPageRef{}
	for _, value := range values {
		if value.PageStateID != "" {
			result[value.PageStateID] = value
		}
	}
	return result
}

func sourceRefSet(values []registry.MemorySourceRef) map[string]registry.MemorySourceRef {
	result := map[string]registry.MemorySourceRef{}
	for _, value := range values {
		if value.Type != "" && value.ID != "" {
			result[value.Type+":"+value.ID] = value
		}
	}
	return result
}

func sortedModules(values map[string]registry.BusinessModule) []registry.BusinessModule {
	keys := sortedKeys(values)
	result := make([]registry.BusinessModule, 0, len(keys))
	for _, key := range keys {
		result = append(result, values[key])
	}
	return result
}

func sortedEntryPages(values map[string]registry.EntryPageRef) []registry.EntryPageRef {
	keys := sortedKeys(values)
	result := make([]registry.EntryPageRef, 0, len(keys))
	for _, key := range keys {
		result = append(result, values[key])
	}
	return result
}

func sortedSourceRefs(values map[string]registry.MemorySourceRef) []registry.MemorySourceRef {
	keys := sortedKeys(values)
	result := make([]registry.MemorySourceRef, 0, len(keys))
	for _, key := range keys {
		result = append(result, values[key])
	}
	return result
}

func sortedKeys[T any](values map[string]T) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
