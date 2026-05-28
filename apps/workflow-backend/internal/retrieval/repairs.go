package retrieval

import "github.com/page-agent/workflow-backend/internal/registry"

type RepairSearcher interface {
	SearchRepairs(RepairRequest) ([]RepairHit, error)
}

type RepairRequest struct {
	ProjectID        string     `json:"projectId,omitempty"`
	WorkflowID       string     `json:"workflowId,omitempty"`
	WorkflowVersion  int        `json:"version,omitempty"`
	ChunkID          string     `json:"chunkId,omitempty"`
	StepID           string     `json:"stepId,omitempty"`
	Site             string     `json:"site,omitempty"`
	FailureType      string     `json:"failureType,omitempty"`
	CurrentPageState string     `json:"currentPageState,omitempty"`
	RiskPolicy       RiskPolicy `json:"riskPolicy,omitempty"`
}

type RepairHit struct {
	PatchID             string
	WorkflowID          string
	WorkflowVersion     int
	ChunkID             string
	StepID              string
	Score               float64
	Site                string
	FailureType         string
	AppliesToPageStates []string
	RiskLevel           registry.RiskLevel
	Reasons             []string
}

type RepairService struct {
	searcher  RepairSearcher
	threshold float64
}

func NewRepairService(searcher RepairSearcher) *RepairService {
	return &RepairService{searcher: searcher, threshold: 0.75}
}

func (service *RepairService) Search(request RepairRequest) (RepairHit, bool) {
	hits, err := service.searcher.SearchRepairs(request)
	if err != nil {
		return RepairHit{}, false
	}
	for _, hit := range hits {
		if hit.Score < service.threshold {
			continue
		}
		if hit.WorkflowID != request.WorkflowID {
			continue
		}
		if hit.WorkflowVersion != request.WorkflowVersion {
			continue
		}
		if hit.FailureType != request.FailureType {
			continue
		}
		if riskBlocked(hit.RiskLevel, request.RiskPolicy) {
			continue
		}
		if !stringIn(request.CurrentPageState, hit.AppliesToPageStates) {
			continue
		}
		return hit, true
	}
	return RepairHit{}, false
}

func stringIn(value string, values []string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}
