package rules

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

func RecordMatch(ctx context.Context, db *sql.DB, ruleID int64) error {
	if ruleID <= 0 {
		return errors.New("rule id is required")
	}
	_, err := db.ExecContext(ctx, `
INSERT INTO rule_performance(rule_id, match_count, last_matched_at, updated_at)
VALUES(?, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT(rule_id) DO UPDATE SET
    match_count = match_count + 1,
    last_matched_at = CURRENT_TIMESTAMP,
    updated_at = CURRENT_TIMESTAMP;
`, ruleID)
	if err != nil {
		return fmt.Errorf("record rule match: %w", err)
	}
	return nil
}
