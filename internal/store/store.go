package store

import (
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// Check represents a monitored cron job.
type Check struct {
	ID        int64
	Name      string
	Slug      string
	Schedule  string
	Grace     string
	LastPing  *time.Time
	LastOutput string
	LastFail  *time.Time
	Alerted   bool
	CreatedAt time.Time
}

// AlertDest is an alert destination (webhook URL or email address).
type AlertDest struct {
	ID      int64  `json:"id"`
	CheckID int64  `json:"check_id"`
	Type    string `json:"type"`
	Target  string `json:"target"`
}

// Settings holds SMTP configuration.
type Settings struct {
	SMTPHost     string `json:"smtp_host"`
	SMTPPort     int    `json:"smtp_port"`
	SMTPUser     string `json:"smtp_user"`
	SMTPPassword string `json:"smtp_password"`
	SMTPFrom     string `json:"smtp_from"`
}

// DB wraps the SQLite database.
type DB struct {
	db *sql.DB
}

const schema = `
CREATE TABLE IF NOT EXISTS checks (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT NOT NULL,
	slug TEXT NOT NULL,
	schedule TEXT NOT NULL,
	grace TEXT NOT NULL DEFAULT '5m',
	last_ping TIMESTAMP,
	last_output TEXT NOT NULL DEFAULT '',
	last_fail TIMESTAMP,
	alerted INTEGER NOT NULL DEFAULT 0,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_checks_slug ON checks(slug);

CREATE TABLE IF NOT EXISTS alert_destinations (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	check_id INTEGER NOT NULL REFERENCES checks(id) ON DELETE CASCADE,
	type TEXT NOT NULL,
	target TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_alert_destinations_check_id ON alert_destinations(check_id);

CREATE TABLE IF NOT EXISTS settings (
	id INTEGER PRIMARY KEY CHECK (id = 1),
	smtp_host TEXT NOT NULL DEFAULT '',
	smtp_port INTEGER NOT NULL DEFAULT 587,
	smtp_user TEXT NOT NULL DEFAULT '',
	smtp_password TEXT NOT NULL DEFAULT '',
	smtp_from TEXT NOT NULL DEFAULT ''
);

INSERT OR IGNORE INTO settings (id) VALUES (1);
`

// Open opens or creates a SQLite database at the given path.
// Use ":memory:" for an in-memory database (tests).
func Open(path string) (*DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Enable WAL mode and foreign keys.
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set WAL mode: %w", err)
	}
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}

	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("create schema: %w", err)
	}

	return &DB{db: db}, nil
}

// Close closes the database.
func (d *DB) Close() error {
	return d.db.Close()
}

var nonAlphanumeric = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(name string) string {
	s := strings.ToLower(name)
	s = nonAlphanumeric.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	return s
}

// CreateCheck creates a new check with the given name, schedule, and grace period.
func (d *DB) CreateCheck(name, schedule, grace string) (Check, error) {
	slug := slugify(name)
	now := time.Now().UTC()

	res, err := d.db.Exec(
		`INSERT INTO checks (name, slug, schedule, grace, created_at) VALUES (?, ?, ?, ?, ?)`,
		name, slug, schedule, grace, now,
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return Check{}, fmt.Errorf("a check with the name %q already exists", name)
		}
		return Check{}, fmt.Errorf("insert check: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return Check{}, fmt.Errorf("last insert id: %w", err)
	}

	return Check{
		ID:        id,
		Name:      name,
		Slug:      slug,
		Schedule:  schedule,
		Grace:     grace,
		CreatedAt: now,
	}, nil
}

// GetCheck retrieves a check by ID.
func (d *DB) GetCheck(id int64) (Check, error) {
	return d.scanCheck(d.db.QueryRow(
		`SELECT id, name, slug, schedule, grace, last_ping, last_output, last_fail, alerted, created_at
		 FROM checks WHERE id = ?`, id,
	))
}

