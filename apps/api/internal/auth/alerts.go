package auth

import (
	"context"
	"regexp"
)

var alertIDPattern = regexp.MustCompile(`^[A-Za-z0-9:._/-]{1,120}$`)

// AlertFlag is the per-user read state for a computed alert.
type AlertFlag struct {
	Read      bool
	Dismissed bool
}

// AlertStates returns read and dismissed flags for one user.
func (s *Service) AlertStates(ctx context.Context, userID string) (map[string]AlertFlag, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT alert_id, read_at, dismissed_at FROM alert_state WHERE user_id = ?`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]AlertFlag{}
	for rows.Next() {
		var id string
		var readAt, dismissed sqlNull
		if err := rows.Scan(&id, &readAt, &dismissed); err != nil {
			return nil, err
		}
		out[id] = AlertFlag{Read: readAt.valid && readAt.text != "", Dismissed: dismissed.valid && dismissed.text != ""}
	}
	return out, rows.Err()
}

type sqlNull struct {
	text  string
	valid bool
}

func (n *sqlNull) Scan(value any) error {
	if value == nil {
		n.text, n.valid = "", false
		return nil
	}
	switch v := value.(type) {
	case string:
		n.text, n.valid = v, true
	case []byte:
		n.text, n.valid = string(v), true
	default:
		n.text, n.valid = "", false
	}
	return nil
}

// MarkAlertsRead marks ids read, or every currently known id when all is set.
func (s *Service) MarkAlertsRead(ctx context.Context, userID string, ids []string) error {
	now := formatTime(s.now())
	for _, id := range ids {
		if !alertIDPattern.MatchString(id) {
			continue
		}
		if _, err := s.db.ExecContext(ctx, `
			INSERT INTO alert_state (user_id, alert_id, read_at) VALUES (?, ?, ?)
			ON CONFLICT(user_id, alert_id) DO UPDATE SET read_at = excluded.read_at
		`, userID, id, now); err != nil {
			return err
		}
	}
	return nil
}

// DismissAlert hides one alert for this user.
func (s *Service) DismissAlert(ctx context.Context, userID, id string) error {
	if !alertIDPattern.MatchString(id) {
		return ValidationError{Message: "Alert id is invalid"}
	}
	now := formatTime(s.now())
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO alert_state (user_id, alert_id, read_at, dismissed_at) VALUES (?, ?, ?, ?)
		ON CONFLICT(user_id, alert_id) DO UPDATE SET read_at = excluded.read_at, dismissed_at = excluded.dismissed_at
	`, userID, id, now, now)
	return err
}
