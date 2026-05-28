package workflow

type RiskLevel string

const (
	RiskLevelReadOnly         RiskLevel = "read_only"
	RiskLevelFormFill         RiskLevel = "form_fill"
	RiskLevelSubmitSearch     RiskLevel = "submit_search"
	RiskLevelStateChange      RiskLevel = "state_change"
	RiskLevelProductionChange RiskLevel = "production_change"
	RiskLevelPermissionChange RiskLevel = "permission_change"
	RiskLevelPayment          RiskLevel = "payment"
	RiskLevelDelete           RiskLevel = "delete"
	RiskLevelLoginSecret      RiskLevel = "login_secret"
	RiskLevelCaptcha          RiskLevel = "captcha"
	RiskLevelMFA              RiskLevel = "mfa"
)

type SafetyPolicy struct {
	AllowedRiskLevels      []RiskLevel `json:"allowedRiskLevels,omitempty"`
	ConfirmationRiskLevels []RiskLevel `json:"confirmationRiskLevels,omitempty"`
	HandoverRiskLevels     []RiskLevel `json:"handoverRiskLevels,omitempty"`
	BlockedRiskLevels      []RiskLevel `json:"blockedRiskLevels,omitempty"`
}

func IsKnownRiskLevel(level RiskLevel) bool {
	switch level {
	case RiskLevelReadOnly,
		RiskLevelFormFill,
		RiskLevelSubmitSearch,
		RiskLevelStateChange,
		RiskLevelProductionChange,
		RiskLevelPermissionChange,
		RiskLevelPayment,
		RiskLevelDelete,
		RiskLevelLoginSecret,
		RiskLevelCaptcha,
		RiskLevelMFA:
		return true
	default:
		return false
	}
}
