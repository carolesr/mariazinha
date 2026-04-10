package webhook

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/carolesr/mariazinha/internal/config"
	"github.com/carolesr/mariazinha/internal/handler"
)

type Server struct {
	bot *handler.Bot
	cfg *config.Config
}

func NewServer(bot *handler.Bot, cfg *config.Config) *Server {
	return &Server{bot: bot, cfg: cfg}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/webhook", s.handleWebhook)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	return mux
}

// handleWebhook handles both GET (verification) and POST (incoming messages)
func (s *Server) handleWebhook(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.verify(w, r)
	case http.MethodPost:
		s.receive(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// verify handles the one-time webhook verification from Meta
func (s *Server) verify(w http.ResponseWriter, r *http.Request) {
	mode := r.URL.Query().Get("hub.mode")
	token := r.URL.Query().Get("hub.verify_token")
	challenge := r.URL.Query().Get("hub.challenge")

	if mode == "subscribe" && token == s.cfg.MetaVerifyToken {
		log.Println("✅ webhook verified by Meta")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(challenge))
		return
	}

	log.Println("⚠️  webhook verification failed — check META_VERIFY_TOKEN")
	http.Error(w, "forbidden", http.StatusForbidden)
}

// receive handles incoming message notifications from Meta
func (s *Server) receive(w http.ResponseWriter, r *http.Request) {
	// Always respond 200 quickly — Meta will retry if we don't
	w.WriteHeader(http.StatusOK)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("read body: %v", err)
		return
	}

	var payload WhatsAppPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		log.Printf("unmarshal payload: %v", err)
		return
	}

	ctx := context.Background()

	for _, entry := range payload.Entry {
		for _, change := range entry.Changes {
			if change.Field != "messages" {
				continue
			}
			for _, msg := range change.Value.Messages {
				if msg.Type != "text" {
					continue
				}

				text := msg.Text.Body
				groupID := change.Value.Metadata.PhoneNumberID // group context
				senderPhone := msg.From
				messageID := msg.ID

				// Resolve sender name from contacts
				senderName := senderPhone
				for _, c := range change.Value.Contacts {
					if c.WaID == senderPhone {
						senderName = c.Profile.Name
						break
					}
				}

				// Only respond if the bot is @mentioned
				if !isMentioned(text, s.cfg.BotName, s.cfg.MetaPhoneID) {
					continue
				}

				clean := stripMention(text, s.cfg.BotName, s.cfg.MetaPhoneID)

				go s.bot.Handle(ctx, groupID, senderPhone, senderName, messageID, clean)
			}
		}
	}
}

func isMentioned(text, botName, phoneID string) bool {
	lower := strings.ToLower(text)
	return strings.Contains(lower, "@"+strings.ToLower(botName)) ||
		strings.Contains(lower, phoneID)
}

func stripMention(text, botName, phoneID string) string {
	text = strings.ReplaceAll(text, "@"+botName, "")
	text = strings.ReplaceAll(text, "@"+strings.ToLower(botName), "")
	text = strings.ReplaceAll(text, phoneID, "")
	return strings.TrimSpace(text)
}

// ── Meta webhook payload structs ──────────────────────────

type WhatsAppPayload struct {
	Object string  `json:"object"`
	Entry  []Entry `json:"entry"`
}

type Entry struct {
	ID      string   `json:"id"`
	Changes []Change `json:"changes"`
}

type Change struct {
	Field string      `json:"field"`
	Value ChangeValue `json:"value"`
}

type ChangeValue struct {
	MessagingProduct string    `json:"messaging_product"`
	Metadata         Metadata  `json:"metadata"`
	Contacts         []Contact `json:"contacts"`
	Messages         []Message `json:"messages"`
}

type Metadata struct {
	DisplayPhoneNumber string `json:"display_phone_number"`
	PhoneNumberID      string `json:"phone_number_id"`
}

type Contact struct {
	Profile Profile `json:"profile"`
	WaID    string  `json:"wa_id"`
}

type Profile struct {
	Name string `json:"name"`
}

type Message struct {
	From string      `json:"from"`
	ID   string      `json:"id"`
	Type string      `json:"type"`
	Text MessageText `json:"text"`
}

type MessageText struct {
	Body string `json:"body"`
}
