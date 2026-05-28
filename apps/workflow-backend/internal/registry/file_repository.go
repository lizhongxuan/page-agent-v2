package registry

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
	"time"
)

type FileRepository struct {
	mu         sync.Mutex
	dir        string
	candidates map[string]WorkflowCandidate
	workflows  map[string]WorkflowRecipe
	versions   map[string]WorkflowVersion
	runs       map[string]WorkflowRun
	stats      map[string]SelectorStats
	interrupts map[string]InterruptHandler
	repairs    map[string]RepairPatch
	outbox     map[string]OutboxEvent
}

func NewFileRepository(dir string) (*FileRepository, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	repo := &FileRepository{
		dir:        dir,
		candidates: map[string]WorkflowCandidate{},
		workflows:  map[string]WorkflowRecipe{},
		versions:   map[string]WorkflowVersion{},
		runs:       map[string]WorkflowRun{},
		stats:      map[string]SelectorStats{},
		interrupts: map[string]InterruptHandler{},
		repairs:    map[string]RepairPatch{},
		outbox:     map[string]OutboxEvent{},
	}
	if err := repo.load(); err != nil {
		return nil, err
	}
	return repo, nil
}

func (repo *FileRepository) SaveCandidate(_ context.Context, candidate WorkflowCandidate) error {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	if candidate.ID == "" {
		candidate.ID = newID("cand")
	}
	repo.candidates[candidate.ID] = candidate
	return repo.persistLocked()
}

func (repo *FileRepository) GetCandidate(_ context.Context, id string) (WorkflowCandidate, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	candidate, ok := repo.candidates[id]
	if !ok {
		return WorkflowCandidate{}, errors.New("workflow candidate not found")
	}
	return candidate, nil
}

func (repo *FileRepository) ListCandidates(_ context.Context, query CandidateListQuery) ([]WorkflowCandidate, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	result := make([]WorkflowCandidate, 0)
	for _, candidate := range repo.candidates {
		if query.ProjectID != "" && candidate.ProjectID != query.ProjectID {
			continue
		}
		if query.Source != "" && candidate.Source != query.Source {
			continue
		}
		if query.Status != "" && candidate.Status != query.Status {
			continue
		}
		result = append(result, candidate)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].CreatedAt.Equal(result[j].CreatedAt) {
			return result[i].ID < result[j].ID
		}
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	return result, nil
}

func (repo *FileRepository) SaveWorkflow(_ context.Context, recipe WorkflowRecipe) error {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	repo.workflows[recipe.ID] = recipe
	return repo.persistLocked()
}

func (repo *FileRepository) GetWorkflow(_ context.Context, id string) (WorkflowRecipe, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	recipe, ok := repo.workflows[id]
	if !ok {
		return WorkflowRecipe{}, errors.New("workflow not found")
	}
	return recipe, nil
}

func (repo *FileRepository) ListActiveWorkflows(_ context.Context, projectID string) ([]WorkflowRecipe, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	result := make([]WorkflowRecipe, 0)
	for _, recipe := range repo.workflows {
		if projectID != "" && recipe.ProjectID != projectID {
			continue
		}
		if recipe.Status != StatusActive || !recipe.Searchable {
			continue
		}
		result = append(result, recipe)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result, nil
}

func (repo *FileRepository) SaveVersion(_ context.Context, version WorkflowVersion) error {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	repo.versions[versionKey(version.WorkflowID, version.Version)] = version
	return repo.persistLocked()
}

func (repo *FileRepository) GetVersion(_ context.Context, workflowID string, version int) (WorkflowVersion, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	value, ok := repo.versions[versionKey(workflowID, version)]
	if !ok {
		return WorkflowVersion{}, errors.New("workflow version not found")
	}
	return value, nil
}

func (repo *FileRepository) SaveRun(_ context.Context, run WorkflowRun) error {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	if run.ID == "" {
		run.ID = newID("run")
	}
	repo.runs[run.ID] = run
	return repo.persistLocked()
}

func (repo *FileRepository) GetRun(_ context.Context, id string) (WorkflowRun, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	run, ok := repo.runs[id]
	if !ok {
		return WorkflowRun{}, errors.New("workflow run not found")
	}
	return run, nil
}

func (repo *FileRepository) SaveSelectorStats(_ context.Context, stats SelectorStats) error {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	repo.stats[selectorStatsKey(stats)] = stats
	return repo.persistLocked()
}

func (repo *FileRepository) ListSelectorStats(_ context.Context, workflowID string, version int) ([]SelectorStats, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	result := make([]SelectorStats, 0)
	for _, stats := range repo.stats {
		if workflowID != "" && stats.WorkflowID != workflowID {
			continue
		}
		if version != 0 && stats.Version != version {
			continue
		}
		result = append(result, stats)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].StepID == result[j].StepID {
			return result[i].Selector < result[j].Selector
		}
		return result[i].StepID < result[j].StepID
	})
	return result, nil
}

func (repo *FileRepository) SaveInterruptHandler(_ context.Context, handler InterruptHandler) error {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	if handler.ID == "" {
		handler.ID = newID("ih")
	}
	repo.interrupts[handler.ID] = handler
	return repo.persistLocked()
}

