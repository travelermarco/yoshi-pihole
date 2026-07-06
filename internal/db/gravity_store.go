package db

import (
	"database/sql"
	"fmt"
	"time"
)

// Domain-list entry types, matching Pi-hole's domainlist.type convention.
const (
	TypeAllowExact = 0
	TypeDenyExact  = 1
	TypeAllowRegex = 2
	TypeDenyRegex  = 3
)

// Adlist source types.
const (
	AdlistBlock = 0
	AdlistAllow = 1
)

type Domain struct {
	ID           int64
	Domain       string
	Type         int
	Enabled      bool
	Comment      string
	DateAdded    int64
	DateModified int64
}

type Adlist struct {
	ID             int64
	Address        string
	Type           int
	Enabled        bool
	Comment        string
	Number         int
	InvalidDomains int
	Status         string
	ETag           string
	DateAdded      int64
	DateUpdated    sql.NullInt64
}

type Group struct {
	ID          int64
	Enabled     bool
	Name        string
	Description string
}

type Client struct {
	ID      int64
	IP      string
	Comment string
}

// RawRegexRule is an unparsed regex filter as stored in domainlist, ready to
// be compiled by the gravity package.
type RawRegexRule struct {
	ID  int64
	Raw string
}

// Snapshot is everything the in-memory matcher engine needs to evaluate
// queries, pulled fresh from gravity.db respecting enabled adlists/domains
// and enabled groups.
type Snapshot struct {
	ExactAllow map[string]struct{}
	// ExactDeny maps domain -> source label ("gravity" or "manual"), so the
	// query log can distinguish a blocklist hit from a manual blacklist hit.
	ExactDeny   map[string]string
	RegexAllow  []RawRegexRule
	RegexDeny   []RawRegexRule
}

type GravityStore struct{ db *sql.DB }

func NewGravityStore(db *sql.DB) *GravityStore { return &GravityStore{db: db} }

func now() int64 { return time.Now().Unix() }

// --- domainlist (manual allow/deny/regex entries) ---

