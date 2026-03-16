package notifier

import (
	"fmt"
	"net/http"
	"net/url"
	"time"
)

type TelegramNotifier struct {
	Token  string
	ChatID string
}

func (t *TelegramNotifier) Send(message string) error {
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", t.Token)

	data := url.Values{}
	data.Set("chat_id", t.ChatID)
	data.Set("text", message)

	client := &http.Client{
		Timeout: 15 * time.Second,
	}

	resp, err := client.PostForm(apiURL, data)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("telegram api returned status %d", resp.StatusCode)
	}

	return nil
}
