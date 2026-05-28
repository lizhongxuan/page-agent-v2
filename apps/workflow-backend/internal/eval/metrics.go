package eval

import "fmt"

type Metrics struct {
	RecallAtK                  float64 `json:"recallAtK"`
	MRR                        float64 `json:"mrr"`
	PrecisionAt1               float64 `json:"precisionAt1"`
	FalsePositiveRate          float64 `json:"falsePositiveRate"`
	VariableBindingAccuracy    float64 `json:"variableBindingAccuracy"`
	PageStateDetectionAccuracy float64 `json:"pageStateDetectionAccuracy"`
	InterruptHandlerHitRate    float64 `json:"interruptHandlerHitRate"`
	RepairPatchReuseRate       float64 `json:"repairPatchReuseRate"`
	ReplayCompletionRate       float64 `json:"replayCompletionRate"`
	FallbackToPageAgentRate    float64 `json:"fallbackToPageAgentRate"`
}

func CalculateMetrics(cases []Case, predictions []Prediction, options Options) (Metrics, error) {
	k := options.K
	if k <= 0 {
		k = 1
	}

	caseByID := make(map[string]Case, len(cases))
	for _, evalCase := range cases {
		if evalCase.ID == "" {
			return Metrics{}, fmt.Errorf("case id is required")
		}
		caseByID[evalCase.ID] = evalCase
	}

	predictionByCaseID := make(map[string]Prediction, len(predictions))
	for _, prediction := range predictions {
		if _, ok := caseByID[prediction.CaseID]; !ok {
			return Metrics{}, fmt.Errorf("prediction references unknown case %q", prediction.CaseID)
		}
		predictionByCaseID[prediction.CaseID] = prediction
	}

	var metrics Metrics
	var positiveCases int
	var negativeCases int
	var recallHits int
	var precisionHits int
	var falsePositiveHits int
	var reciprocalRankSum float64
	var expectedBindingCount int
	var correctBindingCount int
	var expectedPageStateCount int
	var correctPageStateCount int
	var interruptHits int
	var repairReuseHits int
	var replayCompletionHits int
	var fallbackHits int

	for _, evalCase := range cases {
		prediction := predictionByCaseID[evalCase.ID]
		if evalCase.ExpectedWorkflowID == "" {
			negativeCases++
			if prediction.SelectedWorkflowID != "" || len(prediction.RankedWorkflowIDs) > 0 {
				falsePositiveHits++
			}
		} else {
			positiveCases++
			rank := rankOf(evalCase.ExpectedWorkflowID, prediction.RankedWorkflowIDs)
			if rank > 0 {
				reciprocalRankSum += 1 / float64(rank)
				if rank <= k {
					recallHits++
				}
			}
			if firstWorkflowID(prediction) == evalCase.ExpectedWorkflowID {
				precisionHits++
			}
		}

		for name, expectedValue := range evalCase.ExpectedBindings {
			expectedBindingCount++
			if prediction.Bindings[name] == expectedValue {
				correctBindingCount++
			}
		}
		if evalCase.ExpectedStartPageState != "" {
			expectedPageStateCount++
			if prediction.StartPageState == evalCase.ExpectedStartPageState {
				correctPageStateCount++
			}
		}
		if evalCase.ExpectedFinalPageState != "" {
			expectedPageStateCount++
			if prediction.FinalPageState == evalCase.ExpectedFinalPageState {
				correctPageStateCount++
			}
		}
		if prediction.InterruptHandlerHit {
			interruptHits++
		}
		if prediction.RepairPatchReused {
			repairReuseHits++
		}
		if prediction.ReplayCompleted {
			replayCompletionHits++
		}
		if prediction.FallbackToPageAgent {
			fallbackHits++
		}
	}

	metrics.RecallAtK = ratio(recallHits, positiveCases)
	metrics.MRR = ratioFloat(reciprocalRankSum, positiveCases)
	metrics.PrecisionAt1 = ratio(precisionHits, positiveCases)
	metrics.FalsePositiveRate = ratio(falsePositiveHits, negativeCases)
	metrics.VariableBindingAccuracy = ratio(correctBindingCount, expectedBindingCount)
	metrics.PageStateDetectionAccuracy = ratio(correctPageStateCount, expectedPageStateCount)
	metrics.InterruptHandlerHitRate = ratio(interruptHits, len(cases))
	metrics.RepairPatchReuseRate = ratio(repairReuseHits, len(cases))
	metrics.ReplayCompletionRate = ratio(replayCompletionHits, len(cases))
	metrics.FallbackToPageAgentRate = ratio(fallbackHits, len(cases))
	return metrics, nil
}

func rankOf(workflowID string, rankedWorkflowIDs []string) int {
	for index, candidateID := range rankedWorkflowIDs {
		if candidateID == workflowID {
			return index + 1
		}
	}
	return 0
}

func firstWorkflowID(prediction Prediction) string {
	if len(prediction.RankedWorkflowIDs) > 0 {
		return prediction.RankedWorkflowIDs[0]
	}
	return prediction.SelectedWorkflowID
}

func ratio(numerator int, denominator int) float64 {
	if denominator == 0 {
		return 0
	}
	return float64(numerator) / float64(denominator)
}

func ratioFloat(numerator float64, denominator int) float64 {
	if denominator == 0 {
		return 0
	}
	return numerator / float64(denominator)
}
