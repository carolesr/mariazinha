package handler

import (
	"fmt"
	"strings"

	"github.com/carolesr/mariazinha/internal/db"
)

func fmtEventDetail(e *db.Event, participants, waitlist []*db.Participant) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("📅 *%s*\n", e.Name))
	sb.WriteString(fmt.Sprintf("🗓  %s às %s\n", e.Date, e.Time))
	sb.WriteString(fmt.Sprintf("📍 %s\n", e.Location))
	sb.WriteString(fmt.Sprintf("📝 %s\n", e.Description))

	if e.Cost.Valid && e.Cost.String != "" {
		sb.WriteString(fmt.Sprintf("💰 %s\n", e.Cost.String))
	}

	spotsLine := "vagas ilimitadas"
	if e.MaxSpots.Valid {
		remaining := int(e.MaxSpots.Int64) - len(participants)
		spotsLine = fmt.Sprintf("%d/%d vagas", len(participants), e.MaxSpots.Int64)
		if remaining <= 0 {
			spotsLine += " (lotado)"
		}
	}
	sb.WriteString(fmt.Sprintf("👥 %s\n", spotsLine))

	if len(participants) > 0 {
		sb.WriteString("\n*Confirmadas:*\n")
		for i, p := range participants {
			sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, p.Name))
		}
	} else {
		sb.WriteString("\nNenhuma confirmação ainda.\n")
	}

	if len(waitlist) > 0 {
		sb.WriteString("\n*Lista de espera:*\n")
		for i, p := range waitlist {
			sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, p.Name))
		}
	}

	return strings.TrimSpace(sb.String())
}

func fmtEventSummary(e *db.Event) string {
	spotsInfo := "vagas ilimitadas"
	if e.MaxSpots.Valid {
		spotsInfo = fmt.Sprintf("%d/%d vagas", e.Confirmed, e.MaxSpots.Int64)
	} else {
		spotsInfo = fmt.Sprintf("%d confirmada(s)", e.Confirmed)
	}

	cost := ""
	if e.Cost.Valid && e.Cost.String != "" {
		cost = fmt.Sprintf(" · %s", e.Cost.String)
	}

	return fmt.Sprintf("🔹 *%s* [ID: %d]\n   %s às %s · %s · %s%s",
		e.Name, e.ID, e.Date, e.Time, e.Location, spotsInfo, cost)
}

func fmtEventList(events []*db.Event) string {
	if len(events) == 0 {
		return "📭 Nenhum evento programado no momento."
	}
	var sb strings.Builder
	sb.WriteString("📅 *Próximos eventos:*\n\n")
	for _, e := range events {
		sb.WriteString(fmtEventSummary(e))
		sb.WriteString("\n\n")
	}
	sb.WriteString("Para participar, me mencione e diga em qual evento quer entrar!")
	return strings.TrimSpace(sb.String())
}

func fmtJoined(name, eventName string) string {
	return fmt.Sprintf("✅ *%s* confirmada em *%s*!", name, eventName)
}

func fmtJoinedWithSpots(name, eventName string, remaining int) string {
	return fmt.Sprintf("✅ *%s* confirmada em *%s*!\n👥 %d vaga(s) restante(s).", name, eventName, remaining)
}

func fmtAddedToWaitlist(name, eventName string, position int) string {
	return fmt.Sprintf("⚠️ *%s* está lotado.\n⏳ *%s* entrou na lista de espera na posição *%d*.", eventName, name, position)
}

func fmtLeft(name, eventName string) string {
	return fmt.Sprintf("❎ *%s* removida de *%s*.", name, eventName)
}

func fmtLeftWaitlist(name, eventName string) string {
	return fmt.Sprintf("❎ *%s* removida da lista de espera de *%s*.", name, eventName)
}

func fmtPromotedFromWaitlist(name, eventName string) string {
	return fmt.Sprintf("🎉 @%s — abriu uma vaga em *%s*! Você saiu da lista de espera e está confirmada!", name, eventName)
}

func fmtNotFound(query string) string {
	return fmt.Sprintf("❌ Nenhum evento encontrado para *\"%s\"*.\nUse \"lista os eventos\" para ver o que está disponível.", query)
}

func fmtEventCreated(e *db.Event) string {
	spots := "vagas ilimitadas"
	if e.MaxSpots.Valid {
		spots = fmt.Sprintf("%d vagas", e.MaxSpots.Int64)
	}
	cost := ""
	if e.Cost.Valid && e.Cost.String != "" {
		cost = fmt.Sprintf("\n💰 %s", e.Cost.String)
	}
	return fmt.Sprintf(
		"✅ *Evento criado!*\n\n📅 *%s* [ID: %d]\n🗓  %s às %s\n📍 %s\n📝 %s\n👥 %s%s",
		e.Name, e.ID, e.Date, e.Time, e.Location, e.Description, spots, cost,
	)
}

func fmtEventCancelled(eventName string) string {
	return fmt.Sprintf("🚫 Evento *%s* foi cancelado.", eventName)
}

func fmtEventUpdated(eventName, field, value string) string {
	return fmt.Sprintf("✏️ *%s* atualizado: %s → *%s*", eventName, field, value)
}

func fmtAlreadyConfirmed(eventName string) string {
	return fmt.Sprintf("ℹ️ Você já está confirmada em *%s*.", eventName)
}

func fmtAlreadyOnWaitlist(eventName string, position int) string {
	return fmt.Sprintf("ℹ️ Você já está na lista de espera de *%s* na posição *%d*.", eventName, position)
}

func fmtNotInEvent(eventName string) string {
	return fmt.Sprintf("ℹ️ Você não está inscrita em *%s*.", eventName)
}

func fmtUnauthorized() string {
	return "❌ Você não tem permissão para usar comandos de administração."
}

func fmtMissingFields(fields []string) string {
	return fmt.Sprintf("❌ Faltam informações obrigatórias: %s.\nPor favor, forneça todos os dados necessários.", strings.Join(fields, ", "))
}

func fmtInternalError() string {
	return "❌ Ocorreu um erro interno. Por favor, tente novamente."
}
