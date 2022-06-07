package messaging

type Alert struct {
	AlertDescription string `json:"alert_description"`
	Level            string `json:"level"`
	Details          string `json:"details"`
}
