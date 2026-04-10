package handler

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/carolesr/mariazinha/internal/ai"
	"github.com/carolesr/mariazinha/internal/config"
	"github.com/carolesr/mariazinha/internal/db"
	"github.com/carolesr/mariazinha/internal/meta"
)

type Bot struct {
	db   *db.DB
	ai   *ai.Client
	meta *meta.Client
	cfg  *config.Config
}

func NewBot(database *db.DB, aiClient *ai.Client, metaClient *meta.Client, cfg *config.Config) *Bot {
	return &Bot{db: database, ai: aiClient, meta: metaClient, cfg: cfg}
}

// Handle processes an incoming message and sends a reply
func (b *Bot) Handle(ctx context.Context, groupID, senderPhone, senderName, messageID, text string) {
	// Build event context for AI
	events, _ := b.db.ListUpcomingEvents(groupID)
	evtCtx := make([]ai.EventContext, len(events))
	for i, e := range events {
		evtCtx[i] = ai.EventContext{
			ID: e.ID, Name: e.Name, Date: e.Date,
			Time: e.Time, Location: e.Location, Confirmed: e.Confirmed,
		}
		if e.MaxSpots.Valid {
			v := int(e.MaxSpots.Int64)
			evtCtx[i].MaxSpots = &v
		}
	}

	intent, err := b.ai.ParseIntent(ctx, text, evtCtx)
	if err != nil {
		log.Printf("AI error: %v", err)
		b.reply(ctx, groupID, messageID, fmtInternalError())
		return
	}

	switch intent.Action {
	case "join":
		b.handleJoin(ctx, intent, groupID, messageID, senderPhone, senderName)
	case "leave":
		b.handleLeave(ctx, intent, groupID, messageID, senderPhone, senderName)
	case "confirm_attendance":
		b.handleConfirm(ctx, intent, groupID, messageID, senderPhone, senderName)
	case "detail":
		b.handleDetail(ctx, intent, groupID, messageID)
	case "list":
		b.handleList(ctx, groupID, messageID)
	case "create":
		b.handleCreate(ctx, intent, groupID, messageID, senderPhone)
	case "cancel":
		b.handleCancel(ctx, intent, groupID, messageID, senderPhone)
	case "admin_remove":
		b.handleAdminRemove(ctx, intent, groupID, messageID, senderPhone)
	case "admin_edit":
		b.handleAdminEdit(ctx, intent, groupID, messageID, senderPhone)
	default:
		// unknown / unrelated — stay silent
	}
}

// ── Handlers ──────────────────────────────────────────────

func (b *Bot) handleJoin(ctx context.Context, intent *ai.Intent, groupID, messageID, phone, name string) {
	event := b.resolveEvent(ctx, intent, groupID, messageID)
	if event == nil {
		return
	}

	if b.db.IsParticipant(event.ID, phone) {
		b.reply(ctx, groupID, messageID, fmtAlreadyConfirmed(event.Name))
		return
	}
	if b.db.IsOnWaitlist(event.ID, phone) {
		pos := b.db.WaitlistPosition(event.ID, phone)
		b.reply(ctx, groupID, messageID, fmtAlreadyOnWaitlist(event.Name, pos))
		return
	}

	if event.MaxSpots.Valid && int64(event.Confirmed) >= event.MaxSpots.Int64 {
		b.db.AddToWaitlist(event.ID, phone, name)
		pos := b.db.WaitlistPosition(event.ID, phone)
		b.reply(ctx, groupID, messageID, fmtAddedToWaitlist(name, event.Name, pos))
		return
	}

	b.db.AddParticipant(event.ID, phone, name)

	if event.MaxSpots.Valid {
		remaining := int(event.MaxSpots.Int64) - event.Confirmed - 1
		b.reply(ctx, groupID, messageID, fmtJoinedWithSpots(name, event.Name, remaining))
	} else {
		b.reply(ctx, groupID, messageID, fmtJoined(name, event.Name))
	}
}

