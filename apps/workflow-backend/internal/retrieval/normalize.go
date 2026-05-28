package retrieval

import (
	"net/url"
	"regexp"
	"strings"
)

var chineseSearchPattern = regexp.MustCompile(`搜索\s*([^，。,.]+)`)
var englishSearchPattern = regexp.MustCompile(`(?i)\b(?:search|find)\s+(.+?)(?:\s+in\s+|\s+on\s+|$)`)

func NormalizeRequest(request SearchRequest) NormalizedRequest {
	projectID := strings.TrimSpace(request.ProjectID)
	if projectID == "" {
		projectID = "default"
	}
	site := siteFromURL(request.CurrentURL)
	app := appFromSite(site)
	slots := map[string]string{}
	if repo := githubRepoFromURL(request.CurrentURL); repo != "" {
		slots["repo"] = repo
	} else if repo := githubRepoFromTask(request.Task); repo != "" {
		slots["repo"] = repo
	}
	if query := queryFromTask(request.Task); query != "" {
		slots["query"] = query
	}
	return NormalizedRequest{
		ProjectID:           projectID,
		Task:                request.Task,
		CurrentURL:          request.CurrentURL,
		Site:                site,
		App:                 app,
		CandidateSlots:      slots,
		PageFingerprintText: pageFingerprintText(request),
		RiskPolicy:          request.RiskPolicy,
		Limit:               request.Limit,
	}
}

func githubRepoFromTask(task string) string {
	pattern := regexp.MustCompile(`(?i)github\.com/([A-Za-z0-9_.-]+)/([A-Za-z0-9_.-]+)`)
	matches := pattern.FindStringSubmatch(task)
	if len(matches) != 3 {
		return ""
	}
	return matches[1] + "/" + strings.TrimSuffix(matches[2], ".git")
}

func siteFromURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return parsed.Hostname()
}

func appFromSite(site string) string {
	if strings.EqualFold(site, "github.com") {
		return "github"
	}
	parts := strings.Split(site, ".")
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}

func githubRepoFromURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil || !strings.EqualFold(parsed.Hostname(), "github.com") {
		return ""
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return ""
	}
	return parts[0] + "/" + parts[1]
}

func queryFromTask(task string) string {
	if matches := chineseSearchPattern.FindStringSubmatch(task); len(matches) == 2 {
		return strings.TrimSpace(matches[1])
	}
	if matches := englishSearchPattern.FindStringSubmatch(task); len(matches) == 2 {
		value := strings.TrimSpace(matches[1])
		value = strings.TrimSuffix(value, " issues")
		return strings.TrimSpace(value)
	}
	return ""
}

func pageFingerprintText(request SearchRequest) string {
	parts := []string{request.CurrentURL, request.PageObservation.Title}
	parts = append(parts, request.PageObservation.VisibleText...)
	for _, control := range request.PageObservation.Controls {
		parts = append(parts, control.Role, control.Name)
	}
	return strings.Join(nonEmpty(parts), " ")
}

func nonEmpty(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			result = append(result, strings.TrimSpace(value))
		}
	}
	return result
}
