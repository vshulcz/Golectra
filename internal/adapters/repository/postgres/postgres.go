package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/vshulcz/Golectra/internal/domain"
	"github.com/vshulcz/Golectra/internal/ports"
)

type Repo struct {
	db *sql.DB
}

var _ ports.MetricsRepo = (*Repo)(nil)

func New(db *sql.DB) *Repo {
	return &Repo{db: db}
}

func (r *Repo) GetGauge(ctx context.Context, n string) (float64, error) {
	const q = `SELECT value FROM metrics WHERE id=$1 AND mtype=$2`
	var v sql.NullFloat64
	if err := r.db.QueryRowContext(ctx, q, n, string(domain.Gauge)).Scan(&v); err != nil || !v.Valid {
		return 0, domain.ErrNotFound
	}
	return v.Float64, nil
}

func (r *Repo) GetCounter(ctx context.Context, n string) (int64, error) {
	const q = `SELECT delta FROM metrics WHERE id=$1 AND mtype=$2`
	var d sql.NullInt64
	if err := r.db.QueryRowContext(ctx, q, n, string(domain.Counter)).Scan(&d); err != nil || !d.Valid {
		return 0, domain.ErrNotFound
	}
	return d.Int64, nil
}

func (r *Repo) SetGauge(ctx context.Context, n string, v float64) error {
	const q = `
INSERT INTO metrics (id, mtype, value, delta, updated_at)
VALUES ($1, $2, $3, NULL, now())
ON CONFLICT (id)
DO UPDATE SET mtype=$2, value=EXCLUDED.value, delta=NULL, updated_at=now();`
	_, err := r.db.ExecContext(ctx, q, n, string(domain.Gauge), v)
	return err
}

func (r *Repo) AddCounter(ctx context.Context, n string, d int64) error {
	const q = `
INSERT INTO metrics (id, mtype, value, delta, updated_at)
VALUES ($1, $2, NULL, $3, now())
ON CONFLICT (id)
DO UPDATE SET mtype=$2, value=NULL, delta=COALESCE(metrics.delta,0)+EXCLUDED.delta, updated_at=now();`
	_, err := r.db.ExecContext(ctx, q, n, string(domain.Counter), d)
	return err
}

func (r *Repo) UpdateMany(ctx context.Context, items []domain.Metrics) error {
	if len(items) == 0 {
		return nil
	}

	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}

	rollback := true
	defer func() {
		if rollback {
			_ = tx.Rollback()
		}
	}()
	const qGauge = `
INSERT INTO metrics (id, mtype, value, delta, updated_at)
VALUES ($1, $2, $3, NULL, now())
ON CONFLICT (id)
DO UPDATE SET mtype=$2, value=EXCLUDED.value, delta=NULL, updated_at=now();`
	const qCounter = `
INSERT INTO metrics (id, mtype, value, delta, updated_at)
VALUES ($1, $2, NULL, $3, now())
ON CONFLICT (id)
DO UPDATE SET mtype=$2, value=NULL, delta=COALESCE(metrics.delta,0)+EXCLUDED.delta, updated_at=now();`
	for _, it := range items {
		switch it.MType {
		case string(domain.Gauge):
			if it.Value == nil {
				continue
			}
			if _, err := tx.ExecContext(ctx, qGauge, it.ID, string(domain.Gauge), *it.Value); err != nil {
				return err
			}
		case string(domain.Counter):
			if it.Delta == nil {
				continue
			}
			if _, err := tx.ExecContext(ctx, qCounter, it.ID, string(domain.Counter), *it.Delta); err != nil {
				return err
			}
		default:
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	rollback = false
	return nil
}

func (r *Repo) Snapshot(ctx context.Context) (domain.Snapshot, error) {
	const q = `SELECT id, mtype, value, delta FROM metrics`
	g := map[string]float64{}
	c := map[string]int64{}

	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return domain.Snapshot{Gauges: g, Counters: c}, err
	}
	defer rows.Close()

	var id, mtype string
	var (
		v sql.NullFloat64
		d sql.NullInt64
	)
	for rows.Next() {
		if err := rows.Scan(&id, &mtype, &v, &d); err != nil {
			continue
		}
		switch mtype {
		case string(domain.Gauge):
			if v.Valid {
				g[id] = v.Float64
			}
		case string(domain.Counter):
			if d.Valid {
				c[id] = d.Int64
			}
		default:
		}
	}
	return domain.Snapshot{Gauges: g, Counters: c}, nil
}

func (r *Repo) Ping(ctx context.Context) error {
	if r.db == nil {
		return errors.New("db not configured")
	}
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	return r.db.PingContext(ctx)
}