// GetCheckBySlug retrieves a check by slug.
func (d *DB) GetCheckBySlug(slug string) (Check, error) {
	return d.scanCheck(d.db.QueryRow(
		`SELECT id, name, slug, schedule, grace, last_ping, last_output, last_fail, alerted, created_at
		 FROM checks WHERE slug = ?`, slug,
	))
}

// ListChecks returns all checks ordered by name.
func (d *DB) ListChecks() ([]Check, error) {
	rows, err := d.db.Query(
		`SELECT id, name, slug, schedule, grace, last_ping, last_output, last_fail, alerted, created_at
		 FROM checks ORDER BY name`,
	)
	if err != nil {
		return nil, fmt.Errorf("list checks: %w", err)
	}
	defer rows.Close()

	var checks []Check
	for rows.Next() {
		c, err := d.scanCheckRow(rows)
		if err != nil {
			return nil, err
		}
		checks = append(checks, c)
	}
	return checks, rows.Err()
}

// UpdateCheck updates the name, schedule, and grace period of a check.
func (d *DB) UpdateCheck(id int64, name, schedule, grace string) error {
	slug := slugify(name)
	_, err := d.db.Exec(
		`UPDATE checks SET name = ?, slug = ?, schedule = ?, grace = ? WHERE id = ?`,
		name, slug, schedule, grace, id,
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return fmt.Errorf("a check with the name %q already exists", name)
		}
		return fmt.Errorf("update check: %w", err)
	}
	return nil
}

// DeleteCheck deletes a check and its alert destinations.
func (d *DB) DeleteCheck(id int64) error {
	_, err := d.db.Exec(`DELETE FROM checks WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete check: %w", err)
	}
	return nil
}

// RecordPing updates last_ping and last_output for a check.
// Returns true if the check was in alerted state (caller should send recovery).
func (d *DB) RecordPing(slug, output string) (bool, error) {
	now := time.Now().UTC()
	if len(output) > 10240 {
		output = output[:10240]
	}

	var alerted bool
	err := d.db.QueryRow(`SELECT alerted FROM checks WHERE slug = ?`, slug).Scan(&alerted)
	if err != nil {
		return false, fmt.Errorf("get check: %w", err)
	}

	_, err = d.db.Exec(
		`UPDATE checks SET last_ping = ?, last_output = ?, alerted = 0 WHERE slug = ?`,
		now, output, slug,
	)
	if err != nil {
		return false, fmt.Errorf("record ping: %w", err)
	}

	return alerted, nil
}

// RecordPingAt records a ping at a specific time (for testing).
func (d *DB) RecordPingAt(slug, output string, at time.Time) error {
	if len(output) > 10240 {
		output = output[:10240]
	}
	_, err := d.db.Exec(
		`UPDATE checks SET last_ping = ?, last_output = ? WHERE slug = ?`,
		at.UTC(), output, slug,
	)
	if err != nil {
		return fmt.Errorf("record ping at: %w", err)
	}
	return nil
}

// RecordFail updates last_fail and last_output for a check.
func (d *DB) RecordFail(slug, output string) error {
	now := time.Now().UTC()
	if len(output) > 10240 {
		output = output[:10240]
	}

	_, err := d.db.Exec(
		`UPDATE checks SET last_fail = ?, last_output = ? WHERE slug = ?`,
		now, output, slug,
	)
	if err != nil {
		return fmt.Errorf("record fail: %w", err)
	}
	return nil
}

// SetAlerted sets the alerted flag for a check.
func (d *DB) SetAlerted(checkID int64, alerted bool) error {
	val := 0
	if alerted {
		val = 1
	}
	_, err := d.db.Exec(`UPDATE checks SET alerted = ? WHERE id = ?`, val, checkID)
	if err != nil {
		return fmt.Errorf("set alerted: %w", err)
	}
	return nil
}

// CreateAlertDest adds an alert destination for a check.
func (d *DB) CreateAlertDest(checkID int64, destType, target string) (AlertDest, error) {
	res, err := d.db.Exec(
		`INSERT INTO alert_destinations (check_id, type, target) VALUES (?, ?, ?)`,
		checkID, destType, target,
	)
	if err != nil {
		return AlertDest{}, fmt.Errorf("insert alert dest: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return AlertDest{}, fmt.Errorf("last insert id: %w", err)
	}
	return AlertDest{
		ID:      id,
		CheckID: checkID,
		Type:    destType,
		Target:  target,
	}, nil
}

// ListAlertDests returns all alert destinations for a check.
func (d *DB) ListAlertDests(checkID int64) ([]AlertDest, error) {
	rows, err := d.db.Query(
		`SELECT id, check_id, type, target FROM alert_destinations WHERE check_id = ?`,
		checkID,
	)
	if err != nil {
		return nil, fmt.Errorf("list alert dests: %w", err)
	}
	defer rows.Close()

	var dests []AlertDest
	for rows.Next() {
		var d AlertDest
		if err := rows.Scan(&d.ID, &d.CheckID, &d.Type, &d.Target); err != nil {
			return nil, fmt.Errorf("scan alert dest: %w", err)
		}
		dests = append(dests, d)
	}
	return dests, rows.Err()
}

// DeleteAlertDest removes an alert destination by ID.
func (d *DB) DeleteAlertDest(id int64) error {
	_, err := d.db.Exec(`DELETE FROM alert_destinations WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete alert dest: %w", err)
	}
	return nil
}

