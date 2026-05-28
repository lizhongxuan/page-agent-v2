package retrieval

import "strings"

type PageStateSearcher interface {
	SearchPageStates(PageStateRequest) ([]PageStateHit, error)
}

type PageStateRequest struct {
	ProjectID       string
	Site            string
	App             string
	Observation     PageObservation
	FingerprintText string
}

type PageStateHit struct {
	PageStateID      string
	Site             string
	App              string
	Score            float64
	RequiredText     []string
	RequiredControls []Control
	Reasons          []string
}

type PageStateDetector struct {
	searcher  PageStateSearcher
	threshold float64
}

func NewPageStateDetector(searcher PageStateSearcher) *PageStateDetector {
	return &PageStateDetector{searcher: searcher, threshold: 0.75}
}

func (detector *PageStateDetector) Detect(request PageStateRequest) (PageStateHit, bool) {
	hits, err := detector.searcher.SearchPageStates(request)
	if err != nil {
		return PageStateHit{}, false
	}
	for _, hit := range hits {
		if hit.Score < detector.threshold {
			continue
		}
		if !requiredTextMatches(hit.RequiredText, request.Observation.VisibleText) {
			continue
		}
		if !requiredControlsMatch(hit.RequiredControls, request.Observation.Controls) {
			continue
		}
		return hit, true
	}
	return PageStateHit{}, false
}

func requiredTextMatches(required []string, actual []string) bool {
	joined := strings.ToLower(strings.Join(actual, " "))
	for _, text := range required {
		if !strings.Contains(joined, strings.ToLower(text)) {
			return false
		}
	}
	return true
}

func requiredControlsMatch(required []Control, actual []Control) bool {
	for _, want := range required {
		matched := false
		for _, got := range actual {
			if want.Role != "" && got.Role != want.Role {
				continue
			}
			if want.Name != "" && !strings.Contains(strings.ToLower(got.Name), strings.ToLower(want.Name)) {
				continue
			}
			matched = true
			break
		}
		if !matched {
			return false
		}
	}
	return true
}