func (b *Bot) handleLeave(ctx context.Context, intent *ai.Intent, groupID, messageID, phone, name string) {
	event := b.resolveEvent(ctx, intent, groupID, messageID)
	if event == nil {
		return
	}

	removed, err := b.db.RemoveParticipant(event.ID, phone)
	if err != nil {
		log.Printf("remove participant: %v", err)
		b.reply(ctx, groupID, messageID, fmtInternalError())
		return
	}

	if removed {
		b.reply(ctx, groupID, messageID, fmtLeft(name, event.Name))
		b.promoteFromWaitlist(ctx, groupID, event)
		return
	}

	removedWait, _ := b.db.RemoveFromWaitlist(event.ID, phone)
	if removedWait {
		b.reply(ctx, groupID, messageID, fmtLeftWaitlist(name, event.Name))
		return
	}

	b.reply(ctx, groupID, messageID, fmtNotInEvent(event.Name))
}

func (b *Bot) handleConfirm(ctx context.Context, intent *ai.Intent, groupID, messageID, phone, name string) {
	event := b.resolveEvent(ctx, intent, groupID, messageID)
	if event == nil {
		return
	}

	if !b.db.IsParticipant(event.ID, phone) {
		b.reply(ctx, groupID, messageID, fmtNotInEvent(event.Name))
		return
	}
	if b.db.IsConfirmed(event.ID, phone) {
		b.reply(ctx, groupID, messageID, fmtAlreadyPresent(event.Name))
		return
	}

	ok, err := b.db.ConfirmParticipant(event.ID, phone)
	if err != nil {
		log.Printf("confirm participant: %v", err)
		b.reply(ctx, groupID, messageID, fmtInternalError())
		return
	}
	if !ok {
		b.reply(ctx, groupID, messageID, fmtInternalError())
		return
	}

	b.reply(ctx, groupID, messageID, fmtAttendanceConfirmed(name, event.Name))
}

func (b *Bot) handleDetail(ctx context.Context, intent *ai.Intent, groupID, messageID string) {
	event := b.resolveEvent(ctx, intent, groupID, messageID)
	if event == nil {
		return
	}
	participants, _ := b.db.ListParticipants(event.ID)
	waitlist, _ := b.db.ListWaitlist(event.ID)
	b.reply(ctx, groupID, messageID, fmtEventDetail(event, participants, waitlist))
}

func (b *Bot) handleList(ctx context.Context, groupID, messageID string) {
	events, err := b.db.ListUpcomingEvents(groupID)
	if err != nil {
		log.Printf("list events: %v", err)
		b.reply(ctx, groupID, messageID, fmtInternalError())
		return
	}
	b.reply(ctx, groupID, messageID, fmtEventList(events))
}

func (b *Bot) handleCreate(ctx context.Context, intent *ai.Intent, groupID, messageID, phone string) {
	missing := []string{}
	if intent.Name == "" {
		missing = append(missing, "nome")
	}
	if intent.Date == "" {
		missing = append(missing, "data")
	}
	if intent.Time == "" {
		missing = append(missing, "horário")
	}
	if intent.Location == "" {
		missing = append(missing, "local")
	}
	if intent.Description == "" {
		missing = append(missing, "descrição")
	}
	if len(missing) > 0 {
		b.reply(ctx, groupID, messageID, fmtMissingFields(missing))
		return
	}

	e := &db.Event{
		GroupID:     groupID,
		Name:        intent.Name,
		Date:        intent.Date,
		Time:        intent.Time,
		Location:    intent.Location,
		Description: intent.Description,
		CreatedBy:   phone,
	}
	if intent.MaxSpots != nil {
		e.MaxSpots = sql.NullInt64{Int64: int64(*intent.MaxSpots), Valid: true}
	}
	if intent.Cost != "" {
		e.Cost = sql.NullString{String: intent.Cost, Valid: true}
	}

	id, err := b.db.CreateEvent(e)
	if err != nil {
		log.Printf("create event: %v", err)
		b.reply(ctx, groupID, messageID, fmtInternalError())
		return
	}
	e.ID = id
	b.reply(ctx, groupID, messageID, fmtEventCreated(e))
}

