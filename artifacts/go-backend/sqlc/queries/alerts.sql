-- name: ListAlerts :many
SELECT id, name, type, conditions, is_active,
       last_triggered, trigger_count, created_at
FROM alerts
ORDER BY created_at DESC
LIMIT 100;

-- name: GetAlertByID :one
SELECT id, name, type, conditions, is_active,
       last_triggered, trigger_count, created_at
FROM alerts WHERE id = $1;

-- name: CreateAlert :one
INSERT INTO alerts (name, type, conditions)
VALUES ($1, $2, $3::jsonb)
RETURNING id;

-- name: UpdateAlertActive :exec
UPDATE alerts SET is_active = $2 WHERE id = $1;

-- name: DeleteAlert :exec
DELETE FROM alerts WHERE id = $1;

-- name: GetAllActiveAlerts :many
SELECT id, name, type, conditions, is_active,
       last_triggered, trigger_count, created_at
FROM alerts WHERE is_active = TRUE;

-- name: GetAlertTriggers :many
SELECT id, alert_id, article_id, title, source, category, fi_score, triggered_at
FROM alert_triggers
WHERE alert_id = $1
ORDER BY triggered_at DESC
LIMIT $2;

-- name: GetRecentTriggers :many
SELECT t.id, t.alert_id, a.name AS alert_name,
       t.article_id, t.title, t.source, t.category, t.fi_score, t.triggered_at
FROM alert_triggers t
JOIN alerts a ON a.id = t.alert_id
WHERE t.triggered_at >= $1
ORDER BY t.triggered_at DESC
LIMIT 50;

-- name: InsertAlertTrigger :exec
INSERT INTO alert_triggers (alert_id, article_id, title, source, category, fi_score)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: UpdateAlertTriggerStats :exec
UPDATE alerts
SET last_triggered = NOW(),
    trigger_count  = COALESCE(trigger_count, 0) + 1
WHERE id = $1;
