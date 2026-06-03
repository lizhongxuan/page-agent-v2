package manualwiki

import (
	"regexp"
	"strings"
	"unicode"

	"github.com/page-agent/workflow-backend/internal/registry"
)

type Compiler interface {
	Compile(registry.SiteManualSource) ([]registry.SiteManualWikiPage, []registry.SiteManualWikiChunk, error)
}

type FallbackCompiler struct{}

func (FallbackCompiler) Compile(source registry.SiteManualSource) ([]registry.SiteManualWikiPage, []registry.SiteManualWikiChunk, error) {
	title := firstHeading(source.RawContent)
	if title == "" {
		title = source.Title
	}
	if title == "" {
		title = "Site manual"
	}
	pageKey := stableKey(title)
	if pageKey == "" {
		pageKey = "site_manual"
	}
	refs := []registry.MemorySourceRef{{Type: "site_manual_source", ID: source.ID}}
	paragraphs := meaningfulLines(source.RawContent)
	summary := truncateSummary(strings.Join(paragraphs, " "))
	page := registry.SiteManualWikiPage{
		ID:         "manual_page_" + source.ID + "_" + pageKey,
		ProjectID:  source.ProjectID,
		Site:       source.Site,
		Module:     source.Module,
		PageKey:    pageKey,
		Title:      title,
		Summary:    summary,
		Facts:      firstN(paragraphs, 6),
		Procedures: procedureLines(paragraphs),
		SourceRefs: refs,
		Confidence: 0.65,
		Status:     registry.StatusActive,
	}
	chunks := []registry.SiteManualWikiChunk{{
		ID:          "manual_chunk_" + source.ID + "_" + pageKey + "_summary",
		WikiPageID:  page.ID,
		ProjectID:   source.ProjectID,
		Site:        source.Site,
		Module:      source.Module,
		ChunkType:   registry.SiteManualChunkPageSummary,
		Text:        summary,
		TargetTerms: targetTerms(summary),
		SourceRefs:  refs,
		Status:      registry.StatusActive,
	}}
	for index, line := range paragraphs {
		chunkType := registry.SiteManualChunkProcedure
		lower := strings.ToLower(line)
		if strings.Contains(lower, "warning") || strings.Contains(lower, "risk") || strings.Contains(line, "注意") || strings.Contains(line, "风险") {
			chunkType = registry.SiteManualChunkWarning
		}
		chunks = append(chunks, registry.SiteManualWikiChunk{
			ID:          "manual_chunk_" + source.ID + "_" + pageKey + "_" + runeSuffix(index),
			WikiPageID:  page.ID,
			ProjectID:   source.ProjectID,
			Site:        source.Site,
			Module:      source.Module,
			ChunkType:   chunkType,
			Text:        truncateSummary(line),
			TargetTerms: targetTerms(line),
			SourceRefs:  refs,
			Status:      registry.StatusActive,
		})
	}
	return []registry.SiteManualWikiPage{page}, chunks, nil
}

func firstHeading(content string) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") {
			return strings.TrimSpace(strings.TrimLeft(line, "#"))
		}
	}
	return ""
}

func meaningfulLines(content string) []string {
	lines := []string{}
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		line = strings.TrimLeft(line, "-*0123456789. ")
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		lines = append(lines, truncateSummary(line))
	}
	if len(lines) == 0 && strings.TrimSpace(content) != "" {
		lines = append(lines, truncateSummary(strings.TrimSpace(content)))
	}
	return lines
}

func procedureLines(lines []string) []string {
	result := []string{}
	for _, line := range lines {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "click") || strings.Contains(lower, "select") || strings.Contains(lower, "open") || strings.Contains(line, "点击") || strings.Contains(line, "选择") {
			result = append(result, line)
		}
	}
	return result
}

func truncateSummary(value string) string {
	value = strings.TrimSpace(regexp.MustCompile(`\s+`).ReplaceAllString(value, " "))
	runes := []rune(value)
	if len(runes) <= registry.MaxSummaryChars {
		return value
	}
	runes = runes[:registry.MaxSummaryChars]
	for len(runes) > 0 && !unicode.IsSpace(runes[len(runes)-1]) && !strings.ContainsRune(".,;!?。！？；，", runes[len(runes)-1]) {
		runes = runes[:len(runes)-1]
	}
	return strings.TrimSpace(string(runes))
}

func stableKey(value string) string {
	value = strings.ToLower(value)
	value = regexp.MustCompile(`[^a-z0-9\p{Han}]+`).ReplaceAllString(value, "_")
	value = strings.Trim(value, "_")
	if len([]rune(value)) > 48 {
		value = string([]rune(value)[:48])
	}
	return value
}

func targetTerms(value string) []string {
	seen := map[string]bool{}
	result := []string{}
	for _, raw := range regexp.MustCompile(`[A-Za-z0-9\p{Han}]+`).FindAllString(value, -1) {
		term := strings.ToLower(strings.TrimSpace(raw))
		if len([]rune(term)) < 2 || seen[term] {
			continue
		}
		seen[term] = true
		result = append(result, term)
		if len(result) >= 12 {
			break
		}
	}
	return result
}

func firstN(values []string, limit int) []string {
	if len(values) <= limit {
		return values
	}
	return values[:limit]
}

func runeSuffix(index int) string {
	return strings.TrimPrefix(regexp.MustCompile(`[^a-z0-9]`).ReplaceAllString(string(rune('a'+index%26)), ""), "_")
}