func (s *GravityStore) AddDomain(domain string, listType int, comment string) (int64, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	res, err := tx.Exec(
		`INSERT INTO domainlist (domain, type, enabled, comment, date_added, date_modified)
		 VALUES (?, ?, 1, ?, ?, ?)`,
		domain, listType, comment, now(), now(),
	)
	if err != nil {
		return 0, fmt.Errorf("inserting domain: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}

	if _, err := tx.Exec(`INSERT OR IGNORE INTO domainlist_by_group (domainlist_id, group_id) VALUES (?, 0)`, id); err != nil {
		return 0, fmt.Errorf("assigning domain to default group: %w", err)
	}

	return id, tx.Commit()
}

func (s *GravityStore) ListDomains() ([]Domain, error) {
	rows, err := s.db.Query(`SELECT id, domain, type, enabled, comment, date_added, date_modified FROM domainlist ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Domain
	for rows.Next() {
		var d Domain
		var enabled int
		var comment sql.NullString
		if err := rows.Scan(&d.ID, &d.Domain, &d.Type, &enabled, &comment, &d.DateAdded, &d.DateModified); err != nil {
			return nil, err
		}
		d.Enabled = enabled != 0
		d.Comment = comment.String
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *GravityStore) RemoveDomain(id int64) error {
	_, err := s.db.Exec(`DELETE FROM domainlist WHERE id = ?`, id)
	return err
}

func (s *GravityStore) SetDomainEnabled(id int64, enabled bool) error {
	_, err := s.db.Exec(`UPDATE domainlist SET enabled = ?, date_modified = ? WHERE id = ?`, boolToInt(enabled), now(), id)
	return err
}

// --- adlist (blocklist/allowlist subscriptions) ---

func (s *GravityStore) AddAdlist(address string, listType int, comment string) (int64, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	res, err := tx.Exec(
		`INSERT INTO adlist (address, type, enabled, comment, status, date_added, date_modified)
		 VALUES (?, ?, 1, ?, 'pending', ?, ?)`,
		address, listType, comment, now(), now(),
	)
	if err != nil {
		return 0, fmt.Errorf("inserting adlist: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}

	if _, err := tx.Exec(`INSERT OR IGNORE INTO adlist_by_group (adlist_id, group_id) VALUES (?, 0)`, id); err != nil {
		return 0, fmt.Errorf("assigning adlist to default group: %w", err)
	}

	return id, tx.Commit()
}

func (s *GravityStore) ListAdlists() ([]Adlist, error) {
	rows, err := s.db.Query(`SELECT id, address, type, enabled, comment, number, invalid_domains, status, etag, date_added, date_updated FROM adlist ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Adlist
	for rows.Next() {
		var a Adlist
		var enabled int
		var comment, etag sql.NullString
		if err := rows.Scan(&a.ID, &a.Address, &a.Type, &enabled, &comment, &a.Number, &a.InvalidDomains, &a.Status, &etag, &a.DateAdded, &a.DateUpdated); err != nil {
			return nil, err
		}
		a.Enabled = enabled != 0
		a.Comment = comment.String
		a.ETag = etag.String
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *GravityStore) RemoveAdlist(id int64) error {
	_, err := s.db.Exec(`DELETE FROM adlist WHERE id = ?`, id)
	return err
}

func (s *GravityStore) SetAdlistEnabled(id int64, enabled bool) error {
	_, err := s.db.Exec(`UPDATE adlist SET enabled = ?, date_modified = ? WHERE id = ?`, boolToInt(enabled), now(), id)
	return err
}

func (s *GravityStore) UpdateAdlistMeta(id int64, number, invalidDomains int, status, etag string) error {
	_, err := s.db.Exec(
		`UPDATE adlist SET number = ?, invalid_domains = ?, status = ?, etag = ?, date_updated = ?, date_modified = ? WHERE id = ?`,
		number, invalidDomains, status, etag, now(), now(), id,
	)
	return err
}

// ReplaceGravityDomains atomically swaps the compiled domain set for one
// adlist (called once per adlist during a gravity rebuild).
func (s *GravityStore) ReplaceGravityDomains(adlistID int64, domains []string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM gravity WHERE adlist_id = ?`, adlistID); err != nil {
		return fmt.Errorf("clearing old gravity domains: %w", err)
	}

	stmt, err := tx.Prepare(`INSERT OR IGNORE INTO gravity (domain, adlist_id) VALUES (?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, d := range domains {
		if _, err := stmt.Exec(d, adlistID); err != nil {
			return fmt.Errorf("inserting gravity domain %q: %w", d, err)
		}
	}

	return tx.Commit()
}

// --- groups ---

func (s *GravityStore) ListGroups() ([]Group, error) {
	rows, err := s.db.Query(`SELECT id, enabled, name, description FROM "group" ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Group
	for rows.Next() {
		var g Group
		var enabled int
		var desc sql.NullString
		if err := rows.Scan(&g.ID, &enabled, &g.Name, &desc); err != nil {
			return nil, err
		}
		g.Enabled = enabled != 0
		g.Description = desc.String
		out = append(out, g)
	}
	return out, rows.Err()
}

func (s *GravityStore) AddGroup(name, description string) (int64, error) {
	res, err := s.db.Exec(
		`INSERT INTO "group" (enabled, name, description, date_added, date_modified) VALUES (1, ?, ?, ?, ?)`,
		name, description, now(), now(),
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *GravityStore) SetGroupEnabled(id int64, enabled bool) error {
	_, err := s.db.Exec(`UPDATE "group" SET enabled = ?, date_modified = ? WHERE id = ?`, boolToInt(enabled), now(), id)
	return err
}

// --- clients ---

func (s *GravityStore) ListClients() ([]Client, error) {
	rows, err := s.db.Query(`SELECT id, ip, comment FROM client ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Client
	for rows.Next() {
		var c Client
		var comment sql.NullString
		if err := rows.Scan(&c.ID, &c.IP, &comment); err != nil {
			return nil, err
		}
		c.Comment = comment.String
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *GravityStore) AddClient(ip, comment string) (int64, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	res, err := tx.Exec(`INSERT INTO client (ip, comment, date_added, date_modified) VALUES (?, ?, ?, ?)`, ip, comment, now(), now())
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	if _, err := tx.Exec(`INSERT OR IGNORE INTO client_by_group (client_id, group_id) VALUES (?, 0)`, id); err != nil {
		return 0, err
	}
	return id, tx.Commit()
}

// --- snapshot loading for the matcher engine ---

// LoadSnapshot pulls every enabled domain/regex/gravity rule whose owning
// group is enabled, ready for the matcher package to compile into an
// in-memory engine.
func (s *GravityStore) LoadSnapshot() (*Snapshot, error) {
	snap := &Snapshot{
		ExactAllow: map[string]struct{}{},
		ExactDeny:  map[string]string{},
	}

	// Gravity-compiled domains from enabled adlists in enabled groups.
	gravityRows, err := s.db.Query(`
		SELECT g.domain, a.type
		FROM gravity g
		JOIN adlist a ON a.id = g.adlist_id
		JOIN adlist_by_group abg ON abg.adlist_id = a.id
		JOIN "group" gr ON gr.id = abg.group_id
		WHERE a.enabled = 1 AND gr.enabled = 1`)
	if err != nil {
		return nil, fmt.Errorf("loading gravity domains: %w", err)
	}
	for gravityRows.Next() {
		var domain string
		var adlistType int
		if err := gravityRows.Scan(&domain, &adlistType); err != nil {
			gravityRows.Close()
			return nil, err
		}
		if adlistType == AdlistAllow {
			snap.ExactAllow[domain] = struct{}{}
		} else {
			if _, exists := snap.ExactDeny[domain]; !exists {
				snap.ExactDeny[domain] = "gravity"
			}
		}
	}
	gravityRows.Close()
	if err := gravityRows.Err(); err != nil {
		return nil, err
	}

	// Manual domainlist entries (allow-exact, deny-exact, allow-regex, deny-regex).
	domainRows, err := s.db.Query(`
		SELECT dl.id, dl.domain, dl.type
		FROM domainlist dl
		JOIN domainlist_by_group dbg ON dbg.domainlist_id = dl.id
		JOIN "group" gr ON gr.id = dbg.group_id
		WHERE dl.enabled = 1 AND gr.enabled = 1`)
	if err != nil {
		return nil, fmt.Errorf("loading domainlist: %w", err)
	}
	defer domainRows.Close()

	for domainRows.Next() {
		var id int64
		var domain string
		var listType int
		if err := domainRows.Scan(&id, &domain, &listType); err != nil {
			return nil, err
		}
		switch listType {
		case TypeAllowExact:
			snap.ExactAllow[domain] = struct{}{}
		case TypeDenyExact:
			snap.ExactDeny[domain] = "manual" // manual blacklist takes precedence in logging over gravity
		case TypeAllowRegex:
			snap.RegexAllow = append(snap.RegexAllow, RawRegexRule{ID: id, Raw: domain})
		case TypeDenyRegex:
			snap.RegexDeny = append(snap.RegexDeny, RawRegexRule{ID: id, Raw: domain})
		}
	}

	return snap, domainRows.Err()
}

// --- generic key/value metadata (e.g. the hashed admin password) ---

func (s *GravityStore) GetInfo(key string) (string, bool, error) {
	var value string
	err := s.db.QueryRow(`SELECT value FROM info WHERE key = ?`, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return value, true, nil
}

func (s *GravityStore) SetInfo(key, value string) error {
	_, err := s.db.Exec(`INSERT INTO info (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, value)
	return err
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
