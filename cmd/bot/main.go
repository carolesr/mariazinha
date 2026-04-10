package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"github.com/carolesr/mariazinha/internal/ai"
	"github.com/carolesr/mariazinha/internal/config"
	"github.com/carolesr/mariazinha/internal/db"
	"github.com/carolesr/mariazinha/internal/handler"
	"github.com/carolesr/mariazinha/internal/meta"
	"github.com/carolesr/mariazinha/internal/webhook"
)

func main() {
	godotenv.Load()

	cfg := config.Load()

	database := db.Init(cfg.EventsDBPath)
	defer database.Close()

	aiClient := ai.NewClient(cfg.AnthropicAPIKey)
	metaClient := meta.NewClient(cfg.MetaPhoneID, cfg.MetaAccessToken)
	bot := handler.NewBot(database, aiClient, metaClient, cfg)
	srv := webhook.NewServer(bot, cfg)

	httpServer := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: srv.Handler(),
	}

	go func() {
		log.Printf("🤖 %s listening on port %s", cfg.BotName, cfg.Port)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	log.Println("shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	httpServer.Shutdown(ctx)
}
