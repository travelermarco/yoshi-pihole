package db

import (
	"database/sql"
	"log"
	"sync/atomic"
	"time"
)

// Query outcome status codes, stored in query_log.status.
const (
	StatusUnknown             = 0
	StatusBlockedGravity      = 1
	StatusForwarded           = 2
	StatusBlockedRegex        = 4
	StatusBlockedExact        = 5
	StatusBlockedUpstreamFail = 6
)

// QueryEvent is one logged DNS query, pushed onto QueryStore's channel from
// the DNS hot path.
type QueryEvent struct {
	Timestamp   time.Time
	QType       uint16
	Domain      string
	Client      string
	Status      int
	ReplyType   int
	ReplyTimeMS int64
	Forward     string
	RegexID     *int64
}

// LoggedQuery is a query_log row as returned to API/dashboard callers.
type LoggedQuery struct {
	ID          int64
	Timestamp   int64
	QType       int
	Domain      string
	Client      string
	Status      int
	ReplyType   int
	ReplyTimeMS int64
	Forward     string
	RegexID     sql.NullInt64
}

// QueryStore batches query_log writes on a background goroutine so DNS
// resolution never blocks on a disk write. If the buffer fills (the writer
// falling behind an unusual query burst), new events are dropped and counted
// rather than backing up the resolver.
type QueryStore struct {
	db      *sql.DB
	events  chan QueryEvent
	done    chan struct{}
	stopped chan struct{}
	dropped atomic.Int64
}

const (
	queryBufferSize = 1000
	flushInterval   = 500 * time.Millisecond
	flushBatchSize  = 200
)

func NewQueryStore(sqlDB *sql.DB) *QueryStore {
	qs := &QueryStore{
		db:      sqlDB,
		events:  make(chan QueryEvent, queryBufferSize),
		done:    make(chan struct{}),
		stopped: make(chan struct{}),
	}
	go qs.loop()
	return qs
}

// Log enqueues a query event for asynchronous persistence. Never blocks.
func (qs *QueryStore) Log(ev QueryEvent) {
	select {
	case qs.events <- ev:
	default:
		qs.dropped.Add(1)
	}
}

// Dropped returns how many events were discarded because the buffer was full.
func (qs *QueryStore) Dropped() int64 { return qs.dropped.Load() }

func (qs *QueryStore) Close() {
	close(qs.done)
	<-qs.stopped
}

func (qs *QueryStore) loop() {
	defer close(qs.stopped)
	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()

	batch := make([]QueryEvent, 0, flushBatchSize)
	flush := func() {
		if len(batch) == 0 {
			return
		}
		if err := qs.write(batch); err != nil {
			log.Printf("query_store: flush failed: %v", err)
		}
		batch = batch[:0]
	}

	for {
		select {
		case ev := <-qs.events:
			batch = append(batch, ev)
			if len(batch) >= flushBatchSize {
				flush()
			}
		case <-ticker.C:
			flush()
		case <-qs.done:
			// Drain whatever is still queued before exiting.
			for {
				select {
				case ev := <-qs.events:
					batch = append(batch, ev)
				default:
					flush()
					return
				}
			}
		}
	}
}

