package recipe

import "strings"

const RedactedValue = "[REDACTED]"

type RedactionReport struct {
	RedactedFields             []RedactedField `json:"redactedFields,omitempty"`
	ContainsRawSensitiveValues bool            `json:"containsRawSensitiveValues"`
}

type RedactedField struct {
	EventID              string `json:"eventId"`
	Label                string `json:"label,omitempty"`
	FieldType            string `json:"fieldType,omitempty"`
	Replacement          string `json:"replacement"`
	OriginalValuePreview string `json:"originalValuePreview,omitempty"`
}

func BuildRedactionReport(events []RecordedEvent) RedactionReport {
	report := RedactionReport{}
	for _, event := range events {
		if !IsSensitiveEvent(event) {
			continue
		}
		report.RedactedFields = append(report.RedactedFields, RedactedField{
			EventID:     event.ID,
			Label:       event.Label,
			FieldType:   event.FieldType,
			Replacement: RedactedValue,
		})
	}
	return report
}

func IsSensitiveEvent(event RecordedEvent) bool {
	if event.Sensitive {
		return true
	}
	fieldType := strings.ToLower(event.FieldType)
	label := strings.ToLower(event.Label)
	return fieldType == "password" ||
		strings.Contains(label, "password") ||
		strings.Contains(label, "token") ||
		strings.Contains(label, "verification code") ||
		strings.Contains(label, "验证码") ||
		strings.Contains(label, "mfa")
}
