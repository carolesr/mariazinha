package db

import (
	"database/sql"
	"log"
	"time"

	_ "modernc.org/sqlite"
)

type DB struct {
	conn *sql.DB
}

func Init(path string) *DB {
	conn, err := sql.Open("sqlite", path+"?_foreign_keys=on&_journal_mode=WAL")
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	conn.SetMaxOpenConns(1)

	if err := migrate(conn); err != nil {
		log.Fatalf("migration failed: %v", err)
	}

	log.Println("✅ database initialized")
	return &DB{conn: conn}
}

func (d *DB) Close() { d.conn.Close() }

func migrate(conn *sql.DB) error {
	_, err := conn.Exec(`
	CREATE TABLE IF NOT EXISTS events (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		group_id    TEXT    NOT NULL,
		name        TEXT    NOT NULL,
		date        TEXT    NOT NULL,
		time        TEXT    NOT NULL,
		location    TEXT    NOT NULL,
		description TEXT    NOT NULL,
		max_spots   INTEGER,
		cost        TEXT,
		status      TEXT    NOT NULL DEFAULT 'active',
		created_by  TEXT    NOT NULL,
		created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS participants (
		id           INTEGER PRIMARY KEY AUTOINCREMENT,
		event_id     INTEGER NOT NULL REFERENCES events(id),
		phone        TEXT    NOT NULL,
		name         TEXT    NOT NULL,
		is_confirmed  BOOLEAN NOT NULL DEFAULT false,
		joined_at    DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(event_id, phone)
	);

	CREATE TABLE IF NOT EXISTS waitlist (
		id        INTEGER PRIMARY KEY AUTOINCREMENT,
		event_id  INTEGER NOT NULL REFERENCES events(id),
		phone     TEXT    NOT NULL,
		name      TEXT    NOT NULL,
		joined_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(event_id, phone)
	);
	`)
	return err
}

// ── Structs ───────────────────────────────────────────────

type Event struct {
	ID          int64
	GroupID     string
	Name        string
	Date        string
	Time        string
	Location    string
	Description string
	MaxSpots    sql.NullInt64
	Cost        sql.NullString
	Status      string
	CreatedBy   string
	CreatedAt   time.Time
	Confirmed   int
	Waiting     int
}

type Participant struct {
	ID          int64
	EventID     int64
	Phone       string
	Name        string
	IsConfirmed bool
	JoinedAt    time.Time
}

// ── Events ────────────────────────────────────────────────

