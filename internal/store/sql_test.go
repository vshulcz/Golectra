package store

import (
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestSQLStorage_GetGauge(t *testing.T) {
	_, mock, st, done := newMock(t)
	defer done()

	const pat = `SELECT value FROM metrics WHERE id=\$1 AND mtype='gauge'`
	tests := []struct {
		name   string
		setup  func()
		wantV  float64
		wantOK bool
	}{
		{
			"ok",
			func() {
				mock.ExpectQuery(pat).WithArgs("Alloc").
					WillReturnRows(sqlmock.NewRows([]string{"value"}).AddRow(123.45))
			},
			123.45, true,
		},
		{
			"null value",
			func() {
				mock.ExpectQuery(pat).WithArgs("Null").
					WillReturnRows(sqlmock.NewRows([]string{"value"}).AddRow(nil))
			},
			0, false,
		},
		{
			"no rows",
			func() {
				mock.ExpectQuery(pat).WithArgs("Missing").
					WillReturnRows(sqlmock.NewRows([]string{"value"}))
			},
			0, false,
		},
		{
			"db error",
			func() {
				mock.ExpectQuery(pat).WithArgs("Err").
					WillReturnError(errors.New("boom"))
			},
			0, false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.setup()
			got, ok := st.GetGauge(tc.name)
			switch tc.name {
			case "ok":
				got, ok = st.GetGauge("Alloc")
			case "null value":
				got, ok = st.GetGauge("Null")
			case "no rows":
				got, ok = st.GetGauge("Missing")
			case "db error":
				got, ok = st.GetGauge("Err")
			}
			if ok != tc.wantOK || (ok && got != tc.wantV) {
				t.Fatalf("got (%v,%v), want (%v,%v)", got, ok, tc.wantV, tc.wantOK)
			}
		})
	}
}

func TestSQLStorage_GetCounter(t *testing.T) {
	_, mock, st, done := newMock(t)
	defer done()

	const pat = `SELECT delta FROM metrics WHERE id=\$1 AND mtype='counter'`
	tests := []struct {
		name   string
		id     string
		setup  func()
		wantD  int64
		wantOK bool
	}{
		{
			"ok", "Poll", func() {
				mock.ExpectQuery(pat).WithArgs("Poll").
					WillReturnRows(sqlmock.NewRows([]string{"delta"}).AddRow(int64(7)))
			}, 7, true,
		},
		{
			"null", "Null", func() {
				mock.ExpectQuery(pat).WithArgs("Null").
					WillReturnRows(sqlmock.NewRows([]string{"delta"}).AddRow(nil))
			}, 0, false,
		},
		{
			"no rows", "Missing", func() {
				mock.ExpectQuery(pat).WithArgs("Missing").
					WillReturnRows(sqlmock.NewRows([]string{"delta"}))
			}, 0, false,
		},
		{
			"err", "Err", func() {
				mock.ExpectQuery(pat).WithArgs("Err").
					WillReturnError(errors.New("db"))
			}, 0, false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.setup()
			got, ok := st.GetCounter(tc.id)
			if ok != tc.wantOK || (ok && got != tc.wantD) {
				t.Fatalf("got (%v,%v), want (%v,%v)", got, ok, tc.wantD, tc.wantOK)
			}
		})
	}
}

func TestSQLStorage_UpdateGauge(t *testing.T) {
	_, mock, st, done := newMock(t)
	defer done()

	const q = `
INSERT INTO metrics (id, mtype, value, delta, updated_at)
VALUES ($1, 'gauge', $2, NULL, now())
ON CONFLICT (id)
DO UPDATE SET mtype='gauge', value=EXCLUDED.value, delta=NULL, updated_at=now();`

	t.Run("ok", func(t *testing.T) {
		mock.ExpectExec(qm(q)).WithArgs("Alloc", 1.23).
			WillReturnResult(sqlmock.NewResult(0, 1))
		if err := st.UpdateGauge("Alloc", 1.23); err != nil {
			t.Fatalf("UpdateGauge err: %v", err)
		}
	})

	t.Run("error", func(t *testing.T) {
		mock.ExpectExec(qm(q)).WithArgs("Bad", 3.14).
			WillReturnError(errors.New("exec"))
		if err := st.UpdateGauge("Bad", 3.14); err == nil {
			t.Fatalf("expected error")
		}
	})
}

