package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type Client struct {
	apiKey string
	http   *http.Client
}

func NewClient(apiKey string) *Client {
	return &Client{apiKey: apiKey, http: &http.Client{}}
}

type Intent struct {
	Action      string `json:"action"`      // join, leave, detail, list, create, cancel, admin_remove, admin_edit, unknown
	EventQuery  string `json:"event_query"` // name/date fragment to identify the event
	EventID     int64  `json:"event_id"`
	Name        string `json:"name"`
	Date        string `json:"date"`
	Time        string `json:"time"`
	Location    string `json:"location"`
	Description string `json:"description"`
	MaxSpots    *int   `json:"max_spots"`
	Cost        string `json:"cost"`
	EditField   string `json:"edit_field"`
	EditValue   string `json:"edit_value"`
}

type EventContext struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Date      string `json:"date"`
	Time      string `json:"time"`
	Location  string `json:"location"`
	Confirmed int    `json:"confirmed"`
	MaxSpots  *int   `json:"max_spots,omitempty"`
}

const systemPrompt = `You are the intent parser for a WhatsApp event management bot called Mariazinha.
Your only job is to read a message and return a JSON object describing the user's intent.
Respond with ONLY valid JSON — no explanation, no markdown, no code blocks.

Possible actions:
- "join"         — user wants to join an event
- "leave"        — user wants to leave an event
- "detail"       — user wants details about a specific event
- "list"         — user wants to see upcoming events
- "create"       — user wants to create a new event
- "cancel"       — admin wants to cancel an event
- "admin_remove" — admin wants to remove a specific participant (edit_value = their phone or name)
- "admin_edit"   — admin wants to edit an event field (edit_field, edit_value)
- "unknown"      — message is not related to events

For join/leave/detail/cancel, populate "event_query" with whatever identifies the event.
For create, populate: name, date, time, location, description (required), max_spots (int, optional), cost (optional).
For admin_edit: event_query, edit_field (name/date/time/location/description/max_spots/cost), edit_value.

Messages may be in Portuguese or English — handle both.

Examples:
"me coloca no karaoke de sábado"   → {"action":"join","event_query":"karaoke sábado"}
"quero sair do piquenique dia 28"  → {"action":"leave","event_query":"piquenique 28"}
"detalha o evento karaoke"         → {"action":"detail","event_query":"karaoke"}
"lista os eventos dessa semana"    → {"action":"list"}
"cancela a noite de jogos"         → {"action":"cancel","event_query":"noite de jogos"}
"cria trilha sábado 8h no parque barigui, levar água, 15 vagas, gratuito" →
  {"action":"create","name":"Trilha","date":"sábado","time":"8:00","location":"Parque Barigui","description":"levar água","max_spots":15,"cost":"gratuito"}
`

func (c *Client) ParseIntent(ctx context.Context, message string, events []EventContext) (*Intent, error) {
	contextBlock := ""
	if len(events) > 0 {
		b, _ := json.Marshal(events)
		contextBlock = fmt.Sprintf("\n\nActive events in this group:\n%s", string(b))
	}

	body, _ := json.Marshal(map[string]interface{}{
		"model":      "claude-haiku-4-5-20251001",
		"max_tokens": 512,
		"system":     systemPrompt,
		"messages": []map[string]string{
			{"role": "user", "content": message + contextBlock},
		},
	})

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var apiResp struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse API response: %w", err)
	}

	text := ""
	for _, block := range apiResp.Content {
		if block.Type == "text" {
			text += block.Text
		}
	}

	text = strings.TrimSpace(text)
	text = strings.TrimPrefix(text, "```json")
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSuffix(text, "```")
	text = strings.TrimSpace(text)

	var intent Intent
	if err := json.Unmarshal([]byte(text), &intent); err != nil {
		return nil, fmt.Errorf("failed to parse intent JSON: %w\nraw: %s", err, text)
	}

	return &intent, nil
}
