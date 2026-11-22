package main

import (
	"context"
	"database/sql"
	"math/rand/v2"

	_ "modernc.org/sqlite"
)

type link struct {
	Code string `json:"code"`
	URL  string `json:"url"`
	Hops int    `json:"hops"`
}

type store struct {
	db *sql.DB
}

// openStore abre o SQLite em modo WAL. As três réplicas usam o mesmo arquivo,
// e o busy_timeout faz uma escrita esperar a outra em vez de falhar na hora.
// Isso só funciona porque todas rodam no mesmo kernel e sistema de arquivos.
func openStore(path string) (*store, error) {
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS links (
		code       TEXT PRIMARY KEY,
		url        TEXT NOT NULL,
		hops       INTEGER NOT NULL,
		created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		return nil, err
	}
	return &store{db: db}, nil
}

func (s *store) create(ctx context.Context, url string, hops int) (link, error) {
	l := link{Code: newCode(), URL: url, Hops: hops}
	_, err := s.db.ExecContext(ctx, "INSERT INTO links (code, url, hops) VALUES (?, ?, ?)", l.Code, l.URL, l.Hops)
	return l, err
}

func (s *store) get(ctx context.Context, code string) (link, error) {
	l := link{Code: code}
	err := s.db.QueryRowContext(ctx, "SELECT url, hops FROM links WHERE code = ?", code).Scan(&l.URL, &l.Hops)
	return l, err
}

func (s *store) list(ctx context.Context, limit int) ([]link, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT code, url, hops FROM links ORDER BY created_at DESC LIMIT ?", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	links := []link{}
	for rows.Next() {
		var l link
		if err := rows.Scan(&l.Code, &l.URL, &l.Hops); err != nil {
			return nil, err
		}
		links = append(links, l)
	}
	return links, rows.Err()
}

// shortcut: código aleatório sem checar colisão (uma colisão vira erro 500),
// trocar por nova tentativa no INSERT se o volume de links crescer.
func newCode() string {
	const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 7)
	for i := range b {
		b[i] = alphabet[rand.IntN(len(alphabet))]
	}
	return string(b)
}
