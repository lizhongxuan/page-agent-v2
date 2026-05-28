package workflow

type PageFingerprint struct {
	URLPatterns       []string           `json:"urlPatterns,omitempty"`
	TitleAny          []string           `json:"titleAny,omitempty"`
	RequiredText      []string           `json:"requiredText,omitempty"`
	ForbiddenText     []string           `json:"forbiddenText,omitempty"`
	ControlSignatures []ControlSignature `json:"controlSignatures,omitempty"`
	SemanticRegions   []SemanticRegion   `json:"semanticRegions,omitempty"`
	MinMatchScore     float64            `json:"minMatchScore,omitempty"`
}

type ControlSignature struct {
	Role        string `json:"role,omitempty"`
	Name        string `json:"name,omitempty"`
	Label       string `json:"label,omitempty"`
	Placeholder string `json:"placeholder,omitempty"`
}

type SemanticRegion struct {
	Name         string   `json:"name"`
	RequiredText []string `json:"requiredText,omitempty"`
}
