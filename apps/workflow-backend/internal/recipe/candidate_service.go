package recipe

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/page-agent/workflow-backend/internal/workflow"
)

type CandidateListQuery struct {
	ProjectID    string
	Source       CandidateSource
	ReviewStatus ReviewStatus
}

type CandidateRepository interface {
	Save(context.Context, WorkflowCandidate) (WorkflowCandidate, error)
	Get(context.Context, string) (WorkflowCandidate, error)
	List(context.Context, CandidateListQuery) ([]WorkflowCandidate, error)
}

type MemoryCandidateRepository struct {
	mu         sync.RWMutex
	candidates map[string]WorkflowCandidate
}

func NewMemoryCandidateRepository() *MemoryCandidateRepository {
	return &MemoryCandidateRepository{candidates: map[string]WorkflowCandidate{}}
}

func (repo *MemoryCandidateRepository) Save(_ context.Context, candidate WorkflowCandidate) (WorkflowCandidate, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	repo.candidates[candidate.ID] = candidate
	return candidate, nil
}

func (repo *MemoryCandidateRepository) Get(_ context.Context, id string) (WorkflowCandidate, error) {
	repo.mu.RLock()
	defer repo.mu.RUnlock()
	candidate, ok := repo.candidates[id]
	if !ok {
		return WorkflowCandidate{}, errors.New("workflow candidate not found")
	}
	return candidate, nil
}

func (repo *MemoryCandidateRepository) List(_ context.Context, query CandidateListQuery) ([]WorkflowCandidate, error) {
	repo.mu.RLock()
	defer repo.mu.RUnlock()
	result := make([]WorkflowCandidate, 0)
	for _, candidate := range repo.candidates {
		if query.ProjectID != "" && candidate.ProjectID != query.ProjectID {
			continue
		}
		if query.Source != "" && candidate.Source != query.Source {
			continue
		}
		if query.ReviewStatus != "" && candidate.ReviewStatus != query.ReviewStatus {
			continue
		}
		result = append(result, candidate)
	}
	sortCandidates(result)
	return result, nil
}

type CandidateService struct {
	repo      CandidateRepository
	workflows *workflow.Service
}

func NewCandidateService(repo CandidateRepository, workflows *workflow.Service) *CandidateService {
	return &CandidateService{repo: repo, workflows: workflows}
}

func (service *CandidateService) CreateFromSession(ctx context.Context, session RecordedSession) (WorkflowCandidate, error) {
	candidate, err := GenerateCandidateFromSession(session)
	if err != nil {
		return WorkflowCandidate{}, err
	}
	return service.repo.Save(ctx, candidate)
}

func (service *CandidateService) Get(ctx context.Context, id string) (WorkflowCandidate, error) {
	return service.repo.Get(ctx, id)
}

func (service *CandidateService) List(ctx context.Context, query CandidateListQuery) ([]WorkflowCandidate, error) {
	return service.repo.List(ctx, query)
}

func (service *CandidateService) Approve(ctx context.Context, id string) (WorkflowCandidate, error) {
	candidate, err := service.repo.Get(ctx, id)
	if err != nil {
		return WorkflowCandidate{}, err
	}
	now := time.Now().UTC()
	candidate.ReviewStatus = ReviewStatusApproved
	candidate.NotificationStatus = NotificationStatusNotified
	candidate.Searchable = true
	candidate.UserConfirmedAt = &now
	candidate.SearchableAt = &now
	candidate.RecipeDraft.Status = workflow.WorkflowStatusActive
	candidate.RecipeDraft.Version = 1
	if _, err := service.workflows.Create(ctx, candidate.RecipeDraft); err != nil {
		return WorkflowCandidate{}, err
	}
	return service.repo.Save(ctx, candidate)
}

func (service *CandidateService) Reject(ctx context.Context, id string) (WorkflowCandidate, error) {
	candidate, err := service.repo.Get(ctx, id)
	if err != nil {
		return WorkflowCandidate{}, err
	}
	candidate.ReviewStatus = ReviewStatusRejected
	candidate.NotificationStatus = NotificationStatusDismissed
	candidate.Searchable = false
	return service.repo.Save(ctx, candidate)
}
