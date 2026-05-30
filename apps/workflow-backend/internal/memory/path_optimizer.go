package memory

type OptimizePathInput struct {
	Path              []string
	Steps             []PathActionStep
	PageStates        map[string]PathPageState
	DirectTransitions []DirectTransition
}

type PathActionStep struct {
	ID            string
	FromPageID    string
	ToPageID      string
	IsBranchNoise bool
}

type PathPageState struct {
	HasOutput            bool
	HighRiskConfirmation bool
	GuardDependency      bool
}

type DirectTransition struct {
	FromPageID string
	ToPageID   string
}

type OptimizePathResult struct {
	OriginalPath   []string
	OptimizedPath  []string
	NoiseStepIDs   []string
	OptimizedSteps []PathActionStep
}

func OptimizePath(input OptimizePathInput) OptimizePathResult {
	originalPath := copyStrings(input.Path)
	optimizedPath := make([]string, 0, len(input.Path))
	noiseStepIDs := []string{}
	directTransitions := directTransitionSet(input.DirectTransitions)

	for i := 0; i < len(input.Path); i++ {
		optimizedPath = append(optimizedPath, input.Path[i])

		replacementEnd := -1
		for j := len(input.Path) - 1; j > i+1; j-- {
			if !directTransitions[transitionKey(input.Path[i], input.Path[j])] {
				continue
			}
			if !canRemoveIntermediatePages(input.Path[i+1:j], input.PageStates) {
				continue
			}
			replacementEnd = j
			break
		}

		if replacementEnd == -1 {
			continue
		}

		noiseStepIDs = append(noiseStepIDs, noiseStepsForReplacement(input.Steps, input.Path[i], input.Path[replacementEnd], input.Path[i:replacementEnd+1])...)
		i = replacementEnd - 1
	}

	result := OptimizePathResult{
		OriginalPath:  originalPath,
		OptimizedPath: optimizedPath,
		NoiseStepIDs:  uniqueStrings(noiseStepIDs),
	}
	result.OptimizedSteps = MarkBranchNoise(input.Steps, result.NoiseStepIDs)
	return result
}

func MarkBranchNoise(steps []PathActionStep, noiseStepIDs []string) []PathActionStep {
	noiseSet := map[string]bool{}
	for _, stepID := range noiseStepIDs {
		noiseSet[stepID] = true
	}

	result := make([]PathActionStep, len(steps))
	copy(result, steps)
	for index := range result {
		if noiseSet[result[index].ID] {
			result[index].IsBranchNoise = true
		}
	}
	return result
}

func canRemoveIntermediatePages(pageIDs []string, pageStates map[string]PathPageState) bool {
	for _, pageID := range pageIDs {
		state := pageStates[pageID]
		if state.HasOutput || state.HighRiskConfirmation || state.GuardDependency {
			return false
		}
	}
	return true
}

func noiseStepsForReplacement(steps []PathActionStep, fromPageID, toPageID string, replacedPath []string) []string {
	replacedEdges := map[string]bool{}
	for i := 0; i < len(replacedPath)-1; i++ {
		replacedEdges[transitionKey(replacedPath[i], replacedPath[i+1])] = true
	}

	noiseStepIDs := []string{}
	for _, step := range steps {
		if step.ID == "" {
			continue
		}
		if step.FromPageID == fromPageID && step.ToPageID == toPageID {
			continue
		}
		if replacedEdges[transitionKey(step.FromPageID, step.ToPageID)] {
			noiseStepIDs = append(noiseStepIDs, step.ID)
		}
	}
	return noiseStepIDs
}

func directTransitionSet(transitions []DirectTransition) map[string]bool {
	result := map[string]bool{}
	for _, transition := range transitions {
		result[transitionKey(transition.FromPageID, transition.ToPageID)] = true
	}
	return result
}

func transitionKey(fromPageID, toPageID string) string {
	return fromPageID + "\x00" + toPageID
}

func copyStrings(values []string) []string {
	if values == nil {
		return nil
	}
	result := make([]string, len(values))
	copy(result, values)
	return result
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	result := []string{}
	for _, value := range values {
		if seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	if len(result) == 0 {
		return nil
	}
	return result
}
