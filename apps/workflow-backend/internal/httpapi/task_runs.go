package httpapi

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/page-agent/workflow-backend/internal/registry"
)

func validateTaskRunRequest(run registry.TaskRun) error {
	if run.ProjectID == "" {
		return errString("projectId is required")
	}
	if run.Site == "" {
		return errString("site is required")
	}
	if run.TaskTemplate == "" {
		return errString("taskTemplate is required")
	}
	if err := registry.ValidateSummaryLength(run.Summary); err != nil {
		return err
	}
	for _, step := range run.ActionSteps {
		if err := registry.ValidateSummaryLength(step.ReasoningSummary); err != nil {
			return err
		}
		if err := registry.ValidateSummaryLength(step.ResultSummary); err != nil {
			return err
		}
		if step.ActionType == registry.StepFill && step.ValueTemplate == "" {
			return errString("fill action steps require valueTemplate")
		}
	}
	return nil
}

func prepareTaskRun(run *registry.TaskRun) {
	if run.ID == "" {
		run.ID = randomID("task_run")
	}
	if run.CreatedAt.IsZero() {
		run.CreatedAt = time.Now().UTC()
	}
	for index := range run.ActionSteps {
		if run.ActionSteps[index].ID == "" {
			run.ActionSteps[index].ID = randomID("step")
		}
		run.ActionSteps[index].TaskRunID = run.ID
		if run.ActionSteps[index].CreatedAt.IsZero() {
			run.ActionSteps[index].CreatedAt = run.CreatedAt
		}
	}
}

func hasActionInstanceValue(raw []byte) bool {
	var payload struct {
		ActionSteps []map[string]any `json:"actionSteps"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return false
	}
	for _, step := range payload.ActionSteps {
		if _, ok := step["value"]; ok {
			return true
		}
	}
	return false
}

func randomID(prefix string) string {
	var bytes [8]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return prefix + "_" + hex.EncodeToString([]byte(time.Now().UTC().Format("20060102150405.000000000")))
	}
	return prefix + "_" + hex.EncodeToString(bytes[:])
}

type errString string

func (err errString) Error() string {
	return string(err)
}
