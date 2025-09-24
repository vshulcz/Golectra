package postgres

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"net"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jackc/pgerrcode"
	"github.com/lib/pq"
	"github.com/vshulcz/Golectra/internal/domain"
	"github.com/vshulcz/Golectra/internal/misc"
)

func TestRepo_GetGauge(t *testing.T) {
	_, mock, st, done := newMock(t)
	defer done()

	const pat = `SELECT value FROM metrics WHERE id=\$1 AND mtype=\$2`
	tests := []struct {
		name   string
		setup  func()
		wantV  float64
		wantOK bool
	}{
		{
			"ok",
			func() {
				mock.ExpectQuery(pat).WithArgs("Alloc", "gauge").
					WillReturnRows(sqlmock.NewRows([]string{"value"}).AddRow(123.45))
			},
			123.45, true,
		},
		{
			"null value",
			func() {
				mock.ExpectQuery(pat).WithArgs("Null", "gauge").
					WillReturnRows(sqlmock.NewRows([]string{"value"}).AddRow(nil))
			},
			0, false,
		},
		{
			"no rows",
			func() {
				mock.ExpectQuery(pat).WithArgs("Missing", "gauge").
					WillReturnRows(sqlmock.NewRows([]string{"value"}))
			},
			0, false,
		},
		{
			"db error",
			func() {
				mock.ExpectQuery(pat).WithArgs("Err", "gauge").
					WillReturnError(errors.New("boom"))
			},
			0, false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.setup()
			got, err := st.GetGauge(context.TODO(), tc.name)
			switch tc.name {
			case "ok":
				got, err = st.GetGauge(context.TODO(), "Alloc")
			case "null value":
				got, err = st.GetGauge(context.TODO(), "Null")
			case "no rows":
				got, err = st.GetGauge(context.TODO(), "Missing")
			case "db error":
				got, err = st.GetGauge(context.TODO(), "Err")
			default:
			}
			if (err == nil) != tc.wantOK || (err == nil && got != tc.wantV) {
				t.Fatalf("got (%v,%v), want (%v,%v)", got, err, tc.wantV, tc.wantOK)
			}
		})
	}
}

func TestRepo_GetCounter(t *testing.T) {
	_, mock, st, done := newMock(t)
	defer done()

	const pat = `SELECT delta FROM metrics WHERE id=\$1 AND mtype=\$2`
	tests := []struct {
		name   string
		id     string
		setup  func()
		wantD  int64
		wantOK bool
	}{
		{
			"ok", "Poll", func() {
				mock.ExpectQuery(pat).WithArgs("Poll", "counter").
					WillReturnRows(sqlmock.NewRows([]string{"delta"}).AddRow(int64(7)))
			}, 7, true,
		},
		{
			"null", "Null", func() {
				mock.ExpectQuery(pat).WithArgs("Null", "counter").
					WillReturnRows(sqlmock.NewRows([]string{"delta"}).AddRow(nil))
			}, 0, false,
		},
		{
			"no rows", "Missing", func() {
				mock.ExpectQuery(pat).WithArgs("Missing", "counter").
					WillReturnRows(sqlmock.NewRows([]string{"delta"}))
			}, 0, false,
		},
		{
			"err", "Err", func() {
				mock.ExpectQuery(pat).WithArgs("Err", "counter").
					WillReturnError(errors.New("db"))
			}, 0, false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.setup()
			got, err := st.GetCounter(context.TODO(), tc.id)
			if (err == nil) != tc.wantOK || (err != nil && got != tc.wantD) {
				t.Fatalf("got (%v,%v), want (%v,%v)", got, err, tc.wantD, tc.wantOK)
			}
		})
	}
}

