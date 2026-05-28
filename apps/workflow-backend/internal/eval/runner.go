package eval

import (
	"encoding/json"
	"fmt"
	"os"
)

type Case struct {
	ID                     string            `json:"id"`
	RecordedTask           string            `json:"recordedTask"`
	ReplayTask             string            `json:"replayTask"`
	CurrentURL             string            `json:"currentUrl"`
	ExpectedWorkflowID     string            `json:"expectedWorkflowId"`
	ExpectedBindings       map[string]string `json:"expectedBindings"`
	ExpectedStartPageState string            `json:"expectedStartPageState"`
	ExpectedFinalPageState string            `json:"expectedFinalPageState"`
}

type Prediction struct {
	CaseID              string            `json:"caseId"`
	RankedWorkflowIDs   []string          `json:"rankedWorkflowIds"`
	SelectedWorkflowID  string            `json:"selectedWorkflowId,omitempty"`
	Bindings            map[string]string `json:"bindings,omitempty"`
	StartPageState      string            `json:"startPageState,omitempty"`
	FinalPageState      string            `json:"finalPageState,omitempty"`
	InterruptHandlerHit bool              `json:"interruptHandlerHit,omitempty"`
	RepairPatchReused   bool              `json:"repairPatchReused,omitempty"`
	ReplayCompleted     bool              `json:"replayCompleted,omitempty"`
	FallbackToPageAgent bool              `json:"fallbackToPageAgent,omitempty"`
}

type Options struct {
	K int
}

type Runner struct {
	Cases   []Case
	Options Options
}

func LoadCases(path string) ([]Case, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var envelope struct {
		Cases []Case `json:"cases"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, err
	}
	if len(envelope.Cases) == 0 {
		var cases []Case
		if err := json.Unmarshal(data, &cases); err != nil {
			return nil, err
		}
		envelope.Cases = cases
	}

	for i := range envelope.Cases {
		if err := envelope.Cases[i].Validate(); err != nil {
			return nil, fmt.Errorf("case %d: %w", i, err)
		}
	}
	return envelope.Cases, nil
}

func (evalCase Case) Validate() error {
	if evalCase.ID == "" {
		return fmt.Errorf("id is required")
	}
	if evalCase.RecordedTask == "" {
		return fmt.Errorf("recordedTask is required")
	}
	if evalCase.ReplayTask == "" {
		return fmt.Errorf("replayTask is required")
	}
	if evalCase.CurrentURL == "" {
		return fmt.Errorf("currentUrl is required")
	}
	if evalCase.ExpectedWorkflowID == "" {
		return fmt.Errorf("expectedWorkflowId is required")
	}
	if len(evalCase.ExpectedBindings) == 0 {
		return fmt.Errorf("expectedBindings is required")
	}
	if evalCase.ExpectedStartPageState == "" {
		return fmt.Errorf("expectedStartPageState is required")
	}
	if evalCase.ExpectedFinalPageState == "" {
		return fmt.Errorf("expectedFinalPageState is required")
	}
	return nil
}

func (runner Runner) Evaluate(predictions []Prediction) (Metrics, error) {
	return CalculateMetrics(runner.Cases, predictions, runner.Options)
}