func (b *Bot) handleCancel(ctx context.Context, intent *ai.Intent, groupID, messageID, phone string) {
	if !b.cfg.IsAdmin(phone) {
		b.reply(ctx, groupID, messageID, fmtUnauthorized())
		return
	}
	event := b.resolveEvent(ctx, intent, groupID, messageID)
	if event == nil {
		return
	}
	b.db.CancelEvent(event.ID)
	b.reply(ctx, groupID, messageID, fmtEventCancelled(event.Name))
}

func (b *Bot) handleAdminRemove(ctx context.Context, intent *ai.Intent, groupID, messageID, phone string) {
	if !b.cfg.IsAdmin(phone) {
		b.reply(ctx, groupID, messageID, fmtUnauthorized())
		return
	}
	event := b.resolveEvent(ctx, intent, groupID, messageID)
	if event == nil {
		return
	}
	target := intent.EditValue
	removed, _ := b.db.RemoveParticipant(event.ID, target)
	if !removed {
		b.db.RemoveFromWaitlist(event.ID, target)
	}
	b.reply(ctx, groupID, messageID, fmt.Sprintf("✅ Participante *%s* removida de *%s*.", target, event.Name))
	b.promoteFromWaitlist(ctx, groupID, event)
}

func (b *Bot) handleAdminEdit(ctx context.Context, intent *ai.Intent, groupID, messageID, phone string) {
	if !b.cfg.IsAdmin(phone) {
		b.reply(ctx, groupID, messageID, fmtUnauthorized())
		return
	}
	event := b.resolveEvent(ctx, intent, groupID, messageID)
	if event == nil {
		return
	}
	allowed := map[string]bool{
		"name": true, "date": true, "time": true,
		"location": true, "description": true, "max_spots": true, "cost": true,
	}
	if !allowed[intent.EditField] {
		b.reply(ctx, groupID, messageID, fmt.Sprintf("❌ Campo desconhecido: *%s*.", intent.EditField))
		return
	}
	b.db.UpdateEvent(event.ID, intent.EditField, intent.EditValue)
	b.reply(ctx, groupID, messageID, fmtEventUpdated(event.Name, intent.EditField, intent.EditValue))
}

// ── Waitlist promotion ────────────────────────────────────

func (b *Bot) promoteFromWaitlist(ctx context.Context, groupID string, event *db.Event) {
	if !event.MaxSpots.Valid {
		return
	}
	confirmed := b.db.CountConfirmed(event.ID)
	if int64(confirmed) >= event.MaxSpots.Int64 {
		return
	}
	next, err := b.db.NextOnWaitlist(event.ID)
	if err != nil || next == nil {
		return
	}
	b.db.AddParticipant(event.ID, next.Phone, next.Name)
	b.db.RemoveFromWaitlist(event.ID, next.Phone)
	b.meta.SendText(ctx, groupID, fmtPromotedFromWaitlist(next.Name, event.Name))
}

// ── Helpers ───────────────────────────────────────────────

func (b *Bot) resolveEvent(ctx context.Context, intent *ai.Intent, groupID, messageID string) *db.Event {
	var event *db.Event
	var err error

	if intent.EventID > 0 {
		event, err = b.db.GetEventByID(intent.EventID)
	} else if intent.EventQuery != "" {
		event, err = b.db.FindEvent(groupID, intent.EventQuery)
	}

	if err != nil {
		log.Printf("resolve event: %v", err)
		b.reply(ctx, groupID, messageID, fmtInternalError())
		return nil
	}
	if event == nil {
		q := intent.EventQuery
		if q == "" {
			q = fmt.Sprintf("ID %d", intent.EventID)
		}
		b.reply(ctx, groupID, messageID, fmtNotFound(q))
		return nil
	}
	return event
}

func (b *Bot) reply(ctx context.Context, to, quotedID, text string) {
	if err := b.meta.SendReply(ctx, to, text, quotedID); err != nil {
		log.Printf("send reply: %v", err)
	}
}
