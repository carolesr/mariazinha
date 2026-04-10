package meta

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const apiURL = "https://graph.facebook.com/v19.0"

type Client struct {
	phoneID     string
	accessToken string
	http        *http.Client
}

func NewClient(phoneID, accessToken string) *Client {
	return &Client{
		phoneID:     phoneID,
		accessToken: accessToken,
		http:        &http.Client{},
	}
}

// SendText sends a plain text message to a WhatsApp number or group
func (c *Client) SendText(ctx context.Context, to, text string) error {
	payload := map[string]interface{}{
		"messaging_product": "whatsapp",
		"to":                to,
		"type":              "text",
		"text":              map[string]string{"body": text},
	}
	return c.post(ctx, payload)
}

// SendReply sends a text message quoting a specific message
func (c *Client) SendReply(ctx context.Context, to, text, quotedMessageID string) error {
	payload := map[string]interface{}{
		"messaging_product": "whatsapp",
		"to":                to,
		"type":              "text",
		"context":           map[string]string{"message_id": quotedMessageID},
		"text":              map[string]string{"body": text},
	}
	return c.post(ctx, payload)
}

func (c *Client) post(ctx context.Context, payload interface{}) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/%s/messages", apiURL, c.phoneID)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.accessToken)

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("meta API error %d: %s", resp.StatusCode, string(raw))
	}
	return nil
}