func (d *DB) CreateEvent(e *Event) (int64, error) {
	res, err := d.conn.Exec(`
		INSERT INTO events (group_id, name, date, time, location, description, max_spots, cost, created_by)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.GroupID, e.Name, e.Date, e.Time, e.Location, e.Description,
		nullInt(e.MaxSpots), nullStr(e.Cost), e.CreatedBy,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (d *DB) GetEventByID(id int64) (*Event, error) {
	return scanEvent(d.conn.QueryRow(
		`SELECT `+eventCols+` FROM events e WHERE e.id = ? AND e.status = 'active'`, id,
	))
}

func (d *DB) FindEvent(groupID, query string) (*Event, error) {
	return scanEvent(d.conn.QueryRow(`
		SELECT `+eventCols+` FROM events e
		WHERE e.group_id = ? AND e.status = 'active'
		AND LOWER(e.name) LIKE '%' || LOWER(?) || '%'
		ORDER BY e.date ASC LIMIT 1`, groupID, query,
	))
}

func (d *DB) ListUpcomingEvents(groupID string) ([]*Event, error) {
	rows, err := d.conn.Query(`
		SELECT `+eventCols+` FROM events e
		WHERE e.group_id = ? AND e.status = 'active'
		ORDER BY e.date ASC, e.time ASC`, groupID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanEvents(rows)
}

func (d *DB) CancelEvent(id int64) error {
	_, err := d.conn.Exec(`UPDATE events SET status = 'cancelled' WHERE id = ?`, id)
	return err
}

func (d *DB) UpdateEvent(id int64, field, value string) error {
	_, err := d.conn.Exec(`UPDATE events SET `+field+` = ? WHERE id = ?`, value, id)
	return err
}

// ── Participants ──────────────────────────────────────────

func (d *DB) CountConfirmed(eventID int64) int {
	var n int
	d.conn.QueryRow(`SELECT COUNT(*) FROM participants WHERE event_id = ?`, eventID).Scan(&n)
	return n
}

func (d *DB) IsParticipant(eventID int64, phone string) bool {
	var n int
	d.conn.QueryRow(`SELECT COUNT(*) FROM participants WHERE event_id = ? AND phone = ?`, eventID, phone).Scan(&n)
	return n > 0
}

func (d *DB) IsConfirmed(eventID int64, phone string) bool {
	var n int
	d.conn.QueryRow(`SELECT COUNT(*) FROM participants WHERE event_id = ? AND phone = ? AND is_confirmed = true`, eventID, phone).Scan(&n)
	return n > 0
}

func (d *DB) ConfirmParticipant(eventID int64, phone string) (bool, error) {
	res, err := d.conn.Exec(`UPDATE participants SET is_confirmed = true WHERE event_id = ? AND phone = ?`, eventID, phone)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

func (d *DB) AddParticipant(eventID int64, phone, name string) error {
	_, err := d.conn.Exec(
		`INSERT OR IGNORE INTO participants (event_id, phone, name) VALUES (?, ?, ?)`,
		eventID, phone, name,
	)
	return err
}

func (d *DB) RemoveParticipant(eventID int64, phone string) (bool, error) {
	res, err := d.conn.Exec(`DELETE FROM participants WHERE event_id = ? AND phone = ?`, eventID, phone)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

func (d *DB) ListParticipants(eventID int64) ([]*Participant, error) {
	rows, err := d.conn.Query(
		`SELECT id, event_id, phone, name, is_confirmed, joined_at FROM participants WHERE event_id = ? ORDER BY joined_at ASC`,
		eventID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanParticipants(rows)
}

// ── Waitlist ──────────────────────────────────────────────

func (d *DB) IsOnWaitlist(eventID int64, phone string) bool {
	var n int
	d.conn.QueryRow(`SELECT COUNT(*) FROM waitlist WHERE event_id = ? AND phone = ?`, eventID, phone).Scan(&n)
	return n > 0
}

func (d *DB) WaitlistPosition(eventID int64, phone string) int {
	var pos int
	d.conn.QueryRow(`
		SELECT COUNT(*) FROM waitlist
		WHERE event_id = ? AND joined_at <= (SELECT joined_at FROM waitlist WHERE event_id = ? AND phone = ?)`,
		eventID, eventID, phone,
	).Scan(&pos)
	return pos
}

func (d *DB) AddToWaitlist(eventID int64, phone, name string) error {
	_, err := d.conn.Exec(
		`INSERT OR IGNORE INTO waitlist (event_id, phone, name) VALUES (?, ?, ?)`,
		eventID, phone, name,
	)
	return err
}

func (d *DB) RemoveFromWaitlist(eventID int64, phone string) (bool, error) {
	res, err := d.conn.Exec(`DELETE FROM waitlist WHERE event_id = ? AND phone = ?`, eventID, phone)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

func (d *DB) NextOnWaitlist(eventID int64) (*Participant, error) {
	row := d.conn.QueryRow(
		`SELECT id, event_id, phone, name, joined_at FROM waitlist WHERE event_id = ? ORDER BY joined_at ASC LIMIT 1`,
		eventID,
	)
	var p Participant
	err := row.Scan(&p.ID, &p.EventID, &p.Phone, &p.Name, &p.JoinedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &p, err
}

func (d *DB) ListWaitlist(eventID int64) ([]*Participant, error) {
	rows, err := d.conn.Query(
		`SELECT id, event_id, phone, name, joined_at FROM waitlist WHERE event_id = ? ORDER BY joined_at ASC`,
		eventID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanParticipants(rows)
}

// ── Helpers ───────────────────────────────────────────────

const eventCols = `
	e.id, e.group_id, e.name, e.date, e.time, e.location, e.description,
	e.max_spots, e.cost, e.status, e.created_by, e.created_at,
	(SELECT COUNT(*) FROM participants p WHERE p.event_id = e.id) as confirmed,
	(SELECT COUNT(*) FROM waitlist w WHERE w.event_id = e.id) as waiting`

func scanEvent(row *sql.Row) (*Event, error) {
	var e Event
	err := row.Scan(
		&e.ID, &e.GroupID, &e.Name, &e.Date, &e.Time, &e.Location, &e.Description,
		&e.MaxSpots, &e.Cost, &e.Status, &e.CreatedBy, &e.CreatedAt,
		&e.Confirmed, &e.Waiting,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &e, err
}

func scanEvents(rows *sql.Rows) ([]*Event, error) {
	var list []*Event
	for rows.Next() {
		var e Event
		if err := rows.Scan(
			&e.ID, &e.GroupID, &e.Name, &e.Date, &e.Time, &e.Location, &e.Description,
			&e.MaxSpots, &e.Cost, &e.Status, &e.CreatedBy, &e.CreatedAt,
			&e.Confirmed, &e.Waiting,
		); err != nil {
			return nil, err
		}
		list = append(list, &e)
	}
	return list, nil
}

func scanParticipants(rows *sql.Rows) ([]*Participant, error) {
	var list []*Participant
	for rows.Next() {
		var p Participant
		if err := rows.Scan(&p.ID, &p.EventID, &p.Phone, &p.Name, &p.IsConfirmed, &p.JoinedAt); err != nil {
			return nil, err
		}
		list = append(list, &p)
	}
	return list, nil
}

func nullInt(v sql.NullInt64) interface{} {
	if v.Valid {
		return v.Int64
	}
	return nil
}

func nullStr(v sql.NullString) interface{} {
	if v.Valid {
		return v.String
	}
	return nil
}
