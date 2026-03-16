package notifier

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type EmailNotifier struct {
	APIToken string
	InboxID  string
	From     string
	To       string
}

type mailtrapAddress struct {
	Email string `json:"email"`
}

type mailtrapRequest struct {
	From    mailtrapAddress   `json:"from"`
	To      []mailtrapAddress `json:"to"`
	Subject string            `json:"subject"`
	Text    string            `json:"text"`
}

type mailtrapResponse struct {
	Success    bool     `json:"success"`
	MessageIDs []string `json:"message_ids"`
	Errors     []string `json:"errors"`
}

func (e *EmailNotifier) Send(message string) error {
	url := fmt.Sprintf("https://sandbox.api.mailtrap.io/api/send/%s", e.InboxID)

	payload := mailtrapRequest{
		From:    mailtrapAddress{Email: e.From},
		To:      []mailtrapAddress{{Email: e.To}},
		Subject: "Scanner Alert",
		Text:    message,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal request failed: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("create request failed: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+e.APIToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	var result mailtrapResponse
	_ = json.NewDecoder(resp.Body).Decode(&result)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("mailtrap api status %d, errors: %v", resp.StatusCode, result.Errors)
	}

	if !result.Success {
		return fmt.Errorf("mailtrap api returned success=false, errors: %v", result.Errors)
	}

	return nil
}
