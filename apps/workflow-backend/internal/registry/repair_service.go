package registry

import (
	"context"
	"errors"
	"strconv"
	"time"
)

type RepairPatchService struct {
	repo Repository
}

func NewRepairPatchService(repo Repository) *RepairPatchService {
	return &RepairPatchService{repo: repo}
}

func (service *RepairPatchService) CreateCandidate(ctx context.Context, patch RepairPatch) (RepairPatch, error) {
	if patch.ID == "" {
		patch.ID = newID("patch")
	}
	patch.Status = StatusPendingReview
	now := time.Now().UTC()
	if patch.CreatedAt.IsZero() {
		patch.CreatedAt = now
	}
	patch.UpdatedAt = now
	if err := ValidateRepairPatchCandidate(patch); err != nil {
		return RepairPatch{}, err
	}
	if err := service.repo.SaveRepairPatch(ctx, patch); err != nil {
		return RepairPatch{}, err
	}
	return patch, nil
}

func (service *RepairPatchService) Approve(ctx context.Context, id string) (RepairPatch, error) {
	patch, err := service.repo.GetRepairPatch(ctx, id)
	if err != nil {
		return RepairPatch{}, err
	}
	patch.Status = StatusActive
	patch.ReviewedAt = time.Now().UTC()
	patch.UpdatedAt = patch.ReviewedAt
	if err := ValidateRepairPatchCandidate(patch); err != nil {
		return RepairPatch{}, err
	}
	if err := service.repo.SaveRepairPatch(ctx, patch); err != nil {
		return RepairPatch{}, err
	}
	_, err = service.repo.AppendOutboxEvent(ctx, OutboxEvent{
		Type:           "repair_patch_approved",
		IdempotencyKey: "repair_patch_approved:" + patch.ID + ":v" + strconv.Itoa(patch.WorkflowVersion),
		Payload: map[string]any{
			"patchId":   patch.ID,
			"projectId": patch.ProjectID,
		},
	})
	if err != nil {
		return RepairPatch{}, err
	}
	return patch, nil
}

func (service *RepairPatchService) Reject(ctx context.Context, id string) (RepairPatch, error) {
	patch, err := service.repo.GetRepairPatch(ctx, id)
	if err != nil {
		return RepairPatch{}, err
	}
	patch.Status = StatusRejected
	patch.ReviewedAt = time.Now().UTC()
	patch.UpdatedAt = patch.ReviewedAt
	if err := service.repo.SaveRepairPatch(ctx, patch); err != nil {
		return RepairPatch{}, err
	}
	return patch, nil
}

func (service *RepairPatchService) List(ctx context.Context, query RepairPatchListQuery) ([]RepairPatch, error) {
	return service.repo.ListRepairPatches(ctx, query)
}

func ValidateRepairPatchCandidate(patch RepairPatch) error {
	if patch.ProjectID == "" {
		return errors.New("project id is required")
	}
	if patch.WorkflowID == "" {
		return errors.New("workflow id is required")
	}
	if patch.WorkflowVersion <= 0 {
		return errors.New("workflow version is required")
	}
	if patch.ChunkID == "" {
		return errors.New("chunk id is required")
	}
	if patch.StepID == "" {
		return errors.New("step id is required")
	}
	if patch.Site == "" {
		return errors.New("site is required")
	}
	if patch.FailureType == "" {
		return errors.New("failure type is required")
	}
	if patch.FailureSignature == "" {
		return errors.New("failure signature is required")
	}
	if patch.NewTargetSummary == "" && patch.NewTarget.Primary.Text() == "" {
		return errors.New("new target is required")
	}
	if patch.RiskLevel == RiskDestructive || patch.RiskLevel == RiskExternalSend {
		return errors.New("unsafe repair patch risk level")
	}
	return nil
}
