-- name: GetLatestMarketSnapshots :many
SELECT DISTINCT ON (symbol)
       id, symbol, name, exchange,
       price, change_pct, change_abs, prev_close,
       captured_at
FROM market_snapshots
ORDER BY symbol, captured_at DESC;

-- name: GetSymbolHistory :many
SELECT id, symbol, name, exchange,
       price, change_pct, change_abs, prev_close,
       captured_at
FROM market_snapshots
WHERE symbol = $1 AND captured_at >= $2
ORDER BY captured_at ASC
LIMIT 1000;

-- name: InsertMarketSnapshot :one
INSERT INTO market_snapshots (symbol, name, exchange, price, change_pct, change_abs, prev_close)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id;