func TestRepo_SetGauge(t *testing.T) {
	_, mock, st, done := newMock(t)
	defer done()

	const q = `
INSERT INTO metrics (id, mtype, value, delta, updated_at)
VALUES ($1, $2, $3, NULL, now())
ON CONFLICT (id)
DO UPDATE SET mtype=$2, value=EXCLUDED.value, delta=NULL, updated_at=now();`

	t.Run("ok", func(t *testing.T) {
		mock.ExpectExec(qm(q)).WithArgs("Alloc", "gauge", 1.23).
			WillReturnResult(sqlmock.NewResult(0, 1))
		if err := st.SetGauge(context.TODO(), "Alloc", 1.23); err != nil {
			t.Fatalf("UpdateGauge err: %v", err)
		}
	})

	t.Run("error", func(t *testing.T) {
		mock.ExpectExec(qm(q)).WithArgs("Bad", "gauge", 3.14).
			WillReturnError(errors.New("exec"))
		if err := st.SetGauge(context.TODO(), "Bad", 3.14); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestRepo_AddCounter(t *testing.T) {
	_, mock, st, done := newMock(t)
	defer done()

	const q = `
INSERT INTO metrics (id, mtype, value, delta, updated_at)
VALUES ($1, $2, NULL, $3, now())
ON CONFLICT (id)
DO UPDATE SET mtype=$2, value=NULL, delta=COALESCE(metrics.delta,0)+EXCLUDED.delta, updated_at=now();`

	t.Run("ok", func(t *testing.T) {
		mock.ExpectExec(qm(q)).WithArgs("Poll", "counter", int64(5)).
			WillReturnResult(sqlmock.NewResult(0, 1))
		if err := st.AddCounter(context.TODO(), "Poll", 5); err != nil {
			t.Fatalf("AddCounter err: %v", err)
		}
	})

	t.Run("error", func(t *testing.T) {
		mock.ExpectExec(qm(q)).WithArgs("Err", "counter", int64(2)).
			WillReturnError(errors.New("exec"))
		if err := st.AddCounter(context.TODO(), "Err", 2); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestRepo_Snapshot(t *testing.T) {
	_, mock, st, done := newMock(t)
	defer done()

	const pat = `SELECT id, mtype, value, delta FROM metrics`

	rows := sqlmock.NewRows([]string{"id", "mtype", "value", "delta"}).
		AddRow("Alloc", "gauge", 12.5, nil).
		AddRow("PollCount", "counter", nil, int64(9)).
		AddRow("BadRow", "gauge", "oops", nil)
	rows.RowError(2, errors.New("scan err"))

	mock.ExpectQuery(pat).WillReturnRows(rows)

	s, _ := st.Snapshot(context.TODO())
	if s.Gauges["Alloc"] != 12.5 {
		t.Fatalf("gauge Alloc=%v want 12.5", s.Gauges["Alloc"])
	}
	if s.Counters["PollCount"] != 9 {
		t.Fatalf("counter PollCount=%v want 9", s.Counters["PollCount"])
	}
	if _, ok := s.Gauges["BadRow"]; ok {
		t.Fatal("BadRow should be skipped on scan error")
	}

	mock.ExpectQuery(pat).WillReturnError(errors.New("db"))
	s, _ = st.Snapshot(context.TODO())
	if len(s.Gauges) != 0 || len(s.Counters) != 0 {
		t.Fatalf("on error want empty maps, got %v %v", s.Gauges, s.Counters)
	}
}

func TestRepo_Ping(t *testing.T) {
	snil := &Repo{}
	if err := snil.Ping(context.TODO()); err == nil {
		t.Fatal("expected error for nil db")
	}

	_, mock, st, done := newMockWithPing(t)
	defer done()

	mock.ExpectPing().WillReturnError(nil)
	if err := st.Ping(context.TODO()); err != nil {
		t.Fatalf("Ping err: %v", err)
	}

	mock.ExpectPing().WillReturnError(errors.New("down"))
	if err := st.Ping(context.TODO()); err == nil {
		t.Fatal("expected Ping error")
	}
}

func qm(s string) string {
	return regexp.QuoteMeta(s)
}

func newMock(t *testing.T) (*sql.DB, sqlmock.Sqlmock, *Repo, func()) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	st := &Repo{db: db}
	cleanup := func() {
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
		db.Close()
	}
	return db, mock, st, cleanup
}

func newMockWithPing(t *testing.T) (*sql.DB, sqlmock.Sqlmock, *Repo, func()) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	st := &Repo{db: db}
	cleanup := func() {
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
		_ = db.Close()
	}
	return db, mock, st, cleanup
}

func TestRepo_UpdateMany(t *testing.T) {
	t.Run("mixed success commit", func(t *testing.T) {
		_, mock, st, done := newMock(t)
		defer done()

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

		mock.ExpectBegin()
		mock.ExpectExec(qm(qGauge)).WithArgs("g1", "gauge", 3.14).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(qm(qCounter)).WithArgs("c1", "counter", int64(5)).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(qm(qGauge)).WithArgs("g1", "gauge", 2.71).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(qm(qCounter)).WithArgs("c1", "counter", int64(7)).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()

		items := []domain.Metrics{
			{ID: "g1", MType: "gauge", Value: ptrFloat64(3.14)},
			{ID: "c1", MType: "counter", Delta: ptrInt64(5)},
			{ID: "g1", MType: "gauge", Value: ptrFloat64(2.71)},
			{ID: "c1", MType: "counter", Delta: ptrInt64(7)},
		}
		if err := st.UpdateMany(context.TODO(), items); err != nil {
			t.Fatalf("UpdateMany error: %v", err)
		}
	})

	t.Run("ignores nil/unknown but still commits", func(t *testing.T) {
		_, mock, st, done := newMock(t)
		defer done()

		const qGauge = `
INSERT INTO metrics (id, mtype, value, delta, updated_at)
VALUES ($1, $2, $3, NULL, now())
ON CONFLICT (id)
DO UPDATE SET mtype=$2, value=EXCLUDED.value, delta=NULL, updated_at=now();`

		mock.ExpectBegin()
		mock.ExpectExec(qm(qGauge)).WithArgs("g2", "gauge", 1.0).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()

		items := []domain.Metrics{
			{ID: "g2", MType: "gauge", Value: ptrFloat64(1.0)},
			{ID: "nullg", MType: "gauge", Value: nil},
			{ID: "nullc", MType: "counter", Delta: nil},
			{ID: "weird", MType: "unknown", Value: ptrFloat64(123.0)},
		}
		if err := st.UpdateMany(context.TODO(), items); err != nil {
			t.Fatalf("UpdateMany error: %v", err)
		}
	})

	t.Run("empty slice is no-op (no BEGIN)", func(t *testing.T) {
		_, _, st, done := newMock(t)
		defer done()

		if err := st.UpdateMany(context.TODO(), nil); err != nil {
			t.Fatalf("nil slice: %v", err)
		}
		if err := st.UpdateMany(context.TODO(), []domain.Metrics{}); err != nil {
			t.Fatalf("empty slice: %v", err)
		}
	})

	t.Run("rollback on exec error", func(t *testing.T) {
		_, mock, st, done := newMock(t)
		defer done()

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

		mock.ExpectBegin()
		mock.ExpectExec(qm(qGauge)).WithArgs("g1", "gauge", 10.0).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(qm(qCounter)).WithArgs("c1", "counter", int64(5)).WillReturnError(errors.New("boom"))
		mock.ExpectRollback()

		items := []domain.Metrics{
			{ID: "g1", MType: "gauge", Value: ptrFloat64(10)},
			{ID: "c1", MType: "counter", Delta: ptrInt64(5)},
			{ID: "g2", MType: "gauge", Value: ptrFloat64(20)},
		}
		if err := st.UpdateMany(context.TODO(), items); err == nil {
			t.Fatal("expected error and rollback")
		}
	})
}

func ptrFloat64(v float64) *float64 { return &v }
func ptrInt64(v int64) *int64       { return &v }

func Test_isRetryablePG(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"driver.ErrBadConn", driver.ErrBadConn, true},
		{"net.OpError", &net.OpError{Op: "dial", Err: errors.New("refused")}, true},
		{"pq 08 (ConnectionFailure)", &pq.Error{Code: pq.ErrorCode(pgerrcode.ConnectionFailure)}, true},
		{"pq 08 (ConnectionException)", &pq.Error{Code: pq.ErrorCode(pgerrcode.ConnectionException)}, true},
		{"pq ProtocolViolation", &pq.Error{Code: pq.ErrorCode(pgerrcode.ProtocolViolation)}, true},
		{"pq UniqueViolation (non-retryable)", &pq.Error{Code: pq.ErrorCode(pgerrcode.UniqueViolation)}, false},
		{"generic", errors.New("boom"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isRetryablePG(tt.err); got != tt.want {
				t.Fatalf("isRetryablePG(%T) = %v, want %v", tt.err, got, tt.want)
			}
			if got := IsRetryable(tt.err); got != tt.want {
				t.Fatalf("IsRetryable(%T) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestRepo_GetGauge_Retry(t *testing.T) {
	orig := misc.DefaultBackoff
	misc.DefaultBackoff = []time.Duration{1 * time.Millisecond, 1 * time.Millisecond, 1 * time.Millisecond}
	defer func() { misc.DefaultBackoff = orig }()

	_, mock, st, done := newMock(t)
	defer done()

	const q = `SELECT value FROM metrics WHERE id=\$1 AND mtype=\$2`

	mock.ExpectQuery(q).WithArgs("Alloc", "gauge").
		WillReturnError(&pq.Error{Code: pq.ErrorCode(pgerrcode.ConnectionFailure)})
	mock.ExpectQuery(q).WithArgs("Alloc", "gauge").
		WillReturnRows(sqlmock.NewRows([]string{"value"}).AddRow(100.5))

	got, err := st.GetGauge(context.Background(), "Alloc")
	if err != nil {
		t.Fatalf("GetGauge error: %v", err)
	}
	if got != 100.5 {
		t.Fatalf("GetGauge=%v want 100.5", got)
	}
}

func TestRepo_GetCounter_Retry(t *testing.T) {
	orig := misc.DefaultBackoff
	misc.DefaultBackoff = []time.Duration{1 * time.Millisecond, 1 * time.Millisecond, 1 * time.Millisecond}
	defer func() { misc.DefaultBackoff = orig }()

	_, mock, st, done := newMock(t)
	defer done()

	const q = `SELECT delta FROM metrics WHERE id=\$1 AND mtype=\$2`

	mock.ExpectQuery(q).WithArgs("Poll", "counter").
		WillReturnError(&net.OpError{Op: "read", Err: errors.New("reset")})
	mock.ExpectQuery(q).WithArgs("Poll", "counter").
		WillReturnRows(sqlmock.NewRows([]string{"delta"}).AddRow(int64(7)))

	got, err := st.GetCounter(context.Background(), "Poll")
	if err != nil {
		t.Fatalf("GetCounter error: %v", err)
	}
	if got != 7 {
		t.Fatalf("GetCounter=%v want 7", got)
	}
}

func TestRepo_GetGauge_NoRetry(t *testing.T) {
	_, mock, st, done := newMock(t)
	defer done()

	const q = `SELECT value FROM metrics WHERE id=\$1 AND mtype=\$2`
	mock.ExpectQuery(q).WithArgs("Id", "gauge").
		WillReturnError(&pq.Error{Code: pq.ErrorCode(pgerrcode.UniqueViolation)})

	_, err := st.GetGauge(context.Background(), "Id")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRepo_SetGauge_Retry(t *testing.T) {
	orig := misc.DefaultBackoff
	misc.DefaultBackoff = []time.Duration{1 * time.Millisecond, 1 * time.Millisecond, 1 * time.Millisecond}
	defer func() { misc.DefaultBackoff = orig }()

	_, mock, st, done := newMock(t)
	defer done()

	const q = `
INSERT INTO metrics (id, mtype, value, delta, updated_at)
VALUES ($1, $2, $3, NULL, now())
ON CONFLICT (id)
DO UPDATE SET mtype=$2, value=EXCLUDED.value, delta=NULL, updated_at=now();`

	mock.ExpectExec(qm(q)).WithArgs("Alloc", "gauge", 1.23).
		WillReturnError(&pq.Error{Code: pq.ErrorCode(pgerrcode.ConnectionDoesNotExist)})
	mock.ExpectExec(qm(q)).WithArgs("Alloc", "gauge", 1.23).
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := st.SetGauge(context.Background(), "Alloc", 1.23); err != nil {
		t.Fatalf("SetGauge error: %v", err)
	}
}

func TestRepo_AddCounter_Retr(t *testing.T) {
	orig := misc.DefaultBackoff
	misc.DefaultBackoff = []time.Duration{1 * time.Millisecond, 1 * time.Millisecond, 1 * time.Millisecond}
	defer func() { misc.DefaultBackoff = orig }()

	_, mock, st, done := newMock(t)
	defer done()

	const q = `
INSERT INTO metrics (id, mtype, value, delta, updated_at)
VALUES ($1, $2, NULL, $3, now())
ON CONFLICT (id)
DO UPDATE SET mtype=$2, value=NULL, delta=COALESCE(metrics.delta,0)+EXCLUDED.delta, updated_at=now();`

	mock.ExpectExec(qm(q)).WithArgs("C", "counter", int64(5)).WillReturnError(&net.OpError{Op: "write", Err: errors.New("broken pipe")})
	mock.ExpectExec(qm(q)).WithArgs("C", "counter", int64(5)).WillReturnResult(sqlmock.NewResult(0, 1))

	if err := st.AddCounter(context.Background(), "C", 5); err != nil {
		t.Fatalf("AddCounter error: %v", err)
	}
}

func TestRepo_UpdateMany_Retry(t *testing.T) {
	orig := misc.DefaultBackoff
	misc.DefaultBackoff = []time.Duration{1 * time.Millisecond, 1 * time.Millisecond}
	defer func() { misc.DefaultBackoff = orig }()

	_, mock, st, done := newMock(t)
	defer done()

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

	mock.ExpectBegin().WillReturnError(&pq.Error{Code: pq.ErrorCode(pgerrcode.ConnectionException)})
	mock.ExpectBegin()
	mock.ExpectExec(qm(qGauge)).WithArgs("g", "gauge", 3.14).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(qm(qCounter)).WithArgs("c", "counter", int64(7)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	items := []domain.Metrics{
		{ID: "g", MType: "gauge", Value: ptrFloat64(3.14)},
		{ID: "c", MType: "counter", Delta: ptrInt64(7)},
	}
	if err := st.UpdateMany(context.Background(), items); err != nil {
		t.Fatalf("UpdateMany error: %v", err)
	}
}

func TestRepo_Snapshot_Retry(t *testing.T) {
	orig := misc.DefaultBackoff
	misc.DefaultBackoff = []time.Duration{1 * time.Millisecond}
	defer func() { misc.DefaultBackoff = orig }()

	_, mock, st, done := newMock(t)
	defer done()

	const q = `SELECT id, mtype, value, delta FROM metrics`

	mock.ExpectQuery(q).WillReturnError(&pq.Error{Code: pq.ErrorCode(pgerrcode.ConnectionDoesNotExist)})
	rows := sqlmock.NewRows([]string{"id", "mtype", "value", "delta"}).
		AddRow("Alloc", "gauge", 12.5, nil).
		AddRow("PollCount", "counter", nil, int64(9))
	mock.ExpectQuery(q).WillReturnRows(rows)

	s, err := st.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot error: %v", err)
	}
	if s.Gauges["Alloc"] != 12.5 || s.Counters["PollCount"] != 9 {
		t.Fatalf("unexpected snapshot: %+v", s)
	}
}

func TestRepo_Ping_Retry(t *testing.T) {
	orig := misc.DefaultBackoff
	misc.DefaultBackoff = []time.Duration{1 * time.Millisecond}
	defer func() { misc.DefaultBackoff = orig }()

	_, mock, st, done := newMockWithPing(t)
	defer done()

	mock.ExpectPing().WillReturnError(&pq.Error{Code: pq.ErrorCode(pgerrcode.ConnectionException)})
	mock.ExpectPing().WillReturnError(nil)

	if err := st.Ping(context.Background()); err != nil {
		t.Fatalf("Ping error: %v", err)
	}
}

func TestRepo_GetGauge_ContextCancel(t *testing.T) {
	orig := misc.DefaultBackoff
	misc.DefaultBackoff = []time.Duration{50 * time.Millisecond, 50 * time.Millisecond}
	defer func() { misc.DefaultBackoff = orig }()

	_, mock, st, done := newMock(t)
	defer done()

	const q = `SELECT value FROM metrics WHERE id=\$1 AND mtype=\$2`
	mock.ExpectQuery(q).WithArgs("X", "gauge").
		WillReturnError(driver.ErrBadConn)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := st.GetGauge(ctx, "X")
	if err == nil {
		t.Fatal("expected context-related error")
	}
}
