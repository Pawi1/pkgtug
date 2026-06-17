package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"time"
)

type Telegram struct {
	botToken string
	chatID   string
	client   *http.Client
}

func NewTelegram(botToken, chatID string) *Telegram {
	return &Telegram{
		botToken: botToken,
		chatID:   chatID,
		client:   &http.Client{Timeout: 10 * time.Second},
	}
}

func (t *Telegram) Enabled() bool {
	return t.botToken != "" && t.chatID != ""
}

func (t *Telegram) BuildSuccess(pkg, version, platform string) error {
	msg := fmt.Sprintf("✅ <b>%s</b> built\nVersion: <code>%s</code>\nPlatform: <code>%s</code>",
		html.EscapeString(pkg),
		html.EscapeString(version),
		html.EscapeString(platform),
	)
	return t.send(msg)
}

func (t *Telegram) BuildFailure(pkg, version, platform, errMsg string) error {
	msg := fmt.Sprintf("❌ <b>%s</b> build failed\nVersion: <code>%s</code>\nPlatform: <code>%s</code>\n<code>%s</code>",
		html.EscapeString(pkg),
		html.EscapeString(version),
		html.EscapeString(platform),
		html.EscapeString(errMsg),
	)
	return t.send(msg)
}

func (t *Telegram) send(text string) error {
	if !t.Enabled() {
		return nil
	}
	body, _ := json.Marshal(map[string]string{
		"chat_id":    t.chatID,
		"text":       text,
		"parse_mode": "HTML",
	})
	url := "https://api.telegram.org/bot" + t.botToken + "/sendMessage"
	resp, err := t.client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("telegram send: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram send: status %d", resp.StatusCode)
	}
	return nil
}
