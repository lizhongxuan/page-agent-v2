package registry

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type FileRepository struct {
	mu                 sync.Mutex
	dir                string
	candidates         map[string]WorkflowCandidate
	workflows          map[string]WorkflowRecipe
	versions           map[string]WorkflowVersion
	runs               map[string]WorkflowRun
	taskRuns           map[string]TaskRun
	pageStates         map[string]PageState
	pageSurfaces       map[string]PageSurface
	transitions        map[string]PageTransition
	knowledgeDocuments map[string]KnowledgeDocument
	knowledgeChunks    map[string]KnowledgeChunk
	businessProfiles   map[string]BusinessSystemProfile
	pageObservations   map[string]PageObservationEvent
	experiences        map[string]ExperienceMemory
	failures           map[string]FailureMemory
	memoryContexts     map[string]MemoryContextEvent
	attributionEvents  map[string]MemoryAttributionEvent
	evidenceStats      map[string]MemoryEvidenceStats
	memoryReviews      map[string]MemoryReview
	stats              map[string]SelectorStats
	interrupts         map[string]InterruptHandler
	repairs            map[string]RepairPatch
	outbox             map[string]OutboxEvent
}

func NewFileRepository(dir string) (*FileRepository, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	repo := &FileRepository{
		dir:                dir,
		candidates:         map[string]WorkflowCandidate{},
		workflows:          map[string]WorkflowRecipe{},
		versions:           map[string]WorkflowVersion{},
		runs:               map[string]WorkflowRun{},
		taskRuns:           map[string]TaskRun{},
		pageStates:         map[string]PageState{},
		pageSurfaces:       map[string]PageSurface{},
		transitions:        map[string]PageTransition{},
		knowledgeDocuments: map[string]KnowledgeDocument{},
		knowledgeChunks:    map[string]KnowledgeChunk{},
		businessProfiles:   map[string]BusinessSystemProfile{},
		pageObservations:   map[string]PageObservationEvent{},
		experiences:        map[string]ExperienceMemory{},
		failures:           map[string]FailureMemory{},
		memoryContexts:     map[string]MemoryContextEvent{},
		attributionEvents:  map[string]MemoryAttributionEvent{},
		evidenceStats:      map[string]MemoryEvidenceStats{},
		memoryReviews:      map[string]MemoryReview{},
		stats:              map[string]SelectorStats{},
		interrupts:         map[string]InterruptHandler{},
		repairs:            map[string]RepairPatch{},
		outbox:             map[string]OutboxEvent{},
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

func (repo *FileRepository) ListVersions(_ context.Context, workflowID string) ([]WorkflowVersion, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	result := make([]WorkflowVersion, 0)
	for _, version := range repo.versions {
		if workflowID != "" && version.WorkflowID != workflowID {
			continue
		}
		result = append(result, version)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].WorkflowID == result[j].WorkflowID {
			return result[i].Version < result[j].Version
		}
		return result[i].WorkflowID < result[j].WorkflowID
	})
	return result, nil
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

func (repo *FileRepository) SaveTaskRun(_ context.Context, run TaskRun) error {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	if run.ID == "" {
		run.ID = newID("task_run")
	}
	if run.CreatedAt.IsZero() {
		run.CreatedAt = time.Now().UTC()
	}
	if err := ValidateSummaryLength(run.Summary); err != nil {
		return err
	}
	for index := range run.ActionSteps {
		if run.ActionSteps[index].ID == "" {
			run.ActionSteps[index].ID = newID("step")
		}
		if err := ValidateSummaryLength(run.ActionSteps[index].ReasoningSummary); err != nil {
			return err
		}
		if err := ValidateSummaryLength(run.ActionSteps[index].ResultSummary); err != nil {
			return err
		}
		run.ActionSteps[index].TaskRunID = run.ID
		if run.ActionSteps[index].CreatedAt.IsZero() {
			run.ActionSteps[index].CreatedAt = run.CreatedAt
		}
	}
	repo.taskRuns[run.ID] = run
	return repo.persistLocked()
}

func (repo *FileRepository) GetTaskRun(_ context.Context, id string) (TaskRun, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	run, ok := repo.taskRuns[id]
	if !ok {
		return TaskRun{}, errors.New("task run not found")
	}
	return run, nil
}

func (repo *FileRepository) ListTaskRuns(_ context.Context, query TaskRunListQuery) ([]TaskRun, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	result := make([]TaskRun, 0)
	for _, run := range repo.taskRuns {
		if query.ProjectID != "" && run.ProjectID != query.ProjectID {
			continue
		}
		if query.Site != "" && run.Site != query.Site {
			continue
		}
		if query.Status != "" && run.Status != query.Status {
			continue
		}
		result = append(result, run)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].CreatedAt.Equal(result[j].CreatedAt) {
			return result[i].ID < result[j].ID
		}
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	return result, nil
}

func (repo *FileRepository) SavePageState(_ context.Context, state PageState) error {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	if state.ID == "" {
		state.ID = newID("page")
	}
	if state.UpdatedAt.IsZero() {
		state.UpdatedAt = time.Now().UTC()
	}
	repo.pageStates[state.ID] = state
	return repo.persistLocked()
}

func (repo *FileRepository) GetPageState(_ context.Context, id string) (PageState, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	state, ok := repo.pageStates[id]
	if !ok {
		return PageState{}, errors.New("page state not found")
	}
	return state, nil
}

func (repo *FileRepository) ListPageStates(_ context.Context, query PageStateListQuery) ([]PageState, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	result := make([]PageState, 0)
	for _, state := range repo.pageStates {
		if query.ProjectID != "" && state.ProjectID != query.ProjectID {
			continue
		}
		if query.Site != "" && state.Site != query.Site {
			continue
		}
		result = append(result, state)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result, nil
}

func (repo *FileRepository) SavePageSurface(_ context.Context, surface PageSurface) error {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	if surface.ID == "" {
		surface.ID = newID("surface")
	}
	now := time.Now().UTC()
	if surface.FirstSeenAt.IsZero() {
		if existing, ok := repo.pageSurfaces[surface.ID]; ok && !existing.FirstSeenAt.IsZero() {
			surface.FirstSeenAt = existing.FirstSeenAt
		} else {
			surface.FirstSeenAt = now
		}
	}
	if surface.LastSeenAt.IsZero() {
		surface.LastSeenAt = now
	}
	if existing, ok := repo.pageSurfaces[surface.ID]; ok && surface.ObservationCount <= 0 {
		surface.ObservationCount = existing.ObservationCount + 1
	}
	if surface.ObservationCount <= 0 {
		surface.ObservationCount = 1
	}
	if surface.Status == "" {
		surface.Status = StatusActive
	}
	repo.pageSurfaces[surface.ID] = surface
	return repo.persistLocked()
}

func (repo *FileRepository) ListPageSurfaces(_ context.Context, query PageSurfaceListQuery) ([]PageSurface, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	result := make([]PageSurface, 0)
	for _, surface := range repo.pageSurfaces {
		if query.ProjectID != "" && surface.ProjectID != query.ProjectID {
			continue
		}
		if query.Site != "" && surface.Site != query.Site {
			continue
		}
		if query.ParentPageStateID != "" && surface.ParentPageStateID != query.ParentPageStateID {
			continue
		}
		if query.SurfaceType != "" && surface.SurfaceType != query.SurfaceType {
			continue
		}
		result = append(result, surface)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result, nil
}

func (repo *FileRepository) SavePageTransition(_ context.Context, transition PageTransition) error {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	if transition.ID == "" {
		transition.ID = newID("transition")
	}
	repo.transitions[transition.ID] = transition
	return repo.persistLocked()
}

func (repo *FileRepository) ListPageTransitions(_ context.Context, query PageTransitionListQuery) ([]PageTransition, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	result := make([]PageTransition, 0)
	for _, transition := range repo.transitions {
		if query.ProjectID != "" && transition.ProjectID != query.ProjectID {
			continue
		}
		if query.Site != "" && transition.Site != query.Site {
			continue
		}
		if query.FromPageStateID != "" && transition.FromPageState != query.FromPageStateID {
			continue
		}
		if query.ToPageStateID != "" && transition.ToPageState != query.ToPageStateID {
			continue
		}
		result = append(result, transition)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result, nil
}

func (repo *FileRepository) SaveKnowledgeDocument(_ context.Context, document KnowledgeDocument) error {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	if document.ID == "" {
		document.ID = newID("doc")
	}
	if document.CreatedAt.IsZero() {
		document.CreatedAt = time.Now().UTC()
	}
	repo.knowledgeDocuments[document.ID] = document
	return repo.persistLocked()
}

func (repo *FileRepository) SaveKnowledgeChunks(_ context.Context, chunks []KnowledgeChunk) error {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	now := time.Now().UTC()
	documentIDs := map[string]bool{}
	chunkIDs := map[string]bool{}
	for index := range chunks {
		if chunks[index].DocumentID != "" {
			documentIDs[chunks[index].DocumentID] = true
		}
		if chunks[index].ID == "" {
			chunks[index].ID = newID("chunk")
		}
		chunkIDs[chunks[index].ID] = true
	}
	for id, existing := range repo.knowledgeChunks {
		if documentIDs[existing.DocumentID] && !chunkIDs[id] {
			delete(repo.knowledgeChunks, id)
		}
	}
	for _, chunk := range chunks {
		if chunk.ID == "" {
			chunk.ID = newID("chunk")
		}
		if chunk.UpdatedAt.IsZero() {
			chunk.UpdatedAt = now
		}
		repo.knowledgeChunks[chunk.ID] = chunk
	}
	return repo.persistLocked()
}

func (repo *FileRepository) SearchKnowledgeChunks(_ context.Context, query KnowledgeSearchQuery) ([]KnowledgeChunk, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	limit := query.Limit
	if limit <= 0 {
		limit = 3
	}
	queryText := strings.ToLower(strings.Join([]string{
		query.Task,
		query.Title,
		query.URL,
		query.VisibleText,
		strings.Join(query.Hints, " "),
	}, " "))
	result := make([]KnowledgeChunk, 0)
	for _, chunk := range repo.knowledgeChunks {
		if query.ProjectID != "" && chunk.ProjectID != query.ProjectID {
			continue
		}
		if !knowledgeChunkHardGateMatches(chunk, query) {
			continue
		}
		score := scoreKnowledgeChunk(chunk, queryText)
		if len(query.Embedding) > 0 && len(chunk.Embedding) > 0 {
			if vectorScore := cosineSimilarity(query.Embedding, chunk.Embedding); vectorScore > score {
				score = vectorScore
			}
		}
		if score <= 0 {
			continue
		}
		chunk.Score = score
		result = append(result, chunk)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Score == result[j].Score {
			return result[i].ID < result[j].ID
		}
		return result[i].Score > result[j].Score
	})
	if len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

func (repo *FileRepository) SaveBusinessSystemProfile(_ context.Context, profile BusinessSystemProfile) error {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	if profile.UpdatedAt.IsZero() {
		profile.UpdatedAt = time.Now().UTC()
	}
	if profile.Status == "" {
		profile.Status = StatusActive
	}
	if profile.SourceType == "" {
		profile.SourceType = MemorySourceProduction
	}
	if err := ValidateMemoryRecord(profile); err != nil {
		return err
	}
	repo.businessProfiles[businessProfileKey(profile.ProjectID, profile.Site, profile.Module)] = profile
	return repo.persistLocked()
}

func (repo *FileRepository) GetBusinessSystemProfile(_ context.Context, query BusinessSystemProfileQuery) (BusinessSystemProfile, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	candidates := []BusinessSystemProfile{}
	for _, profile := range repo.businessProfiles {
		if query.ProjectID != "" && profile.ProjectID != query.ProjectID {
			continue
		}
		if !profileSourceAllowed(profile.SourceType, query.SourceType) {
			continue
		}
		if query.Site != "" && profile.Site != "" && profile.Site != query.Site {
			continue
		}
		candidates = append(candidates, profile)
	}
	sort.Slice(candidates, func(i, j int) bool {
		return businessProfileRank(candidates[i], query) > businessProfileRank(candidates[j], query)
	})
	for _, profile := range candidates {
		if businessProfileRank(profile, query) > 0 {
			return profile, nil
		}
	}
	return BusinessSystemProfile{}, errors.New("business system profile not found")
}

func (repo *FileRepository) SavePageObservationEvent(_ context.Context, event PageObservationEvent) error {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	now := time.Now().UTC()
	if event.ID == "" {
		event.ID = newID("obs")
	}
	if existing, ok := repo.pageObservations[event.ID]; ok {
		if event.CreatedAt.IsZero() {
			event.CreatedAt = existing.CreatedAt
		}
		if event.SeenCount <= 0 {
			event.SeenCount = existing.SeenCount + 1
		}
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = now
	}
	if event.LastSeenAt.IsZero() {
		event.LastSeenAt = now
	}
	if event.SeenCount <= 0 {
		event.SeenCount = 1
	}
	if err := ValidateSummaryLength(event.VisibleTextSample); err != nil {
		return err
	}
	if containsSensitiveText(event.VisibleTextSample) {
		return errors.New("page observation visible text contains sensitive text")
	}
	repo.pageObservations[event.ID] = event
	return repo.persistLocked()
}

func (repo *FileRepository) ListPageObservationEvents(_ context.Context, query PageObservationEventListQuery) ([]PageObservationEvent, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	result := make([]PageObservationEvent, 0)
	for _, event := range repo.pageObservations {
		if query.ProjectID != "" && event.ProjectID != query.ProjectID {
			continue
		}
		if query.Site != "" && event.Site != query.Site {
			continue
		}
		if query.URLPattern != "" && event.URLPattern != query.URLPattern {
			continue
		}
		result = append(result, event)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].CreatedAt.Equal(result[j].CreatedAt) {
			return result[i].ID < result[j].ID
		}
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	return result, nil
}

func (repo *FileRepository) SaveExperienceMemory(_ context.Context, memory ExperienceMemory) error {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	if memory.ID == "" {
		memory.ID = newID("exp")
	}
	now := time.Now().UTC()
	if memory.CreatedAt.IsZero() {
		memory.CreatedAt = now
	}
	if memory.UpdatedAt.IsZero() {
		memory.UpdatedAt = now
	}
	if memory.ReviewStatus == "" {
		memory.ReviewStatus = ReviewStatusPending
	}
	if err := ValidateMemoryRecord(memory); err != nil {
		return err
	}
	repo.experiences[memory.ID] = memory
	return repo.persistLocked()
}

func (repo *FileRepository) GetExperienceMemory(_ context.Context, id string) (ExperienceMemory, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	memory, ok := repo.experiences[id]
	if !ok {
		return ExperienceMemory{}, errors.New("experience memory not found")
	}
	return memory, nil
}

func (repo *FileRepository) SearchExperienceMemories(ctx context.Context, query ExperienceMemorySearchQuery) ([]ExperienceMemory, error) {
	if query.SearchableOnly == false {
		query.SearchableOnly = true
	}
	return repo.ListExperienceMemories(ctx, query)
}

func (repo *FileRepository) ListExperienceMemories(_ context.Context, query ExperienceMemorySearchQuery) ([]ExperienceMemory, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	limit := query.Limit
	if limit <= 0 {
		limit = math.MaxInt
	}
	queryText := strings.ToLower(query.Task)
	result := make([]ExperienceMemory, 0)
	for _, memory := range repo.experiences {
		if query.ProjectID != "" && memory.ProjectID != query.ProjectID {
			continue
		}
		if query.Site != "" && memory.Site != query.Site {
			continue
		}
		if query.Status != "" && memory.ReviewStatus != query.Status {
			continue
		}
		if query.SearchableOnly && !memory.Searchable {
			continue
		}
		if query.StartPageState != "" && memory.StartPageState != query.StartPageState {
			continue
		}
		score := scoreExperienceMemory(memory, queryText)
		if len(query.Embedding) > 0 && len(memory.Embedding) > 0 {
			if vectorScore := cosineSimilarity(query.Embedding, memory.Embedding); vectorScore > score {
				score = vectorScore
			}
		}
		if query.Task != "" && score <= 0 {
			continue
		}
		memory.Score = score
		result = append(result, memory)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Score == result[j].Score {
			if result[i].SuccessCount == result[j].SuccessCount {
				return result[i].ID < result[j].ID
			}
			return result[i].SuccessCount > result[j].SuccessCount
		}
		return result[i].Score > result[j].Score
	})
	if len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

func (repo *FileRepository) SaveFailureMemory(_ context.Context, memory FailureMemory) error {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	now := time.Now().UTC()
	if memory.ID == "" {
		memory.ID = newID("fail")
	}
	if existing, ok := repo.failures[memory.ID]; ok {
		if memory.CreatedAt.IsZero() {
			memory.CreatedAt = existing.CreatedAt
		}
		if memory.OccurrenceCount <= 0 {
			memory.OccurrenceCount = existing.OccurrenceCount + 1
		}
	}
	if memory.CreatedAt.IsZero() {
		memory.CreatedAt = now
	}
	if memory.LastSeenAt.IsZero() {
		memory.LastSeenAt = now
	}
	if memory.OccurrenceCount <= 0 {
		memory.OccurrenceCount = 1
	}
	if err := ValidateMemoryRecord(memory); err != nil {
		return err
	}
	repo.failures[memory.ID] = memory
	return repo.persistLocked()
}

func (repo *FileRepository) SearchFailureMemories(ctx context.Context, query FailureMemorySearchQuery) ([]FailureMemory, error) {
	return repo.ListFailureMemories(ctx, query)
}

func (repo *FileRepository) ListFailureMemories(_ context.Context, query FailureMemorySearchQuery) ([]FailureMemory, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	limit := query.Limit
	if limit <= 0 {
		limit = math.MaxInt
	}
	queryText := strings.ToLower(query.Task)
	result := make([]FailureMemory, 0)
	for _, memory := range repo.failures {
		if query.ProjectID != "" && memory.ProjectID != query.ProjectID {
			continue
		}
		if query.Site != "" && memory.Site != query.Site {
			continue
		}
		if query.PageStateID != "" && memory.PageStateID != query.PageStateID {
			continue
		}
		score := scoreFailureMemory(memory, queryText)
		if query.Task != "" && score <= 0 {
			continue
		}
		memory.Score = score
		result = append(result, memory)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Score == result[j].Score {
			if result[i].CreatedAt.Equal(result[j].CreatedAt) {
				return result[i].ID < result[j].ID
			}
			return result[i].CreatedAt.After(result[j].CreatedAt)
		}
		return result[i].Score > result[j].Score
	})
	if len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

func (repo *FileRepository) SaveMemoryContextEvent(_ context.Context, event MemoryContextEvent) error {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	if event.ID == "" {
		event.ID = newID("ctx")
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}
	repo.memoryContexts[event.ID] = event
	return repo.persistLocked()
}

func (repo *FileRepository) GetMemoryContextEvent(_ context.Context, id string) (MemoryContextEvent, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	event, ok := repo.memoryContexts[id]
	if !ok {
		return MemoryContextEvent{}, errors.New("memory context event not found")
	}
	return event, nil
}

func (repo *FileRepository) SaveMemoryAttributionEvent(_ context.Context, event MemoryAttributionEvent) error {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	if event.ID == "" {
		event.ID = newID("attr")
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}
	if err := ValidateSummaryLength(event.Reason); err != nil {
		return err
	}
	repo.attributionEvents[event.ID] = event
	return repo.persistLocked()
}

func (repo *FileRepository) ListMemoryAttributionEvents(_ context.Context, query MemoryAttributionEventListQuery) ([]MemoryAttributionEvent, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	result := make([]MemoryAttributionEvent, 0)
	for _, event := range repo.attributionEvents {
		if query.ProjectID != "" && event.ProjectID != query.ProjectID {
			continue
		}
		if query.Site != "" && event.Site != query.Site {
			continue
		}
		if query.TaskRunID != "" && event.TaskRunID != query.TaskRunID {
			continue
		}
		if query.ContextID != "" && event.ContextID != query.ContextID {
			continue
		}
		if query.EvidenceID != "" && event.EvidenceID != query.EvidenceID {
			continue
		}
		if query.EvidenceSource != "" && event.EvidenceSource != query.EvidenceSource {
			continue
		}
		if query.Label != "" && event.Label != query.Label {
			continue
		}
		result = append(result, event)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].CreatedAt.Equal(result[j].CreatedAt) {
			return result[i].ID < result[j].ID
		}
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	return result, nil
}

func (repo *FileRepository) SaveMemoryEvidenceStats(_ context.Context, stats MemoryEvidenceStats) error {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	if stats.LastFeedbackAt.IsZero() {
		stats.LastFeedbackAt = time.Now().UTC()
	}
	repo.evidenceStats[memoryEvidenceStatsKey(stats.ProjectID, stats.EvidenceSource, stats.EvidenceID)] = stats
	return repo.persistLocked()
}

func (repo *FileRepository) GetMemoryEvidenceStats(_ context.Context, projectID string, source MemoryEvidenceSource, evidenceID string) (MemoryEvidenceStats, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	stats, ok := repo.evidenceStats[memoryEvidenceStatsKey(projectID, source, evidenceID)]
	if !ok {
		return MemoryEvidenceStats{}, errors.New("memory evidence stats not found")
	}
	return stats, nil
}

func (repo *FileRepository) ListMemoryEvidenceStats(_ context.Context, query MemoryEvidenceStatsListQuery) ([]MemoryEvidenceStats, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	result := make([]MemoryEvidenceStats, 0)
	for _, stats := range repo.evidenceStats {
		if query.ProjectID != "" && stats.ProjectID != query.ProjectID {
			continue
		}
		if query.Site != "" && stats.Site != query.Site {
			continue
		}
		if query.EvidenceID != "" && stats.EvidenceID != query.EvidenceID {
			continue
		}
		if query.EvidenceSource != "" && stats.EvidenceSource != query.EvidenceSource {
			continue
		}
		result = append(result, stats)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].UtilityScore == result[j].UtilityScore {
			if result[i].LastFeedbackAt.Equal(result[j].LastFeedbackAt) {
				if result[i].EvidenceSource == result[j].EvidenceSource {
					return result[i].EvidenceID < result[j].EvidenceID
				}
				return result[i].EvidenceSource < result[j].EvidenceSource
			}
			return result[i].LastFeedbackAt.After(result[j].LastFeedbackAt)
		}
		return result[i].UtilityScore > result[j].UtilityScore
	})
	return result, nil
}

func (repo *FileRepository) SaveMemoryReview(_ context.Context, review MemoryReview) error {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	if review.ID == "" {
		review.ID = newID("review")
	}
	if review.Status == "" {
		review.Status = ReviewStatusPending
	}
	if review.CreatedAt.IsZero() {
		review.CreatedAt = time.Now().UTC()
	}
	if err := ValidateMemoryRecord(review); err != nil {
		return err
	}
	repo.memoryReviews[review.ID] = review
	return repo.persistLocked()
}

func (repo *FileRepository) ListMemoryReviews(_ context.Context, query MemoryReviewListQuery) ([]MemoryReview, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	result := make([]MemoryReview, 0)
	for _, review := range repo.memoryReviews {
		if query.ProjectID != "" && review.ProjectID != query.ProjectID {
			continue
		}
		if query.TargetType != "" && review.TargetType != query.TargetType {
			continue
		}
		if query.Status != "" && review.Status != query.Status {
			continue
		}
		result = append(result, review)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].CreatedAt.Equal(result[j].CreatedAt) {
			return result[i].ID < result[j].ID
		}
		return result[i].CreatedAt.Before(result[j].CreatedAt)
	})
	return result, nil
}

