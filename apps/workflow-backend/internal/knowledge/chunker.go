package knowledge

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/page-agent/workflow-backend/internal/registry"
)

const MaxChunkChars = 500

type DocumentInput struct {
	ID         string
	ProjectID  string
	Title      string
	Source     string
	URL        string
	Content    string
	Tags       []string
	SourceType string
}

var sensitiveDocumentPattern = regexp.MustCompile(`(?i)\b(token|api[_-]?key|password|passwd|secret|cookie|captcha|authorization)\b\s*[:=]?\s*\S*`)

func ChunkDocument(input DocumentInput) ([]registry.KnowledgeChunk, error) {
	content := normalizeContent(input.Content)
	if content == "" {
		return nil, errors.New("document content is required")
	}
	if sensitiveDocumentPattern.MatchString(content) {
		return nil, errors.New("document contains sensitive material")
	}
	metadata := map[string]any{
		"url":        input.URL,
		"sourceType": normalizedSourceType(input.SourceType),
	}
	if site := siteFromDocumentURL(input.URL); site != "" {
		metadata["site"] = site
	}
	if module := moduleFromTags(input.Tags); module != "" {
		metadata["module"] = module
	}
	if headings := markdownHeadings(input.Content); len(headings) > 0 {
		metadata["headings"] = headings
	}
	chunks := []registry.KnowledgeChunk{}
	runes := []rune(content)
	for start := 0; start < len(runes); start += MaxChunkChars {
		end := start + MaxChunkChars
		if end > len(runes) {
			end = len(runes)
		}
		chunkText := strings.TrimSpace(string(runes[start:end]))
		if chunkText == "" {
			continue
		}
		chunks = append(chunks, registry.KnowledgeChunk{
			ID:         chunkID(input.ID, len(chunks)+1),
			DocumentID: input.ID,
			ProjectID:  input.ProjectID,
			Title:      input.Title,
			Source:     input.Source,
			ChunkText:  chunkText,
			Metadata:   metadata,
			Tags:       input.Tags,
		})
	}
	return chunks, nil
}

func normalizedSourceType(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "test", "seed", "example":
		return value
	default:
		return "production"
	}
}

func moduleFromTags(tags []string) string {
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag != "" {
			return tag
		}
	}
	return ""
}

func siteFromDocumentURL(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return ""
	}
	withoutScheme := rawURL
	if after, ok := strings.CutPrefix(withoutScheme, "https://"); ok {
		withoutScheme = after
	} else if after, ok := strings.CutPrefix(withoutScheme, "http://"); ok {
		withoutScheme = after
	}
	if before, _, ok := strings.Cut(withoutScheme, "/"); ok {
		withoutScheme = before
	}
	if before, _, ok := strings.Cut(withoutScheme, "?"); ok {
		withoutScheme = before
	}
	return strings.ToLower(strings.TrimSpace(withoutScheme))
}

func normalizeContent(content string) string {
	lines := strings.Split(content, "\n")
	normalized := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		normalized = append(normalized, line)
	}
	return strings.Join(normalized, "\n")
}

func markdownHeadings(content string) []string {
	headings := []string{}
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "#") {
			continue
		}
		title := strings.TrimSpace(strings.TrimLeft(line, "#"))
		if title != "" {
			headings = append(headings, title)
		}
	}
	return headings
}

func chunkID(documentID string, index int) string {
	if strings.TrimSpace(documentID) == "" {
		return ""
	}
	return fmt.Sprintf("%s_chunk_%03d", documentID, index)
}