func TestSQLStorage_UpdateCounter(t *testing.T) {
	_, mock, st, done := newMock(t)
	defer done()

	const q = `
INSERT INTO metrics (id, mtype, value, delta, updated_at)
VALUES ($1, 'counter', NULL, $2, now())
ON CONFLICT (id)
DO UPDATE SET mtype='counter', value=NULL, delta=COALESCE(metrics.delta,0)+EXCLUDED.delta, updated_at=now();`

	t.Run("ok", func(t *testing.T) {
		mock.ExpectExec(qm(q)).WithArgs("Poll", int64(5)).
			WillReturnResult(sqlmock.NewResult(0, 1))
		if err := st.UpdateCounter("Poll", 5); err != nil {
			t.Fatalf("UpdateCounter err: %v", err)
		}
	})

	t.Run("error", func(t *testing.T) {
		mock.ExpectExec(qm(q)).WithArgs("Err", int64(2)).
			WillReturnError(errors.New("exec"))
		if err := st.UpdateCounter("Err", 2); err == nil {
			t.Fatalf("expected error")
		}
	})
}

func TestSQLStorage_Snapshot(t *testing.T) {
	_, mock, st, done := newMock(t)
	defer done()

	const pat = `SELECT id, mtype, value, delta FROM metrics`

	rows := sqlmock.NewRows([]string{"id", "mtype", "value", "delta"}).
		AddRow("Alloc", "gauge", 12.5, nil).
		AddRow("PollCount", "counter", nil, int64(9)).
		AddRow("BadRow", "gauge", "oops", nil)
	rows.RowError(2, fmt.Errorf("scan err"))

	mock.ExpectQuery(pat).WillReturnRows(rows)

	g, c := st.Snapshot()
	if g["Alloc"] != 12.5 {
		t.Fatalf("gauge Alloc=%v want 12.5", g["Alloc"])
	}
	if c["PollCount"] != 9 {
		t.Fatalf("counter PollCount=%v want 9", c["PollCount"])
	}
	if _, ok := g["BadRow"]; ok {
		t.Fatalf("BadRow should be skipped on scan error")
	}

	mock.ExpectQuery(pat).WillReturnError(errors.New("db"))
	g2, c2 := st.Snapshot()
	if len(g2) != 0 || len(c2) != 0 {
		t.Fatalf("on error want empty maps, got %v %v", g2, c2)
	}
}

func TestSQLStorage_Ping(t *testing.T) {
	snil := &SQLStorage{}
	if err := snil.Ping(); err == nil {
		t.Fatalf("expected error for nil db")
	}

	_, mock, st, done := newMockWithPing(t)
	defer done()

	mock.ExpectPing().WillReturnError(nil)
	if err := st.Ping(); err != nil {
		t.Fatalf("Ping err: %v", err)
	}

	mock.ExpectPing().WillReturnError(errors.New("down"))
	if err := st.Ping(); err == nil {
		t.Fatalf("expected Ping error")
	}
}

func qm(s string) string {
	return regexp.QuoteMeta(s)
}

func newMock(t *testing.T) (*sql.DB, sqlmock.Sqlmock, *SQLStorage, func()) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	st := &SQLStorage{db: db}
	cleanup := func() {
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
		db.Close()
	}
	return db, mock, st, cleanup
}

func newMockWithPing(t *testing.T) (*sql.DB, sqlmock.Sqlmock, *SQLStorage, func()) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	st := &SQLStorage{db: db}
	cleanup := func() {
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
		_ = db.Close()
	}
	return db, mock, st, cleanup
}
