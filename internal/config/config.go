package config

import (
	"log"
	"os"
	"strings"
)

type Config struct {
	// Meta WhatsApp Cloud API
	MetaAccessToken  string
	MetaPhoneID      string
	MetaVerifyToken  string // any string you choose, used to verify the webhook with Meta

	// Anthropic
	AnthropicAPIKey string

	// Bot
	BotName     string
	AdminPhones []string // comma-separated international format: 5541999887766

	// Server
	Port string

	// SQLite
	EventsDBPath string
}

func Load() *Config {
	cfg := &Config{
		MetaAccessToken: requireEnv("META_ACCESS_TOKEN"),
		MetaPhoneID:     requireEnv("META_PHONE_ID"),
		MetaVerifyToken: requireEnv("META_VERIFY_TOKEN"),
		AnthropicAPIKey: requireEnv("ANTHROPIC_API_KEY"),
		BotName:         getEnv("BOT_NAME", "Mariazinha"),
		Port:            getEnv("PORT", "8080"),
		EventsDBPath:    getEnv("EVENTS_DB_PATH", "./data/events.db"),
	}

	admins := getEnv("ADMIN_PHONES", "")
	if admins != "" {
		for _, p := range strings.Split(admins, ",") {
			cfg.AdminPhones = append(cfg.AdminPhones, strings.TrimSpace(p))
		}
	}

	return cfg
}

func (c *Config) IsAdmin(phone string) bool {
	for _, a := range c.AdminPhones {
		if a == phone {
			return true
		}
	}
	return false
}

func requireEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("required environment variable %s is not set", key)
	}
	return v
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
