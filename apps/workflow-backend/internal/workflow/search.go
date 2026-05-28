package workflow

import (
	"net/url"
	"sort"
	"strings"
)

type SearchRequest struct {
	ProjectID       string          `json:"projectId"`
	Task            string          `json:"task"`
	URL             string          `json:"url"`
	Domain          string          `json:"domain,omitempty"`
	PageFingerprint PageFingerprint `json:"pageFingerprint"`
	Limit           int             `json:"limit,omitempty"`
}

type SearchResult struct {
	Recipe WorkflowRecipe `json:"recipe"`
	Score  float64        `json:"score"`
	Reason []string       `json:"reasons"`
}

func rankWorkflows(recipes []WorkflowRecipe, request SearchRequest) []SearchResult {
	results := make([]SearchResult, 0)
	for _, recipe := range recipes {
		if recipe.Status != WorkflowStatusActive {
			continue
		}
		if !matchesSite(recipe, request) {
			continue
		}
		fpScore := fingerprintScore(recipe.PageFingerprint, request.PageFingerprint)
		if fpScore < recipe.PageFingerprint.MinMatchScore {
			continue
		}
		intentScore := keywordScore(recipe.Intent+" "+recipe.Name+" "+strings.Join(recipe.Tags, " "), request.Task)
		score := 0.35*intentScore + 0.25 + 0.2*fpScore + 0.2
		results = append(results, SearchResult{
			Recipe: recipe,
			Score:  score,
			Reason: []string{"status active", "site matched", "fingerprint matched"},
		})
	}
	sort.Slice(results, func(i, j int) bool { return results[i].Score > results[j].Score })
	if request.Limit > 0 && len(results) > request.Limit {
		results = results[:request.Limit]
	}
	return results
}

func matchesSite(recipe WorkflowRecipe, request SearchRequest) bool {
	domain := request.Domain
	if domain == "" && request.URL != "" {
		parsed, err := url.Parse(request.URL)
		if err == nil {
			domain = parsed.Hostname()
		}
	}
	return domain == "" || recipe.Site == "" || strings.Contains(domain, recipe.Site)
}

func fingerprintScore(expected PageFingerprint, actual PageFingerprint) float64 {
	total := 0
	matched := 0
	for _, required := range expected.RequiredText {
		total++
		if containsAny(actual.RequiredText, required) || containsAny(actual.TitleAny, required) {
			matched++
		}
	}
	for _, control := range expected.ControlSignatures {
		total++
		if hasControl(actual.ControlSignatures, control) {
			matched++
		}
	}
	if total == 0 {
		return 1
	}
	return float64(matched) / float64(total)
}

func keywordScore(haystack string, needle string) float64 {
	haystack = strings.ToLower(haystack)
	words := strings.Fields(strings.ToLower(needle))
	if len(words) == 0 {
		return 0.5
	}
	matches := 0
	for _, word := range words {
		if strings.Contains(haystack, word) {
			matches++
		}
	}
	return float64(matches) / float64(len(words))
}

func containsAny(values []string, needle string) bool {
	needle = strings.ToLower(needle)
	for _, value := range values {
		if strings.Contains(strings.ToLower(value), needle) || strings.Contains(needle, strings.ToLower(value)) {
			return true
		}
	}
	return false
}

func hasControl(controls []ControlSignature, expected ControlSignature) bool {
	for _, control := range controls {
		if expected.Role != "" && control.Role != expected.Role {
			continue
		}
		if expected.Name != "" && !strings.Contains(strings.ToLower(control.Name), strings.ToLower(expected.Name)) {
			continue
		}
		return true
	}
	return false
}
