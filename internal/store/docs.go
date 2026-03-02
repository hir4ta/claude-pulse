package store

import (
	"crypto/sha256"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// DocRow represents a row in the docs table.
type DocRow struct {
	ID          int64
	URL         string
	SectionPath string
	Content     string
	ContentHash string
	SourceType  string
	Version     string
	CrawledAt   string
	TTLDays     int
}

// ContentHashOf returns the SHA-256 hex hash of content for change detection.
func ContentHashOf(content string) string {
	h := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", h)
}

// UpsertDoc inserts or updates a doc section. Returns the row ID and whether
// the content was actually changed (false if hash matched existing row).
func (s *Store) UpsertDoc(doc *DocRow) (id int64, changed bool, err error) {
	doc.ContentHash = ContentHashOf(doc.Content)
	if doc.CrawledAt == "" {
		doc.CrawledAt = time.Now().UTC().Format(time.RFC3339)
	}
	if doc.TTLDays == 0 {
		doc.TTLDays = 7
	}

	var existingID int64
	var existingHash string
	err = s.db.QueryRow(
		`SELECT id, content_hash FROM docs WHERE url = ? AND section_path = ?`,
		doc.URL, doc.SectionPath,
	).Scan(&existingID, &existingHash)
	if err == nil && existingHash == doc.ContentHash {
		return existingID, false, nil
	}

	res, err := s.db.Exec(`
		INSERT INTO docs (url, section_path, content, content_hash, source_type, version, crawled_at, ttl_days)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(url, section_path) DO UPDATE SET
			content = excluded.content,
			content_hash = excluded.content_hash,
			source_type = excluded.source_type,
			version = excluded.version,
			crawled_at = excluded.crawled_at,
			ttl_days = excluded.ttl_days`,
		doc.URL, doc.SectionPath, doc.Content, doc.ContentHash,
		doc.SourceType, doc.Version, doc.CrawledAt, doc.TTLDays,
	)
	if err != nil {
		return 0, false, fmt.Errorf("store: upsert doc: %w", err)
	}

	id, _ = res.LastInsertId()
	if id == 0 {
		_ = s.db.QueryRow(
			`SELECT id FROM docs WHERE url = ? AND section_path = ?`,
			doc.URL, doc.SectionPath,
		).Scan(&id)
	}
	return id, true, nil
}

// GetDoc retrieves a single doc by ID.
func (s *Store) GetDoc(id int64) (*DocRow, error) {
	var d DocRow
	var version sql.NullString
	err := s.db.QueryRow(`
		SELECT id, url, section_path, content, content_hash, source_type, version, crawled_at, ttl_days
		FROM docs WHERE id = ?`, id,
	).Scan(&d.ID, &d.URL, &d.SectionPath, &d.Content, &d.ContentHash,
		&d.SourceType, &version, &d.CrawledAt, &d.TTLDays)
	if err != nil {
		return nil, err
	}
	d.Version = version.String
	return &d, nil
}

// GetDocsByIDs retrieves multiple docs by their IDs.
func (s *Store) GetDocsByIDs(ids []int64) ([]DocRow, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	query := "SELECT id, url, section_path, content, content_hash, source_type, version, crawled_at, ttl_days FROM docs WHERE id IN ("
	args := make([]any, len(ids))
	for i, id := range ids {
		if i > 0 {
			query += ","
		}
		query += "?"
		args[i] = id
	}
	query += ")"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("store: get docs by ids: %w", err)
	}
	defer rows.Close()

	var docs []DocRow
	for rows.Next() {
		var d DocRow
		var version sql.NullString
		if err := rows.Scan(&d.ID, &d.URL, &d.SectionPath, &d.Content, &d.ContentHash,
			&d.SourceType, &version, &d.CrawledAt, &d.TTLDays); err != nil {
			return nil, fmt.Errorf("store: scan docs by ids: %w", err)
		}
		d.Version = version.String
		docs = append(docs, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate docs by ids: %w", err)
	}
	return docs, nil
}

var fts5Replacer = strings.NewReplacer(
	`"`, " ", `(`, " ", `)`, " ",
	`*`, " ", `+`, " ", `^`, " ",
	`:`, " ", `{`, " ", `}`, " ",
)

var fts5Reserved = map[string]bool{
	"AND": true, "OR": true, "NOT": true, "NEAR": true,
}

// SanitizeFTS5Query strips FTS5 special characters and reserved words from a
// user query. Short single-word queries (3-6 chars) get prefix expansion.
func SanitizeFTS5Query(query string) string {
	q := fts5Replacer.Replace(query)
	words := strings.Fields(q)
	filtered := words[:0]
	for _, w := range words {
		w = strings.TrimLeft(w, "-")
		if w == "" || fts5Reserved[strings.ToUpper(w)] {
			continue
		}
		filtered = append(filtered, w)
	}
	if len(filtered) == 0 {
		return ""
	}
	if len(filtered) == 1 && len(filtered[0]) >= 3 && len(filtered[0]) <= 6 {
		return filtered[0] + "*"
	}
	return strings.Join(filtered, " ")
}

// SearchDocsFTS searches the docs table using FTS5 full-text search.
func (s *Store) SearchDocsFTS(rawQuery string, sourceType string, limit int) ([]DocRow, error) {
	if limit <= 0 {
		limit = 10
	}
	query := SanitizeFTS5Query(rawQuery)
	if query == "" {
		return nil, nil
	}

	words := strings.Fields(query)
	if len(words) > 1 {
		phraseQuery := `"` + strings.Join(words, " ") + `"`
		results, err := s.matchDocsFTS(phraseQuery, sourceType, limit)
		if err == nil && len(results) > 0 {
			return results, nil
		}
		query = strings.Join(words, " OR ")
	}
	return s.matchDocsFTS(query, sourceType, limit)
}

func (s *Store) matchDocsFTS(query string, sourceType string, limit int) ([]DocRow, error) {
	var sqlQuery string
	var args []any

	if sourceType != "" {
		sqlQuery = `
			SELECT d.id, d.url, d.section_path, d.content, d.content_hash,
			       d.source_type, d.version, d.crawled_at, d.ttl_days
			FROM docs_fts f
			JOIN docs d ON d.id = f.rowid
			WHERE docs_fts MATCH ? AND d.source_type = ?
			ORDER BY rank
			LIMIT ?`
		args = []any{query, sourceType, limit}
	} else {
		sqlQuery = `
			SELECT d.id, d.url, d.section_path, d.content, d.content_hash,
			       d.source_type, d.version, d.crawled_at, d.ttl_days
			FROM docs_fts f
			JOIN docs d ON d.id = f.rowid
			WHERE docs_fts MATCH ?
			ORDER BY rank
			LIMIT ?`
		args = []any{query, limit}
	}

	rows, err := s.db.Query(sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("store: search docs fts: %w", err)
	}
	defer rows.Close()

	var docs []DocRow
	for rows.Next() {
		var d DocRow
		var version sql.NullString
		if err := rows.Scan(&d.ID, &d.URL, &d.SectionPath, &d.Content, &d.ContentHash,
			&d.SourceType, &version, &d.CrawledAt, &d.TTLDays); err != nil {
			return nil, fmt.Errorf("store: scan fts result: %w", err)
		}
		d.Version = version.String
		docs = append(docs, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate fts results: %w", err)
	}
	return docs, nil
}

// DeleteExpiredDocs removes docs whose TTL has expired.
func (s *Store) DeleteExpiredDocs() (int64, error) {
	res, err := s.db.Exec(`
		DELETE FROM docs
		WHERE julianday('now') - julianday(crawled_at) > ttl_days`)
	if err != nil {
		return 0, fmt.Errorf("store: delete expired docs: %w", err)
	}
	s.db.Exec(`DELETE FROM embeddings WHERE source = 'docs' AND source_id NOT IN (SELECT id FROM docs)`)
	return res.RowsAffected()
}
