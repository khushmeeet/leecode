package main

import (
	"database/sql"
	"net/url"
	"strings"

	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS applications (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	company    TEXT NOT NULL,
	role       TEXT NOT NULL DEFAULT '',
	url        TEXT NOT NULL DEFAULT '',
	notes      TEXT NOT NULL DEFAULT '',
	status     TEXT NOT NULL DEFAULT 'Applied' CHECK (status IN ('Applied','Rejected','Accepted')),
	created_at TEXT NOT NULL DEFAULT (date('now'))
);
CREATE TABLE IF NOT EXISTS links (
	id          INTEGER PRIMARY KEY AUTOINCREMENT,
	title       TEXT NOT NULL,
	url         TEXT NOT NULL,
	description TEXT NOT NULL DEFAULT '',
	tag         TEXT NOT NULL DEFAULT '',
	read        INTEGER NOT NULL DEFAULT 0,
	created_at  TEXT NOT NULL DEFAULT (date('now'))
);
CREATE TABLE IF NOT EXISTS problems (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	title      TEXT NOT NULL,
	url        TEXT NOT NULL DEFAULT '',
	difficulty TEXT NOT NULL DEFAULT 'Easy' CHECK (difficulty IN ('Easy','Medium','Hard')),
	pattern    TEXT NOT NULL DEFAULT '',
	notes      TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL DEFAULT (date('now'))
);
CREATE TABLE IF NOT EXISTS behavioral (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	question   TEXT NOT NULL,
	answer     TEXT NOT NULL DEFAULT '',
	category   TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL DEFAULT (date('now'))
);
`

func openDB(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(schema); err != nil {
		return nil, err
	}
	// links.description arrived after the first release; older db files need the column.
	if _, err := db.Exec(`ALTER TABLE links ADD COLUMN description TEXT NOT NULL DEFAULT ''`); err != nil &&
		!strings.Contains(err.Error(), "duplicate column") {
		return nil, err
	}
	return db, nil
}

type Application struct {
	ID                        int64
	Company, Role, URL, Notes string
	Status, CreatedAt         string
}

type Link struct {
	ID                           int64
	Title, URL, Description, Tag string
	Host                         string // derived from URL, not stored
	Read                         bool
	CreatedAt                    string
}

type Problem struct {
	ID                              int64
	Title, URL, Difficulty, Pattern string
	Notes, CreatedAt                string
}

type Story struct {
	ID                         int64
	Question, Answer, Category string
	CreatedAt                  string
}

func listApplications(db *sql.DB) ([]Application, error) {
	rows, err := db.Query(`SELECT id, company, role, url, notes, status, created_at FROM applications ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Application
	for rows.Next() {
		var a Application
		if err := rows.Scan(&a.ID, &a.Company, &a.Role, &a.URL, &a.Notes, &a.Status, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func listLinks(db *sql.DB, tag string) ([]Link, error) {
	q := `SELECT id, title, url, description, tag, read, created_at FROM links`
	var args []any
	if tag != "" {
		q += ` WHERE tag = ?`
		args = append(args, tag)
	}
	q += ` ORDER BY read ASC, id DESC`
	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Link
	for rows.Next() {
		var l Link
		if err := rows.Scan(&l.ID, &l.Title, &l.URL, &l.Description, &l.Tag, &l.Read, &l.CreatedAt); err != nil {
			return nil, err
		}
		if u, err := url.Parse(l.URL); err == nil {
			l.Host = strings.TrimPrefix(u.Host, "www.")
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

func listProblems(db *sql.DB, difficulty, pattern string) ([]Problem, error) {
	q := `SELECT id, title, url, difficulty, pattern, notes, created_at FROM problems WHERE 1=1`
	var args []any
	if difficulty != "" {
		q += ` AND difficulty = ?`
		args = append(args, difficulty)
	}
	if pattern != "" {
		q += ` AND pattern = ?`
		args = append(args, pattern)
	}
	q += ` ORDER BY id DESC`
	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Problem
	for rows.Next() {
		var p Problem
		if err := rows.Scan(&p.ID, &p.Title, &p.URL, &p.Difficulty, &p.Pattern, &p.Notes, &p.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func listStories(db *sql.DB, category string) ([]Story, error) {
	q := `SELECT id, question, answer, category, created_at FROM behavioral`
	var args []any
	if category != "" {
		q += ` WHERE category = ?`
		args = append(args, category)
	}
	q += ` ORDER BY id DESC`
	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Story
	for rows.Next() {
		var s Story
		if err := rows.Scan(&s.ID, &s.Question, &s.Answer, &s.Category, &s.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// distinctValues returns the non-empty distinct values of a column, for filter dropdowns.
// table and column are always compile-time constants, never user input.
func distinctValues(db *sql.DB, table, column string) ([]string, error) {
	rows, err := db.Query(`SELECT DISTINCT ` + column + ` FROM ` + table + ` WHERE ` + column + ` != '' ORDER BY ` + column)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

type DashboardCounts struct {
	Applied, Rejected, Accepted int
	UnreadLinks, TotalLinks     int
	Problems, Stories           int
}

func dashboardCounts(db *sql.DB) (DashboardCounts, error) {
	var c DashboardCounts
	err := db.QueryRow(`SELECT
		(SELECT COUNT(*) FROM applications WHERE status = 'Applied'),
		(SELECT COUNT(*) FROM applications WHERE status = 'Rejected'),
		(SELECT COUNT(*) FROM applications WHERE status = 'Accepted'),
		(SELECT COUNT(*) FROM links WHERE read = 0),
		(SELECT COUNT(*) FROM links),
		(SELECT COUNT(*) FROM problems),
		(SELECT COUNT(*) FROM behavioral)`).
		Scan(&c.Applied, &c.Rejected, &c.Accepted, &c.UnreadLinks, &c.TotalLinks, &c.Problems, &c.Stories)
	return c, err
}