func (repo *FileRepository) ListActiveInterruptHandlers(_ context.Context, projectID string) ([]InterruptHandler, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	result := make([]InterruptHandler, 0)
	for _, handler := range repo.interrupts {
		if projectID != "" && handler.ProjectID != projectID {
			continue
		}
		if handler.Status != StatusActive {
			continue
		}
		result = append(result, handler)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result, nil
}

func (repo *FileRepository) SaveRepairPatch(_ context.Context, patch RepairPatch) error {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	if patch.ID == "" {
		patch.ID = newID("patch")
	}
	repo.repairs[patch.ID] = patch
	return repo.persistLocked()
}

func (repo *FileRepository) GetRepairPatch(_ context.Context, id string) (RepairPatch, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	patch, ok := repo.repairs[id]
	if !ok {
		return RepairPatch{}, errors.New("repair patch not found")
	}
	return patch, nil
}

func (repo *FileRepository) ListRepairPatches(_ context.Context, query RepairPatchListQuery) ([]RepairPatch, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	result := make([]RepairPatch, 0)
	for _, patch := range repo.repairs {
		if query.ProjectID != "" && patch.ProjectID != query.ProjectID {
			continue
		}
		if query.WorkflowID != "" && patch.WorkflowID != query.WorkflowID {
			continue
		}
		if query.Status != "" && patch.Status != query.Status {
			continue
		}
		result = append(result, patch)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].CreatedAt.Equal(result[j].CreatedAt) {
			return result[i].ID < result[j].ID
		}
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	return result, nil
}

func (repo *FileRepository) ListActiveRepairPatches(_ context.Context, projectID string) ([]RepairPatch, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	result := make([]RepairPatch, 0)
	for _, patch := range repo.repairs {
		if projectID != "" && patch.ProjectID != projectID {
			continue
		}
		if patch.Status != StatusActive {
			continue
		}
		result = append(result, patch)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result, nil
}

func (repo *FileRepository) AppendOutboxEvent(_ context.Context, event OutboxEvent) (OutboxEvent, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	for _, existing := range repo.outbox {
		if event.IdempotencyKey != "" && existing.IdempotencyKey == event.IdempotencyKey {
			return existing, nil
		}
	}
	if event.ID == "" {
		event.ID = newID("evt")
	}
	if event.Status == "" {
		event.Status = StatusPendingReview
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}
	repo.outbox[event.ID] = event
	return event, repo.persistLocked()
}

func (repo *FileRepository) ListOutboxEvents(_ context.Context, status Status) ([]OutboxEvent, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	result := make([]OutboxEvent, 0)
	for _, event := range repo.outbox {
		if status != "" && event.Status != status {
			continue
		}
		result = append(result, event)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].CreatedAt.Equal(result[j].CreatedAt) {
			return result[i].ID < result[j].ID
		}
		return result[i].CreatedAt.Before(result[j].CreatedAt)
	})
	return result, nil
}

func (repo *FileRepository) load() error {
	if err := readJSON(repo.path("candidates.json"), &repo.candidates); err != nil {
		return err
	}
	if err := readJSON(repo.path("workflows.json"), &repo.workflows); err != nil {
		return err
	}
	if err := readJSON(repo.path("versions.json"), &repo.versions); err != nil {
		return err
	}
	if err := readJSON(repo.path("runs.json"), &repo.runs); err != nil {
		return err
	}
	if err := readJSON(repo.path("selector_stats.json"), &repo.stats); err != nil {
		return err
	}
	if err := readJSON(repo.path("interrupt_handlers.json"), &repo.interrupts); err != nil {
		return err
	}
	if err := readJSON(repo.path("repair_patches.json"), &repo.repairs); err != nil {
		return err
	}
	return readJSON(repo.path("outbox.json"), &repo.outbox)
}

func (repo *FileRepository) persistLocked() error {
	if err := writeJSON(repo.path("candidates.json"), repo.candidates); err != nil {
		return err
	}
	if err := writeJSON(repo.path("workflows.json"), repo.workflows); err != nil {
		return err
	}
	if err := writeJSON(repo.path("versions.json"), repo.versions); err != nil {
		return err
	}
	if err := writeJSON(repo.path("runs.json"), repo.runs); err != nil {
		return err
	}
	if err := writeJSON(repo.path("selector_stats.json"), repo.stats); err != nil {
		return err
	}
	if err := writeJSON(repo.path("interrupt_handlers.json"), repo.interrupts); err != nil {
		return err
	}
	if err := writeJSON(repo.path("repair_patches.json"), repo.repairs); err != nil {
		return err
	}
	return writeJSON(repo.path("outbox.json"), repo.outbox)
}

func (repo *FileRepository) path(name string) string {
	return filepath.Join(repo.dir, name)
}

func readJSON(path string, target any) error {
	content, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	return json.Unmarshal(content, target)
}

func writeJSON(path string, value any) error {
	content, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, content, 0o644)
}

func versionKey(workflowID string, version int) string {
	return workflowID + ":v" + strconv.Itoa(version)
}

func selectorStatsKey(stats SelectorStats) string {
	return versionKey(stats.WorkflowID, stats.Version) + ":" + stats.StepID + ":" + stats.Strategy + ":" + stats.Selector
}

func newID(prefix string) string {
	var bytes [8]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return prefix + "_" + time.Now().UTC().Format("20060102150405.000000000")
	}
	return prefix + "_" + hex.EncodeToString(bytes[:])
}
