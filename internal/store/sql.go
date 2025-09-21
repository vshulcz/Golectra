package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type SQLStorage struct {
	db *sql.DB
}

func NewSQLStorage(db *sql.DB) Storage {
	return &SQLStorage{
		db: db,
	}
}

func (s *SQLStorage) GetGauge(n string) (float64, bool) {
	const query = `SELECT value FROM metrics WHERE id=$1 AND mtype='gauge'`
	var v sql.NullFloat64
	err := s.db.QueryRow(query, n).Scan(&v)
	if err != nil || !v.Valid {
		return 0, false
	}
	return v.Float64, true
}
func (s *SQLStorage) GetCounter(n string) (int64, bool) {
	const query = `SELECT delta FROM metrics WHERE id=$1 AND mtype='counter'`
	var d sql.NullInt64
	err := s.db.QueryRow(query, n).Scan(&d)
	if err != nil || !d.Valid {
		return 0, false
	}
	return d.Int64, true
}
func (s *SQLStorage) UpdateGauge(n string, v float64) error {
	const query = `
INSERT INTO metrics (id, mtype, value, delta, updated_at)
VALUES ($1, 'gauge', $2, NULL, now())
ON CONFLICT (id)
DO UPDATE SET mtype='gauge', value=EXCLUDED.value, delta=NULL, updated_at=now();`
	_, err := s.db.Exec(query, n, v)
	return err
}
func (s *SQLStorage) UpdateCounter(n string, d int64) error {
	const query = `
INSERT INTO metrics (id, mtype, value, delta, updated_at)
VALUES ($1, 'counter', NULL, $2, now())
ON CONFLICT (id)
DO UPDATE SET mtype='counter', value=NULL, delta=COALESCE(metrics.delta,0)+EXCLUDED.delta, updated_at=now();`
	_, err := s.db.Exec(query, n, d)
	return err
}
func (s *SQLStorage) Snapshot() (map[string]float64, map[string]int64) {
	const query = `SELECT id, mtype, value, delta FROM metrics`
	g := map[string]float64{}
	c := map[string]int64{}

	rows, err := s.db.Query(query)
	if err != nil {
		return g, c
	}
	defer rows.Close()

	var id, mtype string
	var (
		val sql.NullFloat64
		dlt sql.NullInt64
	)
	for rows.Next() {
		if err := rows.Scan(&id, &mtype, &val, &dlt); err != nil {
			continue
		}
		switch mtype {
		case "gauge":
			if val.Valid {
				g[id] = val.Float64
			}
		case "counter":
			if dlt.Valid {
				c[id] = dlt.Int64
			}
		}
	}
	return g, c
}

func (s *SQLStorage) Ping() error {
	if s.db == nil {
		return fmt.Errorf("db not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	return s.db.PingContext(ctx)
}