// GetSettings retrieves the SMTP settings.
func (d *DB) GetSettings() (Settings, error) {
	var s Settings
	err := d.db.QueryRow(
		`SELECT smtp_host, smtp_port, smtp_user, smtp_password, smtp_from FROM settings WHERE id = 1`,
	).Scan(&s.SMTPHost, &s.SMTPPort, &s.SMTPUser, &s.SMTPPassword, &s.SMTPFrom)
	if err != nil {
		return Settings{}, fmt.Errorf("get settings: %w", err)
	}
	return s, nil
}

// SMTPConfig returns the SMTP settings as individual values, implementing alert.SettingsProvider.
func (d *DB) SMTPConfig() (host string, port int, user, password, from string, err error) {
	s, err := d.GetSettings()
	if err != nil {
		return "", 0, "", "", "", err
	}
	return s.SMTPHost, s.SMTPPort, s.SMTPUser, s.SMTPPassword, s.SMTPFrom, nil
}

// UpdateSettings updates the SMTP settings.
func (d *DB) UpdateSettings(s Settings) error {
	_, err := d.db.Exec(
		`UPDATE settings SET smtp_host = ?, smtp_port = ?, smtp_user = ?, smtp_password = ?, smtp_from = ? WHERE id = 1`,
		s.SMTPHost, s.SMTPPort, s.SMTPUser, s.SMTPPassword, s.SMTPFrom,
	)
	if err != nil {
		return fmt.Errorf("update settings: %w", err)
	}
	return nil
}

type scanner interface {
	Scan(dest ...any) error
}

func (d *DB) scanCheck(row scanner) (Check, error) {
	var c Check
	var lastPing, lastFail sql.NullTime
	err := row.Scan(
		&c.ID, &c.Name, &c.Slug, &c.Schedule, &c.Grace,
		&lastPing, &c.LastOutput, &lastFail, &c.Alerted, &c.CreatedAt,
	)
	if err != nil {
		return Check{}, fmt.Errorf("scan check: %w", err)
	}
	if lastPing.Valid {
		c.LastPing = &lastPing.Time
	}
	if lastFail.Valid {
		c.LastFail = &lastFail.Time
	}
	return c, nil
}

func (d *DB) scanCheckRow(rows *sql.Rows) (Check, error) {
	var c Check
	var lastPing, lastFail sql.NullTime
	err := rows.Scan(
		&c.ID, &c.Name, &c.Slug, &c.Schedule, &c.Grace,
		&lastPing, &c.LastOutput, &lastFail, &c.Alerted, &c.CreatedAt,
	)
	if err != nil {
		return Check{}, fmt.Errorf("scan check: %w", err)
	}
	if lastPing.Valid {
		c.LastPing = &lastPing.Time
	}
	if lastFail.Valid {
		c.LastFail = &lastFail.Time
	}
	return c, nil
}
