package retrieval

import "testing"

func TestDetectPageStateMatchesGitHubRepoHome(t *testing.T) {
	detector := NewPageStateDetector(fakePageStateSearcher{
		hits: []PageStateHit{
			{
				PageStateID:      "github_repo_home",
				Site:             "github.com",
				App:              "github",
				Score:            0.92,
				RequiredText:     []string{"Code", "Issues", "Pull requests"},
				RequiredControls: []Control{{Role: "link", Name: "Issues"}},
			},
		},
	})

	result, ok := detector.Detect(PageStateRequest{
		ProjectID: "default",
		Site:      "github.com",
		App:       "github",
		Observation: PageObservation{
			VisibleText: []string{"Code", "Issues", "Pull requests"},
			Controls:    []Control{{Role: "link", Name: "Issues"}},
		},
		FingerprintText: "github repo home Code Issues Pull requests link Issues",
	})

	if !ok {
		t.Fatal("expected page state match")
	}
	if result.PageStateID != "github_repo_home" {
		t.Fatalf("unexpected page state: %#v", result)
	}
}

func TestDetectPageStateRejectsMissingControls(t *testing.T) {
	detector := NewPageStateDetector(fakePageStateSearcher{
		hits: []PageStateHit{
			{
				PageStateID:      "github_issues_list",
				Score:            0.95,
				RequiredText:     []string{"Issues"},
				RequiredControls: []Control{{Role: "textbox", Name: "Search"}},
			},
		},
	})

	_, ok := detector.Detect(PageStateRequest{
		ProjectID: "default",
		Site:      "github.com",
		Observation: PageObservation{
			VisibleText: []string{"Issues"},
			Controls:    []Control{{Role: "link", Name: "Issues"}},
		},
		FingerprintText: "Issues link Issues",
	})

	if ok {
		t.Fatal("expected page state rejection when required control is missing")
	}
}

type fakePageStateSearcher struct {
	hits []PageStateHit
}

func (searcher fakePageStateSearcher) SearchPageStates(PageStateRequest) ([]PageStateHit, error) {
	return searcher.hits, nil
}
