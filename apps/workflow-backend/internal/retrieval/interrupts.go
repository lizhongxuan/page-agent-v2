package retrieval

import "github.com/page-agent/workflow-backend/internal/registry"

type InterruptSearcher interface {
	SearchInterrupts(InterruptRequest) ([]InterruptHit, error)
}

type InterruptRequest struct {
	ProjectID        string          `json:"projectId,omitempty"`
	Site             string          `json:"site,omitempty"`
	CurrentPageState string          `json:"currentPageState,omitempty"`
	Observation      PageObservation `json:"pageObservation,omitempty"`
	FingerprintText  string          `json:"fingerprintText,omitempty"`
	RiskPolicy       RiskPolicy      `json:"riskPolicy,omitempty"`
}

type InterruptHit struct {
	HandlerID           string
	WorkflowID          string
	Version             int
	Score               float64
	Site                string
	RiskLevel           registry.RiskLevel
	AppliesToPageStates []string
	RequiredText        []string
	TargetControls      []Control
	Reasons             []string
}

type InterruptService struct {
	searcher  InterruptSearcher
	threshold float64
}

func NewInterruptService(searcher InterruptSearcher) *InterruptService {
	return &InterruptService{searcher: searcher, threshold: 0.75}
}

func (service *InterruptService) Search(request InterruptRequest) (InterruptHit, bool) {
	hits, err := service.searcher.SearchInterrupts(request)
	if err != nil {
		return InterruptHit{}, false
	}
	for _, hit := range hits {
		if hit.Score < service.threshold {
			continue
		}
		if riskBlocked(hit.RiskLevel, request.RiskPolicy) {
			continue
		}
		if !stringIn(request.CurrentPageState, hit.AppliesToPageStates) {
			continue
		}
		if !requiredTextMatches(hit.RequiredText, request.Observation.VisibleText) {
			continue
		}
		if !requiredControlsMatch(hit.TargetControls, request.Observation.Controls) {
			continue
		}
		return hit, true
	}
	return InterruptHit{}, false
}