func (qs *QueryStore) write(batch []QueryEvent) error {
	tx, err := qs.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO query_log (timestamp, qtype, domain, client, status, reply_type, reply_time_ms, forward, regex_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, ev := range batch {
		var regexID any
		if ev.RegexID != nil {
			regexID = *ev.RegexID
		}
		if _, err := stmt.Exec(ev.Timestamp.Unix(), ev.QType, ev.Domain, ev.Client, ev.Status, ev.ReplyType, ev.ReplyTimeMS, ev.Forward, regexID); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// Filter narrows Recent() results.
type Filter struct {
	Domain string // substring match
	Status *int
	Since  int64 // unix seconds; 0 = no lower bound
}

func (qs *QueryStore) Recent(limit, offset int, f Filter) ([]LoggedQuery, error) {
	query := `SELECT id, timestamp, qtype, domain, client, status, reply_type, reply_time_ms, forward, regex_id FROM query_log WHERE 1=1`
	var args []any

	if f.Domain != "" {
		query += ` AND domain LIKE ?`
		args = append(args, "%"+f.Domain+"%")
	}
	if f.Status != nil {
		query += ` AND status = ?`
		args = append(args, *f.Status)
	}
	if f.Since > 0 {
		query += ` AND timestamp >= ?`
		args = append(args, f.Since)
	}
	query += ` ORDER BY id DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)

	rows, err := qs.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []LoggedQuery
	for rows.Next() {
		var q LoggedQuery
		var forward sql.NullString
		if err := rows.Scan(&q.ID, &q.Timestamp, &q.QType, &q.Domain, &q.Client, &q.Status, &q.ReplyType, &q.ReplyTimeMS, &forward, &q.RegexID); err != nil {
			return nil, err
		}
		q.Forward = forward.String
		out = append(out, q)
	}
	return out, rows.Err()
}

// DomainCount is one row of a top-N domain aggregation.
type DomainCount struct {
	Domain string
	Count  int
}

// Summary computes dashboard stats over queries since sinceUnix.
type Summary struct {
	Total             int
	Blocked           int
	Forwarded         int
	TopDomains        []DomainCount
	TopBlockedDomains []DomainCount
	QueryTypeCounts   map[string]int
}

func (qs *QueryStore) Summary(sinceUnix int64) (Summary, error) {
	var s Summary
	s.QueryTypeCounts = map[string]int{}

	if err := qs.db.QueryRow(`SELECT COUNT(*) FROM query_log WHERE timestamp >= ?`, sinceUnix).Scan(&s.Total); err != nil {
		return s, err
	}
	if err := qs.db.QueryRow(
		`SELECT COUNT(*) FROM query_log WHERE timestamp >= ? AND status IN (?, ?, ?, ?)`,
		sinceUnix, StatusBlockedGravity, StatusBlockedRegex, StatusBlockedExact, StatusBlockedUpstreamFail,
	).Scan(&s.Blocked); err != nil {
		return s, err
	}
	if err := qs.db.QueryRow(`SELECT COUNT(*) FROM query_log WHERE timestamp >= ? AND status = ?`, sinceUnix, StatusForwarded).Scan(&s.Forwarded); err != nil {
		return s, err
	}

	var err error
	s.TopDomains, err = qs.topDomains(sinceUnix, nil)
	if err != nil {
		return s, err
	}
	blockedOnly := true
	s.TopBlockedDomains, err = qs.topDomains(sinceUnix, &blockedOnly)
	if err != nil {
		return s, err
	}

	rows, err := qs.db.Query(`SELECT qtype, COUNT(*) FROM query_log WHERE timestamp >= ? GROUP BY qtype`, sinceUnix)
	if err != nil {
		return s, err
	}
	defer rows.Close()
	for rows.Next() {
		var qtype, count int
		if err := rows.Scan(&qtype, &count); err != nil {
			return s, err
		}
		s.QueryTypeCounts[qtypeName(qtype)] = count
	}

	return s, rows.Err()
}

func (qs *QueryStore) topDomains(sinceUnix int64, blockedOnly *bool) ([]DomainCount, error) {
	query := `SELECT domain, COUNT(*) c FROM query_log WHERE timestamp >= ?`
	args := []any{sinceUnix}
	if blockedOnly != nil && *blockedOnly {
		query += ` AND status IN (?, ?, ?, ?)`
		args = append(args, StatusBlockedGravity, StatusBlockedRegex, StatusBlockedExact, StatusBlockedUpstreamFail)
	}
	query += ` GROUP BY domain ORDER BY c DESC LIMIT 10`

	rows, err := qs.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []DomainCount
	for rows.Next() {
		var dc DomainCount
		if err := rows.Scan(&dc.Domain, &dc.Count); err != nil {
			return nil, err
		}
		out = append(out, dc)
	}
	return out, rows.Err()
}

func qtypeName(t int) string {
	switch t {
	case 1:
		return "A"
	case 28:
		return "AAAA"
	case 5:
		return "CNAME"
	case 15:
		return "MX"
	case 16:
		return "TXT"
	case 2:
		return "NS"
	case 6:
		return "SOA"
	case 33:
		return "SRV"
	case 65:
		return "HTTPS"
	default:
		return "OTHER"
	}
}