func (repo *FileRepository) ApproveMemoryReview(_ context.Context, id string) error {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	review, ok := repo.memoryReviews[id]
	if !ok {
		return errors.New("memory review not found")
	}
	review.Status = ReviewStatusApproved
	review.ReviewedAt = time.Now().UTC()
	repo.memoryReviews[id] = review
	return repo.persistLocked()
}

func (repo *FileRepository) RejectMemoryReview(_ context.Context, id string) error {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	review, ok := repo.memoryReviews[id]
	if !ok {
		return errors.New("memory review not found")
	}
	review.Status = ReviewStatusRejected
	review.ReviewedAt = time.Now().UTC()
	repo.memoryReviews[id] = review
	return repo.persistLocked()
}

func (repo *FileRepository) RankActiveWorkflowsByEmbedding(_ context.Context, projectID string, workflowIDs []string, embedding []float32, limit int) ([]WorkflowEmbeddingHit, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	if len(workflowIDs) == 0 || len(embedding) == 0 {
		return nil, nil
	}
	if limit <= 0 {
		limit = 3
	}
	allowed := map[string]bool{}
	for _, workflowID := range workflowIDs {
		allowed[workflowID] = true
	}
	hits := []WorkflowEmbeddingHit{}
	for _, workflow := range repo.workflows {
		if !allowed[workflow.ID] {
			continue
		}
		if projectID != "" && workflow.ProjectID != projectID {
			continue
		}
		if workflow.Status != StatusActive || !workflow.Searchable || len(workflow.Embedding) == 0 {
			continue
		}
		score := cosineSimilarity(embedding, workflow.Embedding)
		if score <= 0 {
			continue
		}
		hits = append(hits, WorkflowEmbeddingHit{Workflow: workflow, Score: score})
	}
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].Score == hits[j].Score {
			return hits[i].Workflow.ID < hits[j].Workflow.ID
		}
		return hits[i].Score > hits[j].Score
	})
	if len(hits) > limit {
		hits = hits[:limit]
	}
	return hits, nil
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

