package messaging

type Alert struct {
	AlertDescription string `json:"alert_description"`
	Severity         string `json:"severity"`
	Details          string `json:"details"`
}
