package rules

import (
	"context"
	"database/sql"
	"fmt"
)

// LoadEnabled reads enabled rule definitions in deterministic order.
func LoadEnabled(ctx context.Context, db *sql.DB) ([]Rule, error) {
	rows, err := db.QueryContext(ctx, `
SELECT id, name, source, matcher, pattern, COALESCE(field, ''), action, priority, enabled
FROM rules
WHERE enabled = 1
ORDER BY priority, id;
`)
	if err != nil {
		return nil, fmt.Errorf("load enabled rules: %w", err)
	}
	defer rows.Close()

	var loaded []Rule
	for rows.Next() {
		var rule Rule
		var enabled int
		if err := rows.Scan(&rule.ID, &rule.Name, &rule.Source, &rule.Matcher, &rule.Pattern, &rule.Field, &rule.Action, &rule.Priority, &enabled); err != nil {
			return nil, fmt.Errorf("scan rule: %w", err)
		}
		rule.Enabled = enabled != 0
		loaded = append(loaded, rule)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("load enabled rules: %w", err)
	}
	return loaded, nil
}