func (repo *FileRepository) PruneMemory(_ context.Context, policy MemoryPrunePolicy) (MemoryPruneResult, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	result := MemoryPruneResult{}
	if policy.MaxTaskRuns > 0 {
		result.DeletedTaskRuns = repo.pruneTaskRunsLocked(policy)
	}
	if policy.MaxPageObservationEvents > 0 {
		result.DeletedPageObservationEvents = repo.prunePageObservationsLocked(policy)
	}
	if policy.MaxMemoryContextEvents > 0 {
		result.DeletedMemoryContextEvents = repo.pruneMemoryContextsLocked(policy)
	}
	if policy.MaxFailureMemories > 0 {
		result.DeletedFailureMemories = repo.pruneFailuresLocked(policy)
	}
	if result.DeletedTaskRuns == 0 &&
		result.DeletedPageObservationEvents == 0 &&
		result.DeletedMemoryContextEvents == 0 &&
		result.DeletedFailureMemories == 0 {
		return result, nil
	}
	return result, repo.persistLocked()
}

func (repo *FileRepository) pruneTaskRunsLocked(policy MemoryPrunePolicy) int {
	candidates := []pruneCandidate{}
	for id, run := range repo.taskRuns {
		if !memoryProjectAndSiteMatch(policy, run.ProjectID, run.Site) {
			continue
		}
		candidates = append(candidates, pruneCandidate{id: id, at: run.CreatedAt})
	}
	return pruneCandidates(candidates, policy.MaxTaskRuns, func(id string) {
		delete(repo.taskRuns, id)
	})
}

