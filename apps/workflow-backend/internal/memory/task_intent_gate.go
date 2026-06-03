package memory

import (
	"regexp"
	"strings"

	"github.com/page-agent/workflow-backend/internal/registry"
)

type taskIntentGateResult struct {
	Passed       bool
	Score        float64
	Reason       string
	MatchedTerms []string
}

func evaluateTaskIntentGate(task string, guide registry.SiteTaskGuide) taskIntentGateResult {
	task = strings.TrimSpace(task)
	if task == "" {
		return taskIntentGateResult{Passed: false, Reason: "missing_task"}
	}
	if matched := matchedIntentTerms(task, guide.TaskIntentTerms.Negative); len(matched) > 0 {
		return taskIntentGateResult{Passed: false, Reason: "opposite_intent:" + strings.Join(matched, ","), MatchedTerms: matched}
	}
	positive := guide.TaskIntentTerms.Positive
	positive = append(positive, intentTermsFromText(strings.Join([]string{guide.TaskIntentKey, guide.TaskIntentSummary, guide.Summary}, " "))...)
	if len(positive) == 0 {
		positive = intentTermsFromText(strings.Join([]string{guide.TaskIntentKey, guide.TaskIntentSummary}, " "))
	}
	positive = uniqueLimited(positive, 50)
	matched := matchedIntentTerms(task, positive)
	if len(matched) == 0 {
		return taskIntentGateResult{Passed: false, Reason: "task_intent_mismatch"}
	}
	guideActions := matchedPrimaryActionTerms(strings.Join(append(positive, guide.TaskIntentSummary), " "))
	taskActions := matchedPrimaryActionTerms(task)
	if len(guideActions) > 0 && len(taskActions) > 0 && !hasSharedString(guideActions, taskActions) {
		return taskIntentGateResult{Passed: false, Reason: "task_action_mismatch", MatchedTerms: matched}
	}
	score := float64(len(matched))
	if strings.Contains(strings.ToLower(guide.TaskIntentSummary), strings.ToLower(task)) ||
		strings.Contains(strings.ToLower(task), strings.ToLower(guide.TaskIntentSummary)) {
		score += 1
	}
	return taskIntentGateResult{Passed: true, Score: score, Reason: "task_intent_matched", MatchedTerms: matched}
}

func containsAnyIntentTerm(text string, terms []string) bool {
	return len(matchedIntentTerms(text, terms)) > 0
}

func matchedIntentTerms(text string, terms []string) []string {
	text = strings.ToLower(strings.TrimSpace(text))
	if text == "" {
		return nil
	}
	matched := []string{}
	for _, term := range terms {
		term = strings.ToLower(strings.TrimSpace(term))
		if term == "" {
			continue
		}
		if isASCIIIntentTerm(term) {
			if regexp.MustCompile(`(^|[^a-z0-9_])` + regexp.QuoteMeta(term) + `([^a-z0-9_]|$)`).MatchString(text) {
				matched = append(matched, term)
			}
			continue
		}
		if strings.Contains(text, term) {
			matched = append(matched, term)
		}
	}
	return uniqueLimited(matched, 20)
}

func matchedPrimaryActionTerms(text string) []string {
	groups := map[string][]string{
		"restore":  {"restore", "recover", "恢复"},
		"reset":    {"reset", "credential", "重置", "凭证"},
		"restart":  {"restart", "restarted"},
		"refund":   {"refund", "refunded"},
		"escalate": {"escalate", "escalated", "escalation"},
		"rollback": {"rollback", "rolled back"},
		"export":   {"export", "exported"},
		"delete":   {"delete", "remove", "drop", "cleanup", "删除", "移除", "销毁", "清理"},
		"merge":    {"merge", "merged"},
		"promote":  {"promote", "promoted"},
		"capture":  {"capture", "captured"},
		"archive":  {"archive", "archived"},
	}
	result := []string{}
	for canonical, terms := range groups {
		if len(matchedIntentTerms(text, terms)) > 0 {
			result = append(result, canonical)
		}
	}
	return uniqueLimited(result, 20)
}

func hasSharedString(left, right []string) bool {
	seen := map[string]bool{}
	for _, value := range left {
		seen[value] = true
	}
	for _, value := range right {
		if seen[value] {
			return true
		}
	}
	return false
}

func isASCIIIntentTerm(value string) bool {
	for _, r := range value {
		if r > 127 {
			return false
		}
	}
	return true
}
