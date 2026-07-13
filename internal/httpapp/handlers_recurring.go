package httpapp

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/hafio/gosplit/internal/store"
)

type recurrenceRow struct {
	ID           int64
	Cron         string
	TemplateName string
	NextRun      string
}

type templateOption struct {
	ID   string
	Name string
}

func (s *Server) handleRecurringList(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	me := s.currentUser(r)
	recs, err := s.Svc.ListRecurrences(ctx, me.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	rows := make([]recurrenceRow, 0, len(recs))
	for _, rec := range recs {
		name := rec.TemplateExpenseID
		if e, err := s.Store.GetExpense(ctx, rec.TemplateExpenseID); err == nil {
			name = e.Name
		}
		next := ""
		if rec.NextRunAt.Valid {
			next = rec.NextRunAt.String
		}
		rows = append(rows, recurrenceRow{ID: rec.ID, Cron: rec.CronExpression, TemplateName: name, NextRun: next})
	}

	// Offer the user's recent non-conversion expenses as templates.
	recent, _ := s.Store.ListActivity(ctx, me.ID, store.ExpenseFilter{})
	opts := make([]templateOption, 0, 20)
	for _, e := range recent {
		if e.DeletedAt.Valid || e.SplitType == "CURRENCY_CONVERSION" || e.ConversionToID.Valid {
			continue
		}
		opts = append(opts, templateOption{ID: e.ID, Name: e.Name + " (" + dateOnly(e.ExpenseDate) + ")"})
		if len(opts) >= 20 {
			break
		}
	}

	s.render(w, r, "recurring", "title.recurring", map[string]any{"Recurrences": rows, "Templates": opts})
}

func (s *Server) handleRecurringCreate(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	me := s.currentUser(r)
	tmplID := r.FormValue("template_id")
	cronExpr := r.FormValue("cron")
	if _, err := s.Svc.CreateRecurrence(ctx, me.ID, tmplID, cronExpr); err != nil {
		s.renderErr(w, r, "message", "msg.recurrence_failed", err.Error(), http.StatusBadRequest, err.Error())
		return
	}
	s.redirectFlash(w, r, "/recurring", "flash.recurrence_scheduled")
}

func (s *Server) handleRecurringDelete(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	me := s.currentUser(r)
	id, _ := atoi64(chi.URLParam(r, "id"))
	_ = s.Svc.DeleteRecurrence(ctx, id, me.ID)
	http.Redirect(w, r, "/recurring", http.StatusSeeOther)
}