func (repo *FileRepository) prunePageObservationsLocked(policy MemoryPrunePolicy) int {
	candidates := []pruneCandidate{}
	for id, event := range repo.pageObservations {
		if !memoryProjectAndSiteMatch(policy, event.ProjectID, event.Site) {
			continue
		}
		candidates = append(candidates, pruneCandidate{id: id, at: nonZeroTime(event.LastSeenAt, event.CreatedAt)})
	}
	return pruneCandidates(candidates, policy.MaxPageObservationEvents, func(id string) {
		delete(repo.pageObservations, id)
	})
}

func (repo *FileRepository) pruneMemoryContextsLocked(policy MemoryPrunePolicy) int {
	candidates := []pruneCandidate{}
	for id, event := range repo.memoryContexts {
		if policy.ProjectID != "" && event.ProjectID != policy.ProjectID {
			continue
		}
		candidates = append(candidates, pruneCandidate{id: id, at: event.CreatedAt})
	}
	return pruneCandidates(candidates, policy.MaxMemoryContextEvents, func(id string) {
		delete(repo.memoryContexts, id)
	})
}

func (repo *FileRepository) pruneFailuresLocked(policy MemoryPrunePolicy) int {
	candidates := []pruneCandidate{}
	for id, failure := range repo.failures {
		if !memoryProjectAndSiteMatch(policy, failure.ProjectID, failure.Site) {
			continue
		}
		candidates = append(candidates, pruneCandidate{id: id, at: nonZeroTime(failure.LastSeenAt, failure.CreatedAt)})
	}
	return pruneCandidates(candidates, policy.MaxFailureMemories, func(id string) {
		delete(repo.failures, id)
	})
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
	if err := readJSON(repo.path("task_runs.json"), &repo.taskRuns); err != nil {
		return err
	}
	if err := readJSON(repo.path("page_states.json"), &repo.pageStates); err != nil {
		return err
	}
	if err := readJSON(repo.path("page_surfaces.json"), &repo.pageSurfaces); err != nil {
		return err
	}
	if err := readJSON(repo.path("page_transitions.json"), &repo.transitions); err != nil {
		return err
	}
	if err := readJSON(repo.path("knowledge_documents.json"), &repo.knowledgeDocuments); err != nil {
		return err
	}
	if err := readJSON(repo.path("knowledge_chunks.json"), &repo.knowledgeChunks); err != nil {
		return err
	}
	if err := readJSON(repo.path("business_system_profiles.json"), &repo.businessProfiles); err != nil {
		return err
	}
	if err := readJSON(repo.path("page_observation_events.json"), &repo.pageObservations); err != nil {
		return err
	}
	if err := readJSON(repo.path("experience_memories.json"), &repo.experiences); err != nil {
		return err
	}
	if err := readJSON(repo.path("failure_memories.json"), &repo.failures); err != nil {
		return err
	}
	if err := readJSON(repo.path("memory_context_events.json"), &repo.memoryContexts); err != nil {
		return err
	}
	if err := readJSON(repo.path("memory_attribution_events.json"), &repo.attributionEvents); err != nil {
		return err
	}
	if err := readJSON(repo.path("memory_evidence_stats.json"), &repo.evidenceStats); err != nil {
		return err
	}
	if err := readJSON(repo.path("memory_reviews.json"), &repo.memoryReviews); err != nil {
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
	if err := writeJSON(repo.path("task_runs.json"), repo.taskRuns); err != nil {
		return err
	}
	if err := writeJSON(repo.path("page_states.json"), repo.pageStates); err != nil {
		return err
	}
	if err := writeJSON(repo.path("page_surfaces.json"), repo.pageSurfaces); err != nil {
		return err
	}
	if err := writeJSON(repo.path("page_transitions.json"), repo.transitions); err != nil {
		return err
	}
	if err := writeJSON(repo.path("knowledge_documents.json"), repo.knowledgeDocuments); err != nil {
		return err
	}
	if err := writeJSON(repo.path("knowledge_chunks.json"), repo.knowledgeChunks); err != nil {
		return err
	}
	if err := writeJSON(repo.path("business_system_profiles.json"), repo.businessProfiles); err != nil {
		return err
	}
	if err := writeJSON(repo.path("page_observation_events.json"), repo.pageObservations); err != nil {
		return err
	}
	if err := writeJSON(repo.path("experience_memories.json"), repo.experiences); err != nil {
		return err
	}
	if err := writeJSON(repo.path("failure_memories.json"), repo.failures); err != nil {
		return err
	}
	if err := writeJSON(repo.path("memory_context_events.json"), repo.memoryContexts); err != nil {
		return err
	}
	if err := writeJSON(repo.path("memory_attribution_events.json"), repo.attributionEvents); err != nil {
		return err
	}
	if err := writeJSON(repo.path("memory_evidence_stats.json"), repo.evidenceStats); err != nil {
		return err
	}
	if err := writeJSON(repo.path("memory_reviews.json"), repo.memoryReviews); err != nil {
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

func memoryEvidenceStatsKey(projectID string, source MemoryEvidenceSource, evidenceID string) string {
	return strings.TrimSpace(projectID) + ":" + string(source) + ":" + strings.TrimSpace(evidenceID)
}

func businessProfileKey(projectID, site, module string) string {
	return strings.Join([]string{
		strings.TrimSpace(projectID),
		strings.TrimSpace(site),
		strings.TrimSpace(module),
	}, "\x00")
}

func profileSourceAllowed(profileSource MemorySourceType, requested MemorySourceType) bool {
	if profileSource == "" {
		profileSource = MemorySourceProduction
	}
	if requested != "" {
		return profileSource == requested
	}
	return profileSource == MemorySourceProduction
}

func businessProfileRank(profile BusinessSystemProfile, query BusinessSystemProfileQuery) int {
	if query.ProjectID != "" && profile.ProjectID != query.ProjectID {
		return 0
	}
	score := 1
	if query.Site != "" {
		if profile.Site == query.Site {
			score += 8
		} else if profile.Site != "" {
			return 0
		}
	}
	if query.Module != "" {
		if strings.EqualFold(profile.Module, query.Module) {
			score += 12
		} else if profile.Module != "" {
			return 0
		}
	} else if profile.Module != "" {
		return 0
	}
	if profile.Status == StatusDisabled {
		return 0
	}
	return score
}

func scoreKnowledgeChunk(chunk KnowledgeChunk, queryText string) float64 {
	searchText := strings.ToLower(strings.Join([]string{
		chunk.Title,
		chunk.Source,
		chunk.ChunkText,
		strings.Join(chunk.Tags, " "),
	}, " "))
	score := 0.0
	for _, term := range strings.Fields(queryText) {
		term = strings.Trim(term, " \t\n\r,.，。:：;；/\\")
		if len([]rune(term)) < 2 {
			continue
		}
		if strings.Contains(searchText, term) {
			score += 1
		}
	}
	if score == 0 && queryText != "" {
		for _, r := range queryText {
			if r > 127 && strings.ContainsRune(searchText, r) {
				score += 0.2
			}
		}
	}
	return score
}

func scoreExperienceMemory(memory ExperienceMemory, queryText string) float64 {
	searchText := strings.ToLower(memory.SearchableText())
	score := 0.0
	for _, term := range strings.Fields(queryText) {
		term = strings.Trim(term, " \t\n\r,.，。:：;；/\\")
		if len([]rune(term)) < 2 {
			continue
		}
		if strings.Contains(searchText, strings.ToLower(term)) {
			score += 1
		}
	}
	if score == 0 && queryText != "" {
		for _, r := range queryText {
			if r > 127 && strings.ContainsRune(searchText, r) {
				score += 0.2
			}
		}
	}
	score += math.Min(float64(memory.SuccessCount), 10) * 0.05
	score -= math.Min(float64(memory.FailureCount), 10) * 0.03
	return score
}

func scoreFailureMemory(memory FailureMemory, queryText string) float64 {
	searchText := strings.ToLower(strings.Join([]string{
		memory.PageStateID,
		memory.ActionName,
		memory.FailureType,
		memory.FailureSummary,
		memory.AvoidHint,
	}, "\n"))
	score := 0.0
	for _, term := range strings.Fields(queryText) {
		term = strings.Trim(term, " \t\n\r,.，。:：;；/\\")
		if len([]rune(term)) < 2 {
			continue
		}
		if strings.Contains(searchText, strings.ToLower(term)) {
			score += 1
		}
	}
	if score == 0 && queryText != "" {
		for _, r := range queryText {
			if r > 127 && strings.ContainsRune(searchText, r) {
				score += 0.2
			}
		}
	}
	return score
}

func knowledgeChunkHardGateMatches(chunk KnowledgeChunk, query KnowledgeSearchQuery) bool {
	sourceType := MemorySourceType(metadataString(chunk.Metadata, "sourceType"))
	if sourceType == "" {
		sourceType = MemorySourceProduction
	}
	if sourceType != MemorySourceProduction {
		return false
	}
	if query.Module != "" {
		module := metadataString(chunk.Metadata, "module")
		if module != "" && !strings.EqualFold(module, query.Module) {
			return false
		}
	} else if (query.Site != "" || query.URL != "") && metadataString(chunk.Metadata, "module") != "" {
		return false
	}
	if query.Site != "" {
		site := metadataString(chunk.Metadata, "site")
		if site != "" && site != query.Site {
			return false
		}
	}
	return knowledgeChunkScopeMatches(chunk, query.URL)
}

func knowledgeChunkScopeMatches(chunk KnowledgeChunk, queryURL string) bool {
	docURL := metadataString(chunk.Metadata, "url")
	if docURL == "" {
		docURL = metadataString(chunk.Metadata, "urlPattern")
	}
	if docURL == "" {
		return strings.TrimSpace(queryURL) == ""
	}
	if strings.TrimSpace(queryURL) == "" {
		return true
	}
	docURL = normalizeScopeURL(docURL)
	queryURL = normalizeScopeURL(queryURL)
	return docURL == queryURL || strings.HasPrefix(queryURL, docURL+"/") || strings.HasPrefix(queryURL, docURL+"?")
}

func metadataString(metadata map[string]any, key string) string {
	if metadata == nil {
		return ""
	}
	value, ok := metadata[key]
	if !ok {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	default:
		return ""
	}
}

func normalizeScopeURL(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	parsed, err := url.Parse(value)
	if err == nil && parsed.Scheme != "" && parsed.Host != "" {
		host := strings.ToLower(parsed.Host)
		hostname := strings.ToLower(parsed.Hostname())
		if isLoopbackHost(hostname) {
			host = hostname
		}
		path := strings.TrimRight(parsed.EscapedPath(), "/")
		if path == "" {
			path = "/"
		}
		return parsed.Scheme + "://" + host + path
	}
	if before, _, ok := strings.Cut(value, "#"); ok {
		value = before
	}
	if before, _, ok := strings.Cut(value, "?"); ok {
		value = before
	}
	return strings.TrimRight(value, "/")
}

func isLoopbackHost(hostname string) bool {
	if hostname == "localhost" {
		return true
	}
	ip := net.ParseIP(hostname)
	return ip != nil && ip.IsLoopback()
}

type pruneCandidate struct {
	id string
	at time.Time
}

func pruneCandidates(candidates []pruneCandidate, limit int, remove func(string)) int {
	if limit <= 0 || len(candidates) <= limit {
		return 0
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].at.Equal(candidates[j].at) {
			return candidates[i].id < candidates[j].id
		}
		return candidates[i].at.After(candidates[j].at)
	})
	deleted := 0
	for _, candidate := range candidates[limit:] {
		remove(candidate.id)
		deleted++
	}
	return deleted
}

func memoryProjectAndSiteMatch(policy MemoryPrunePolicy, projectID string, site string) bool {
	if policy.ProjectID != "" && projectID != policy.ProjectID {
		return false
	}
	if policy.Site != "" && site != policy.Site {
		return false
	}
	return true
}

func nonZeroTime(values ...time.Time) time.Time {
	for _, value := range values {
		if !value.IsZero() {
			return value
		}
	}
	return time.Time{}
}

func cosineSimilarity(left []float32, right []float32) float64 {
	if len(left) == 0 || len(left) != len(right) {
		return 0
	}
	var dot float64
	var leftNorm float64
	var rightNorm float64
	for index := range left {
		l := float64(left[index])
		r := float64(right[index])
		dot += l * r
		leftNorm += l * l
		rightNorm += r * r
	}
	if leftNorm == 0 || rightNorm == 0 {
		return 0
	}
	return dot / (math.Sqrt(leftNorm) * math.Sqrt(rightNorm))
}

func newID(prefix string) string {
	var bytes [8]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return prefix + "_" + time.Now().UTC().Format("20060102150405.000000000")
	}
	return prefix + "_" + hex.EncodeToString(bytes[:])
}
