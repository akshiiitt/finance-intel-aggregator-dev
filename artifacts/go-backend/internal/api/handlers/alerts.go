package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

// AlertsHandler handles all /api/alerts/* routes.
type AlertsHandler struct {
	pool *pgxpool.Pool
}

// NewAlertsHandler creates an alerts handler.
func NewAlertsHandler(pool *pgxpool.Pool) *AlertsHandler {
	return &AlertsHandler{pool: pool}
}

type alertRow struct {
	ID            int64       `json:"id"`
	Name          string      `json:"name"`
	Type          string      `json:"type"`
	Conditions    interface{} `json:"conditions"`
	IsActive      bool        `json:"isActive"`
	LastTriggered *string     `json:"lastTriggered"`
	TriggerCount  int         `json:"triggerCount"`
	CreatedAt     string      `json:"createdAt"`
}

// fetchAlert loads a single alert row by ID.
func (h *AlertsHandler) fetchAlert(c *gin.Context, id int64) (*alertRow, error) {
	var a alertRow
	var condRaw string
	err := h.pool.QueryRow(c.Request.Context(), `
		SELECT id, name, type, conditions::text, is_active,
		       to_char(last_triggered,'YYYY-MM-DD"T"HH24:MI:SS"Z"'),
		       COALESCE(trigger_count,0),
		       to_char(created_at,'YYYY-MM-DD"T"HH24:MI:SS"Z"')
		FROM alerts
		WHERE id = $1
	`, id).Scan(&a.ID, &a.Name, &a.Type, &condRaw, &a.IsActive,
		&a.LastTriggered, &a.TriggerCount, &a.CreatedAt)
	if err != nil {
		return nil, err
	}
	var cond interface{}
	_ = json.Unmarshal([]byte(condRaw), &cond)
	a.Conditions = cond
	return &a, nil
}

// ListAlerts handles GET /api/alerts
// Response mirrors Node.js exactly: { alerts: [...] }
func (h *AlertsHandler) ListAlerts(c *gin.Context) {
	rows, err := h.pool.Query(c.Request.Context(), `
		SELECT id, name, type, conditions::text, is_active,
		       to_char(last_triggered,'YYYY-MM-DD"T"HH24:MI:SS"Z"'),
		       COALESCE(trigger_count,0),
		       to_char(created_at,'YYYY-MM-DD"T"HH24:MI:SS"Z"')
		FROM alerts
		ORDER BY created_at DESC
		LIMIT 50
	`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query failed"})
		return
	}
	defer rows.Close()

	result := []alertRow{}
	for rows.Next() {
		var a alertRow
		var condRaw string
		if err := rows.Scan(&a.ID, &a.Name, &a.Type, &condRaw, &a.IsActive,
			&a.LastTriggered, &a.TriggerCount, &a.CreatedAt); err == nil {
			var cond interface{}
			_ = json.Unmarshal([]byte(condRaw), &cond)
			a.Conditions = cond
			result = append(result, a)
		}
	}

	// Node.js wraps in { alerts: [...] }
	c.JSON(http.StatusOK, gin.H{"alerts": result})
}

type AlertConditions struct {
	Keywords   []string `json:"keywords"`
	Categories []string `json:"categories"`
	MinFiScore *float64 `json:"minFiScore"`
}

// CreateAlert handles POST /api/alerts
// Returns the full created alert object (matching Node.js mapAlert shape).
func (h *AlertsHandler) CreateAlert(c *gin.Context) {
	var body struct {
		Name       string          `json:"name" binding:"required"`
		Type       string          `json:"type" binding:"required"`
		Conditions AlertConditions `json:"conditions" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate conditions
	if len(body.Conditions.Keywords) == 0 && len(body.Conditions.Categories) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "At least one of keywords or categories must be specified"})
		return
	}
	if body.Conditions.MinFiScore != nil && (*body.Conditions.MinFiScore < 0 || *body.Conditions.MinFiScore > 100) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "minFiScore must be between 0 and 100"})
		return
	}

	// S-5: Limit total active alerts to prevent DoS/flooding
	var totalAlerts int
	err := h.pool.QueryRow(c.Request.Context(), "SELECT COUNT(*) FROM alerts").Scan(&totalAlerts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to query alert count"})
		return
	}
	if totalAlerts >= 100 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Alert limit reached (maximum 100 alerts). Delete existing alerts before creating new ones."})
		return
	}

	condJSON, err := json.Marshal(body.Conditions)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid conditions shape"})
		return
	}

	var id int64
	err = h.pool.QueryRow(c.Request.Context(), `
		INSERT INTO alerts (name, type, conditions)
		VALUES ($1, $2, $3::jsonb)
		RETURNING id
	`, body.Name, body.Type, string(condJSON)).Scan(&id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "insert failed"})
		return
	}

	a, err := h.fetchAlert(c, id)
	if err != nil {
		c.JSON(http.StatusCreated, gin.H{"id": id})
		return
	}
	c.JSON(http.StatusCreated, a)
}

// PatchAlert handles PATCH /api/alerts/:id
// Mirrors Node.js exactly: only accepts isActive, returns full alert object.
func (h *AlertsHandler) PatchAlert(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid id"})
		return
	}

	var body struct {
		IsActive *bool `json:"isActive"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if body.IsActive == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No fields to update"})
		return
	}

	_, err = h.pool.Exec(c.Request.Context(), `
		UPDATE alerts SET is_active = $1 WHERE id = $2
	`, *body.IsActive, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "update failed"})
		return
	}

	// Return full updated alert — mirrors Node.js mapAlert(updated)
	a, err := h.fetchAlert(c, id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.JSON(http.StatusOK, a)
}

