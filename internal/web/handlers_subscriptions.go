package web

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/bhaskarjha-com/niyantra/internal/readiness"
	"github.com/bhaskarjha-com/niyantra/internal/store"
)

// getSubscriptionByID extracts the path parameter and dispatches to getSubscription.
func (s *Server) getSubscriptionByID(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonError(w, "invalid subscription ID", http.StatusBadRequest)
		return
	}
	s.getSubscription(w, id)
}

// updateSubscriptionByID extracts the path parameter and dispatches to updateSubscription.
func (s *Server) updateSubscriptionByID(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonError(w, "invalid subscription ID", http.StatusBadRequest)
		return
	}
	s.updateSubscription(w, r, id)
}

// deleteSubscriptionByID extracts the path parameter and dispatches to deleteSubscription.
func (s *Server) deleteSubscriptionByID(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonError(w, "invalid subscription ID", http.StatusBadRequest)
		return
	}
	s.deleteSubscription(w, id)
}

func (s *Server) listSubscriptions(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	category := r.URL.Query().Get("category")

	subs, err := s.store.ListSubscriptions(status, category)
	if err != nil {
		s.logger.Error("list subscriptions failed", "error", err)
		jsonError(w, "database error", http.StatusInternalServerError)
		return
	}
	if subs == nil {
		subs = []*store.Subscription{} // ensure JSON array, not null
	}

	// Compute days-until-renewal for each
	type subResponse struct {
		*store.Subscription
		DaysUntilRenewal  *int `json:"daysUntilRenewal,omitempty"`
		DaysUntilTrialEnd *int `json:"daysUntilTrialEnd,omitempty"`
	}

	var items []subResponse
	now := time.Now()
	for _, sub := range subs {
		item := subResponse{Subscription: sub}
		if sub.NextRenewal != "" {
			if t, err := time.Parse("2006-01-02", sub.NextRenewal); err == nil {
				days := int(math.Ceil(t.Sub(now).Hours() / 24))
				item.DaysUntilRenewal = &days
			}
		}
		if sub.TrialEndsAt != "" {
			if t, err := time.Parse("2006-01-02", sub.TrialEndsAt); err == nil {
				days := int(math.Ceil(t.Sub(now).Hours() / 24))
				item.DaysUntilTrialEnd = &days
			}
		}
		items = append(items, item)
	}

	writeJSON(w, map[string]interface{}{
		"subscriptions": items,
		"count":         len(items),
	})
}

