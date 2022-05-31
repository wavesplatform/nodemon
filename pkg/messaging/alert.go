package messaging

type Alert struct {
	AlertDescription string `json:"alert_description"`
	Level            string `json:"severity"`
	Details          string `json:"details"`
}
