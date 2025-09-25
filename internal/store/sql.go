package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type SQLStorage struct {
	inner Storage
	db    *sql.DB
}

func NewSQLStorage(inner Storage, db *sql.DB) Storage {
	return &SQLStorage{
		inner: inner,
		db:    db,
	}
}

func (s *SQLStorage) GetGauge(n string) (float64, bool) {
	return s.inner.GetGauge(n)
}
func (s *SQLStorage) GetCounter(n string) (int64, bool) {
	return s.inner.GetCounter(n)
}
func (s *SQLStorage) UpdateGauge(n string, v float64) error {
	return s.inner.UpdateGauge(n, v)
}
func (s *SQLStorage) UpdateCounter(n string, d int64) error {
	return s.inner.UpdateCounter(n, d)
}
func (s *SQLStorage) Snapshot() (map[string]float64, map[string]int64) {
	return s.inner.Snapshot()
}

func (s *SQLStorage) Ping() error {
	if s.db == nil {
		return fmt.Errorf("db not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	return s.db.PingContext(ctx)
}
