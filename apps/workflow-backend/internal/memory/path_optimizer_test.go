package memory

import (
	"reflect"
	"testing"
)

func TestOptimizePathRemovesNoOutputLoopWhenDirectTransitionExists(t *testing.T) {
	result := OptimizePath(OptimizePathInput{
		Path: []string{"A", "B", "C", "A", "D"},
		Steps: []PathActionStep{
			{ID: "step-a-b", FromPageID: "A", ToPageID: "B"},
			{ID: "step-b-c", FromPageID: "B", ToPageID: "C"},
			{ID: "step-c-a", FromPageID: "C", ToPageID: "A"},
			{ID: "step-a-d", FromPageID: "A", ToPageID: "D"},
		},
		DirectTransitions: []DirectTransition{{FromPageID: "A", ToPageID: "D"}},
	})

	assertStringSlicesEqual(t, result.OriginalPath, []string{"A", "B", "C", "A", "D"})
	assertStringSlicesEqual(t, result.OptimizedPath, []string{"A", "D"})
	assertStringSlicesEqual(t, result.NoiseStepIDs, []string{"step-a-b", "step-b-c", "step-c-a"})
}

func TestOptimizePathKeepsPagesWithOutput(t *testing.T) {
	result := OptimizePath(OptimizePathInput{
		Path: []string{"A", "B", "C", "A", "D"},
		PageStates: map[string]PathPageState{
			"B": {HasOutput: true},
		},
		DirectTransitions: []DirectTransition{{FromPageID: "A", ToPageID: "D"}},
	})

	assertStringSlicesEqual(t, result.OptimizedPath, []string{"A", "B", "C", "A", "D"})
	assertStringSlicesEqual(t, result.NoiseStepIDs, nil)
}

func TestOptimizePathKeepsHighRiskConfirmationPages(t *testing.T) {
	result := OptimizePath(OptimizePathInput{
		Path: []string{"A", "B", "C", "A", "D"},
		PageStates: map[string]PathPageState{
			"C": {HighRiskConfirmation: true},
		},
		DirectTransitions: []DirectTransition{{FromPageID: "A", ToPageID: "D"}},
	})

	assertStringSlicesEqual(t, result.OptimizedPath, []string{"A", "B", "C", "A", "D"})
	assertStringSlicesEqual(t, result.NoiseStepIDs, nil)
}

func TestOptimizePathKeepsGuardDependencyPages(t *testing.T) {
	result := OptimizePath(OptimizePathInput{
		Path: []string{"A", "B", "C", "A", "D"},
		PageStates: map[string]PathPageState{
			"B": {GuardDependency: true},
		},
		DirectTransitions: []DirectTransition{{FromPageID: "A", ToPageID: "D"}},
	})

	assertStringSlicesEqual(t, result.OptimizedPath, []string{"A", "B", "C", "A", "D"})
	assertStringSlicesEqual(t, result.NoiseStepIDs, nil)
}

func TestOptimizePathDoesNotCompressWithoutDirectTransition(t *testing.T) {
	result := OptimizePath(OptimizePathInput{
		Path: []string{"A", "B", "C", "A", "D"},
	})

	assertStringSlicesEqual(t, result.OptimizedPath, []string{"A", "B", "C", "A", "D"})
	assertStringSlicesEqual(t, result.NoiseStepIDs, nil)
}

func TestPathActionStepsCanBeMarkedAsBranchNoise(t *testing.T) {
	steps := MarkBranchNoise([]PathActionStep{
		{ID: "step-a-b"},
		{ID: "step-b-c"},
		{ID: "step-a-d"},
	}, []string{"step-a-b", "step-b-c"})

	if !steps[0].IsBranchNoise || !steps[1].IsBranchNoise {
		t.Fatalf("expected first two steps to be marked as branch noise: %+v", steps)
	}
	if steps[2].IsBranchNoise {
		t.Fatalf("expected step-a-d to remain non-noise: %+v", steps[2])
	}
}

func assertStringSlicesEqual(t *testing.T, got, want []string) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}