func (s *Server) createSubscription(w http.ResponseWriter, r *http.Request) {
	var sub store.Subscription
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&sub); err != nil {
		jsonError(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	if sub.Platform == "" {
		jsonError(w, "platform is required", http.StatusBadRequest)
		return
	}

	// Apply defaults
	if sub.Status == "" {
		sub.Status = "active"
	}
	if sub.CostCurrency == "" {
		sub.CostCurrency = "USD"
	}
	if sub.BillingCycle == "" {
		sub.BillingCycle = "monthly"
	}
	if sub.Category == "" {
		sub.Category = "other"
	}
	if sub.LimitPeriod == "" {
		sub.LimitPeriod = "monthly"
	}

	id, err := s.store.InsertSubscription(&sub)
	if err != nil {
		s.logger.Error("create subscription failed", "error", err)
		jsonError(w, "database error", http.StatusInternalServerError)
		return
	}

	sub.ID = id
	s.logger.Info("subscription created", "id", id, "platform", sub.Platform)

	w.WriteHeader(http.StatusCreated)
	writeJSON(w, sub)
}

func (s *Server) getSubscription(w http.ResponseWriter, id int64) {
	sub, err := s.store.GetSubscription(id)
	if err != nil {
		s.logger.Error("get subscription failed", "error", err)
		jsonError(w, "database error", http.StatusInternalServerError)
		return
	}
	if sub == nil {
		jsonError(w, "subscription not found", http.StatusNotFound)
		return
	}
	writeJSON(w, sub)
}

func (s *Server) updateSubscription(w http.ResponseWriter, r *http.Request, id int64) {
	var sub store.Subscription
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&sub); err != nil {
		jsonError(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	sub.ID = id

	if sub.Platform == "" {
		jsonError(w, "platform is required", http.StatusBadRequest)
		return
	}

	// Check exists
	existing, err := s.store.GetSubscription(id)
	if err != nil || existing == nil {
		jsonError(w, "subscription not found", http.StatusNotFound)
		return
	}

	if err := s.store.UpdateSubscription(&sub); err != nil {
		s.logger.Error("update subscription failed", "error", err)
		jsonError(w, "database error", http.StatusInternalServerError)
		return
	}

	s.logger.Info("subscription updated", "id", id, "platform", sub.Platform)
	writeJSON(w, sub)
}

func (s *Server) deleteSubscription(w http.ResponseWriter, id int64) {
	existing, err := s.store.GetSubscription(id)
	if err != nil || existing == nil {
		jsonError(w, "subscription not found", http.StatusNotFound)
		return
	}

	if err := s.store.DeleteSubscription(id); err != nil {
		s.logger.Error("delete subscription failed", "error", err)
		jsonError(w, "database error", http.StatusInternalServerError)
		return
	}

	s.logger.Info("subscription deleted", "id", id, "platform", existing.Platform)
	writeJSON(w, map[string]string{"message": "deleted"})
}

// handleOverview returns unified stats: spending, renewals, and auto-tracked summary.
func (s *Server) handleOverview(w http.ResponseWriter, r *http.Request) {
	// Subscription stats
	stats, err := s.store.SubscriptionOverview()
	if err != nil {
		s.logger.Error("overview stats failed", "error", err)
		jsonError(w, "database error", http.StatusInternalServerError)
		return
	}

	// Upcoming renewals
	renewals, err := s.store.UpcomingRenewals(10)
	if err != nil {
		renewals = nil
	}
	if renewals == nil {
		renewals = []*store.Subscription{}
	}

	// Renewal response with days
	type renewalItem struct {
		ID          int64   `json:"id"`
		Platform    string  `json:"platform"`
		NextRenewal string  `json:"nextRenewal"`
		CostAmount  float64 `json:"costAmount"`
		DaysUntil   int     `json:"daysUntil"`
	}
	var renewalItems []renewalItem
	now := time.Now()
	for _, r := range renewals {
		days := 0
		if t, err := time.Parse("2006-01-02", r.NextRenewal); err == nil {
			days = int(math.Ceil(t.Sub(now).Hours() / 24))
		}
		renewalItems = append(renewalItems, renewalItem{
			ID:          r.ID,
			Platform:    r.Platform,
			NextRenewal: r.NextRenewal,
			CostAmount:  r.CostAmount,
			DaysUntil:   days,
		})
	}
	if renewalItems == nil {
		renewalItems = []renewalItem{}
	}

	// Auto-tracked quota summary
	snapshots, _ := s.store.LatestPerAccount()
	var quotaSummary interface{}
	if len(snapshots) > 0 {
		quotaSummary = readiness.Calculate(snapshots, 0.0)
	}

	// Quick links from all subscriptions
	allSubs, _ := s.store.ListSubscriptions("", "")
	type quickLink struct {
		Platform string `json:"platform"`
		URL      string `json:"url"`
		Category string `json:"category"`
	}
	var links []quickLink
	for _, sub := range allSubs {
		if sub.URL != "" {
			links = append(links, quickLink{
				Platform: sub.Platform,
				URL:      sub.URL,
				Category: sub.Category,
			})
		}
	}
	if links == nil {
		links = []quickLink{}
	}

	// Phase 10: Server-computed insights
	insights, _ := s.store.GenerateInsights()
	if insights == nil {
		insights = []store.Insight{}
	}

	writeJSON(w, map[string]interface{}{
		"stats":             stats,
		"renewals":          renewalItems,
		"quotaSummary":      quotaSummary,
		"quickLinks":        links,
		"insights":          insights,
		"subscriptionCount": s.store.SubscriptionCount(),
	})
}

// handlePresets returns platform preset templates.
func (s *Server) handlePresets(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]interface{}{
		"presets": store.Presets,
	})
}

// handleExportCSV exports subscriptions as CSV for expense/tax reports.
func (s *Server) handleExportCSV(w http.ResponseWriter, r *http.Request) {

	subs, err := s.store.ListSubscriptions("", "")
	if err != nil {
		s.logger.Error("export CSV failed", "error", err)
		jsonError(w, "database error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition",
		fmt.Sprintf("attachment; filename=niyantra-subscriptions-%s.csv",
			time.Now().Format("2006-01-02")))

	writer := csv.NewWriter(w)
	defer writer.Flush()

	// Header row
	// TODO(audit#23): Add Token Limit, Credit Limit, Request Limit columns
	// for Cursor USD credit billing and Copilot request-based quota systems.
	writer.Write([]string{
		"Platform", "Category", "Plan", "Status",
		"Monthly Cost", "Currency", "Billing Cycle",
		"Annual Cost", "Email", "Next Renewal",
		"Notes", "Dashboard URL",
	})

	for _, sub := range subs {
		monthly := store.ToMonthlyExported(sub.CostAmount, sub.BillingCycle)
		annual := monthly * 12

		writer.Write([]string{
			sub.Platform, sub.Category, sub.PlanName, sub.Status,
			fmt.Sprintf("%.2f", monthly), sub.CostCurrency, sub.BillingCycle,
			fmt.Sprintf("%.2f", annual), sub.Email, sub.NextRenewal,
			sub.Notes, sub.URL,
		})
	}
}
