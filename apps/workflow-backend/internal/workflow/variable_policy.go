package workflow

type VariableType string

const (
	VariableTypeString  VariableType = "string"
	VariableTypeNumber  VariableType = "number"
	VariableTypeBoolean VariableType = "boolean"
)

type VariableSource string

const (
	VariableSourceUserTask   VariableSource = "user_task"
	VariableSourcePage       VariableSource = "page"
	VariableSourceUserAnswer VariableSource = "user_answer"
	VariableSourceRecording  VariableSource = "recording"
)

type BindingMode string

const (
	BindingModeAuto               BindingMode = "auto"
	BindingModeAskIfMissing       BindingMode = "ask_if_missing"
	BindingModeConfirmIfAmbiguous BindingMode = "confirm_if_ambiguous"
	BindingModeAlwaysConfirm      BindingMode = "always_confirm"
	BindingModeHandoverOnly       BindingMode = "handover_only"
)

type Variable struct {
	Name        string         `json:"name"`
	Type        VariableType   `json:"type"`
	Required    bool           `json:"required"`
	Source      VariableSource `json:"source"`
	Sensitive   bool           `json:"sensitive"`
	Description string         `json:"description,omitempty"`
	BindingMode BindingMode    `json:"bindingMode"`
}

func IsKnownBindingMode(mode BindingMode) bool {
	switch mode {
	case BindingModeAuto,
		BindingModeAskIfMissing,
		BindingModeConfirmIfAmbiguous,
		BindingModeAlwaysConfirm,
		BindingModeHandoverOnly:
		return true
	default:
		return false
	}
}
