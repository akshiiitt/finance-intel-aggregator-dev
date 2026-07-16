-- name: GetIPOs :many
SELECT id, company_name, exchange,
       price_band_low, price_band_high, lot_size,
       open_date, close_date, listing_date,
       issue_size_cr, gmp, subscription_x,
       status, sector, updated_at
FROM ipo_calendar
ORDER BY
    CASE status
        WHEN 'open'     THEN 1
        WHEN 'upcoming' THEN 2
        WHEN 'closed'   THEN 3
        ELSE 4
    END,
    open_date DESC NULLS LAST
LIMIT $1 OFFSET $2;

-- name: GetIPOsByStatus :many
SELECT id, company_name, exchange,
       price_band_low, price_band_high, lot_size,
       open_date, close_date, listing_date,
       issue_size_cr, gmp, subscription_x,
       status, sector, updated_at
FROM ipo_calendar
WHERE status = $1
ORDER BY open_date DESC NULLS LAST, updated_at DESC
LIMIT $2 OFFSET $3;

-- name: CreateIPO :one
INSERT INTO ipo_calendar (
    company_name, exchange, price_band_low, price_band_high, lot_size,
    open_date, close_date, listing_date, issue_size_cr, gmp, subscription_x, status, sector
) VALUES (
    $1, $2, $3, $4, $5,
    $6, $7, $8, $9, $10, $11, $12, $13
) RETURNING id;

-- name: DeleteIPO :exec
DELETE FROM ipo_calendar WHERE id = $1;

-- name: UpdateIPOStatus :exec
UPDATE ipo_calendar SET status = $2, updated_at = NOW() WHERE id = $1;

-- name: UpdateIPOGMP :exec
UPDATE ipo_calendar SET gmp = $2, subscription_x = $3, updated_at = NOW() WHERE id = $1;
