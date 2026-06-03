package manualwiki

import (
	"strings"

	"github.com/page-agent/workflow-backend/internal/registry"
)

func FormatPrompt(matches []registry.SiteManualKnowledgeMatch) string {
	if len(matches) == 0 {
		return ""
	}
	var builder strings.Builder
	builder.WriteString("<site_manual_knowledge>\n")
	for _, match := range matches {
		chunk := match.Chunk
		builder.WriteString(`  <manual id="`)
		builder.WriteString(escapeXML(chunk.ID))
		builder.WriteString(`" confidence="`)
		builder.WriteString(formatScore(match.Score))
		builder.WriteString("\">\n")
		builder.WriteString("    <summary>")
		builder.WriteString(escapeXML(chunk.Text))
		builder.WriteString("</summary>\n")
		if len(chunk.SourceRefs) > 0 {
			builder.WriteString("    <source_refs>")
			for index, ref := range chunk.SourceRefs {
				if index > 0 {
					builder.WriteString(", ")
				}
				builder.WriteString(escapeXML(ref.Type + ":" + ref.ID))
			}
			builder.WriteString("</source_refs>\n")
		}
		builder.WriteString("  </manual>\n")
	}
	builder.WriteString("</site_manual_knowledge>")
	return builder.String()
}

func escapeXML(value string) string {
	value = strings.ReplaceAll(value, "&", "&amp;")
	value = strings.ReplaceAll(value, "<", "&lt;")
	value = strings.ReplaceAll(value, ">", "&gt;")
	value = strings.ReplaceAll(value, `"`, "&quot;")
	return value
}

func formatScore(score float64) string {
	if score <= 0 {
		return "0.00"
	}
	if score >= 1 {
		return "1.00"
	}
	return "0." + string(rune('0'+int(score*10))) + string(rune('0'+int(score*100)%10))
}