// DeleteAlert handles DELETE /api/alerts/:id
// Returns { success: true } matching Node.js.
func (h *AlertsHandler) DeleteAlert(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid id"})
		return
	}

	_, err = h.pool.Exec(c.Request.Context(), `DELETE FROM alerts WHERE id = $1`, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "delete failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// GetGlobalTriggers handles GET /api/alerts/triggers
// Returns the 50 most recent triggers across all alerts.
// Response shape mirrors Node.js exactly: { triggers: [{id, alertId, articleId, title, source, category, fiScore, triggeredAt}] }
func (h *AlertsHandler) GetGlobalTriggers(c *gin.Context) {
	rows, err := h.pool.Query(c.Request.Context(), `
		SELECT id, alert_id, article_id, title, source, category,
		       fi_score::float8,
		       to_char(triggered_at,'YYYY-MM-DD"T"HH24:MI:SS"Z"')
		FROM alert_triggers
		ORDER BY triggered_at DESC
		LIMIT 50
	`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query failed"})
		return
	}
	defer rows.Close()

	type triggerRow struct {
		ID          int64    `json:"id"`
		AlertID     int64    `json:"alertId"`
		ArticleID   int64    `json:"articleId"`
		Title       string   `json:"title"`
		Source      *string  `json:"source"`
		Category    *string  `json:"category"`
		FiScore     *float64 `json:"fiScore"`
		TriggeredAt string   `json:"triggeredAt"`
	}

	result := []triggerRow{}
	for rows.Next() {
		var r triggerRow
		if err := rows.Scan(&r.ID, &r.AlertID, &r.ArticleID, &r.Title,
			&r.Source, &r.Category, &r.FiScore, &r.TriggeredAt); err == nil {
			result = append(result, r)
		}
	}

	c.JSON(http.StatusOK, gin.H{"triggers": result})
}

// GetAlertTriggers handles GET /api/alerts/:id/triggers
// Returns per-alert triggers (Go-only bonus endpoint, not in Node.js).
func (h *AlertsHandler) GetAlertTriggers(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	limit := intQuery(c, "limit", 20)
	if limit > 100 {
		limit = 100
	}
	rows, err := h.pool.Query(c.Request.Context(), `
		SELECT id, alert_id, article_id, title, source, category,
		       fi_score::float8,
		       to_char(triggered_at,'YYYY-MM-DD"T"HH24:MI:SS"Z"')
		FROM alert_triggers
		WHERE alert_id = $1
		ORDER BY triggered_at DESC
		LIMIT $2
	`, id, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query failed"})
		return
	}
	defer rows.Close()

	type triggerRow struct {
		ID          int64    `json:"id"`
		AlertID     int64    `json:"alertId"`
		ArticleID   int64    `json:"articleId"`
		Title       string   `json:"title"`
		Source      *string  `json:"source"`
		Category    *string  `json:"category"`
		FiScore     *float64 `json:"fiScore"`
		TriggeredAt string   `json:"triggeredAt"`
	}
	result := []triggerRow{}
	for rows.Next() {
		var r triggerRow
		if err := rows.Scan(&r.ID, &r.AlertID, &r.ArticleID, &r.Title, &r.Source, &r.Category,
			&r.FiScore, &r.TriggeredAt); err == nil {
			result = append(result, r)
		}
	}
	c.JSON(http.StatusOK, gin.H{"triggers": result})
}

// GetRecentTriggers handles GET /api/alerts/triggers/recent
// Returns triggers from the last 24 hours with alertName (Go bonus endpoint).
func (h *AlertsHandler) GetRecentTriggers(c *gin.Context) {
	since := time.Now().Add(-24 * time.Hour)
	rows, err := h.pool.Query(c.Request.Context(), `
		SELECT t.id, t.alert_id, a.name as alert_name, t.article_id, t.title,
		       t.source, t.category, t.fi_score::float8,
		       to_char(t.triggered_at,'YYYY-MM-DD"T"HH24:MI:SS"Z"')
		FROM alert_triggers t
		JOIN alerts a ON a.id = t.alert_id
		WHERE t.triggered_at >= $1
		ORDER BY t.triggered_at DESC
		LIMIT 50
	`, since)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query failed"})
		return
	}
	defer rows.Close()

	type triggerRow struct {
		ID          int64    `json:"id"`
		AlertID     int64    `json:"alertId"`
		AlertName   string   `json:"alertName"`
		ArticleID   int64    `json:"articleId"`
		Title       string   `json:"title"`
		Source      *string  `json:"source"`
		Category    *string  `json:"category"`
		FiScore     *float64 `json:"fiScore"`
		TriggeredAt string   `json:"triggeredAt"`
	}
	result := []triggerRow{}
	for rows.Next() {
		var r triggerRow
		if err := rows.Scan(&r.ID, &r.AlertID, &r.AlertName, &r.ArticleID, &r.Title,
			&r.Source, &r.Category, &r.FiScore, &r.TriggeredAt); err == nil {
			result = append(result, r)
		}
	}
	c.JSON(http.StatusOK, result)
}
