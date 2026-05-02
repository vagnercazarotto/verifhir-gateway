package sqlite

import (
	"context"
	"fmt"
	"time"

	"github.com/vagnercazarotto/verifhir-gateway/internal/store"
)

// Summary computes aggregate message statistics between from and to (RFC3339,
// empty = unbounded). It satisfies the store.Reporter interface.
func (s *Store) Summary(ctx context.Context, from, to string) (*store.ReportSummary, error) {
	var fromT, toT time.Time
	var err error
	if from != "" {
		fromT, err = time.Parse(time.RFC3339, from)
		if err != nil {
			return nil, fmt.Errorf("sqlite: invalid from: %w", err)
		}
	}
	if to != "" {
		toT, err = time.Parse(time.RFC3339, to)
		if err != nil {
			return nil, fmt.Errorf("sqlite: invalid to: %w", err)
		}
	}

	// Build a parameterised WHERE clause. Values never go into the SQL string.
	where := "1=1"
	args := []any{}
	if !fromT.IsZero() {
		where += " AND created_at >= ?"
		args = append(args, fromT.UTC().Format(time.RFC3339))
	}
	if !toT.IsZero() {
		where += " AND created_at <= ?"
		args = append(args, toT.UTC().Format(time.RFC3339))
	}

	// ---- totals ----
	var total int
	var avgScore float64
	row := s.db.QueryRowContext(ctx,
		//nolint:gosec // where clause built from static strings; args are parameterised
		`SELECT COUNT(*), COALESCE(AVG(quality_score), 0) FROM messages WHERE `+where,
		args...)
	if err := row.Scan(&total, &avgScore); err != nil {
		return nil, fmt.Errorf("sqlite: summary totals: %w", err)
	}

	// ---- by status ----
	statusRows, err := s.db.QueryContext(ctx,
		//nolint:gosec // same as above
		`SELECT status, COUNT(*) FROM messages WHERE `+where+` GROUP BY status`,
		args...)
	if err != nil {
		return nil, fmt.Errorf("sqlite: summary by status: %w", err)
	}
	defer statusRows.Close()
	byStatus := map[string]int{}
	for statusRows.Next() {
		var st string
		var cnt int
		if err := statusRows.Scan(&st, &cnt); err != nil {
			continue
		}
		byStatus[st] = cnt
	}

	// ---- by day ----
	dayRows, err := s.db.QueryContext(ctx,
		//nolint:gosec // same as above
		`SELECT
			substr(created_at, 1, 10)              AS day,
			COUNT(*)                               AS total,
			COALESCE(AVG(quality_score), 0)        AS avg_score,
			SUM(CASE WHEN status = 'sent'          THEN 1 ELSE 0 END),
			SUM(CASE WHEN status = 'failed'        THEN 1 ELSE 0 END),
			SUM(CASE WHEN status = 'dead_lettered' THEN 1 ELSE 0 END),
			SUM(CASE WHEN status = 'pending'       THEN 1 ELSE 0 END)
		 FROM messages WHERE `+where+`
		 GROUP BY day
		 ORDER BY day ASC`,
		args...)
	if err != nil {
		return nil, fmt.Errorf("sqlite: summary by day: %w", err)
	}
	defer dayRows.Close()
	var byDay []store.DaySummary
	for dayRows.Next() {
		var ds store.DaySummary
		if err := dayRows.Scan(
			&ds.Date, &ds.Total, &ds.AvgScore,
			&ds.Sent, &ds.Failed, &ds.DeadLettered, &ds.Pending,
		); err != nil {
			continue
		}
		byDay = append(byDay, ds)
	}
	if err := dayRows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: summary by day rows: %w", err)
	}

	return &store.ReportSummary{
		From:     from,
		To:       to,
		Total:    total,
		AvgScore: avgScore,
		ByStatus: byStatus,
		ByDay:    byDay,
	}, nil
}
