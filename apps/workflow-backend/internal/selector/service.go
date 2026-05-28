package selector

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

type MemoryRepository struct {
	mu    sync.RWMutex
	runs  map[string]WorkflowRun
	stats map[string]SelectorStat
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{runs: map[string]WorkflowRun{}, stats: map[string]SelectorStat{}}
}

type Service struct {
	repo *MemoryRepository
}

func NewService(repo *MemoryRepository) *Service {
	return &Service{repo: repo}
}

func (service *Service) RecordRun(_ context.Context, run WorkflowRun) (WorkflowRun, error) {
	if run.ID == "" {
		return WorkflowRun{}, errors.New("run id is required")
	}
	if run.ProjectID == "" {
		return WorkflowRun{}, errors.New("project id is required")
	}
	if run.Result == "" {
		return WorkflowRun{}, errors.New("result is required")
	}
	if run.CreatedAt.IsZero() {
		run.CreatedAt = time.Now().UTC()
	}
	service.repo.mu.Lock()
	defer service.repo.mu.Unlock()
	service.repo.runs[run.ID] = run
	return run, nil
}

func (service *Service) RecordSelectorStats(_ context.Context, runID string, updates []SelectorStatUpdate) error {
	service.repo.mu.Lock()
	defer service.repo.mu.Unlock()
	if _, ok := service.repo.runs[runID]; !ok {
		return errors.New("workflow run not found")
	}
	now := time.Now().UTC()
	for _, update := range updates {
		key := statKey(update)
		stat := service.repo.stats[key]
		stat.WorkflowID = update.WorkflowID
		stat.WorkflowVersion = update.WorkflowVersion
		stat.StepID = update.StepID
		stat.Strategy = update.Strategy
		stat.Selector = update.Selector
		if update.Success {
			stat.SuccessCount++
			stat.LastSuccessAt = now
		} else {
			stat.FailCount++
			stat.LastFailAt = now
			stat.LastFailureReason = update.FailureReason
		}
		service.repo.stats[key] = stat
	}
	return nil
}

func (service *Service) Stats(_ context.Context, workflowID string, version int) ([]SelectorStat, error) {
	service.repo.mu.RLock()
	defer service.repo.mu.RUnlock()
	result := make([]SelectorStat, 0)
	for _, stat := range service.repo.stats {
		if stat.WorkflowID == workflowID && stat.WorkflowVersion == version {
			result = append(result, stat)
		}
	}
	return result, nil
}

func statKey(update SelectorStatUpdate) string {
	return fmt.Sprintf("%s:%d:%s:%s:%s", update.WorkflowID, update.WorkflowVersion, update.StepID, update.Strategy, update.Selector)
}
