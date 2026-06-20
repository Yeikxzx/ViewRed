package storage

import (
	"context"
)

// GetCustomRules retrieves all custom rules from the database.
func (s *Storage) GetCustomRules(ctx context.Context) ([]CustomRule, error) {
	query := `
		SELECT id, rule_text, enabled, created_at
		FROM custom_rules
		ORDER BY id ASC
	`
	rows, err := s.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []CustomRule
	for rows.Next() {
		var r CustomRule
		if err := rows.Scan(&r.ID, &r.RuleText, &r.Enabled, &r.CreatedAt); err != nil {
			return nil, err
		}
		rules = append(rules, r)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return rules, nil
}

// SaveCustomRule inserts a new custom rule.
func (s *Storage) SaveCustomRule(ctx context.Context, ruleText string, enabled bool) (int64, error) {
	query := `
		INSERT INTO custom_rules (rule_text, enabled)
		VALUES ($1, $2)
		RETURNING id
	`
	var id int64
	err := s.pool.QueryRow(ctx, query, ruleText, enabled).Scan(&id)
	if err != nil {
		return 0, err
	}
	return id, nil
}

// DeleteCustomRule deletes a custom rule by ID.
func (s *Storage) DeleteCustomRule(ctx context.Context, id int64) error {
	query := `DELETE FROM custom_rules WHERE id = $1`
	_, err := s.pool.Exec(ctx, query, id)
	return err
}
