package database

import (
	"errors"
	"io"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	migratedb "github.com/golang-migrate/migrate/v4/database"
	"github.com/golang-migrate/migrate/v4/source"
)

var registerStubDBOnce sync.Once

type stubSource struct {
	openFn     func(string) (source.Driver, error)
	closeFn    func() error
	firstFn    func() (uint, error)
	prevFn     func(uint) (uint, error)
	nextFn     func(uint) (uint, error)
	readUpFn   func(uint) (io.ReadCloser, string, error)
	readDownFn func(uint) (io.ReadCloser, string, error)
}

func (s *stubSource) Open(url string) (source.Driver, error) {
	if s.openFn != nil {
		return s.openFn(url)
	}
	return s, nil
}

func (s *stubSource) Close() error {
	if s.closeFn != nil {
		return s.closeFn()
	}
	return nil
}

func (s *stubSource) First() (uint, error) {
	if s.firstFn != nil {
		return s.firstFn()
	}
	return 0, os.ErrNotExist
}

func (s *stubSource) Prev(version uint) (uint, error) {
	if s.prevFn != nil {
		return s.prevFn(version)
	}
	return 0, os.ErrNotExist
}

func (s *stubSource) Next(version uint) (uint, error) {
	if s.nextFn != nil {
		return s.nextFn(version)
	}
	return 0, os.ErrNotExist
}

func (s *stubSource) ReadUp(version uint) (io.ReadCloser, string, error) {
	if s.readUpFn != nil {
		return s.readUpFn(version)
	}
	return nil, "", os.ErrNotExist
}

func (s *stubSource) ReadDown(version uint) (io.ReadCloser, string, error) {
	if s.readDownFn != nil {
		return s.readDownFn(version)
	}
	return nil, "", os.ErrNotExist
}

type stubDB struct {
	openFn       func(string) (migratedb.Driver, error)
	closeFn      func() error
	lockFn       func() error
	unlockFn     func() error
	runFn        func(io.Reader) error
	setVersionFn func(int, bool) error
	versionFn    func() (int, bool, error)
	dropFn       func() error
}

func (d *stubDB) Open(url string) (migratedb.Driver, error) {
	if d.openFn != nil {
		return d.openFn(url)
	}
	return d, nil
}

func (d *stubDB) Close() error {
	if d.closeFn != nil {
		return d.closeFn()
	}
	return nil
}

func (d *stubDB) Lock() error {
	if d.lockFn != nil {
		return d.lockFn()
	}
	return nil
}

func (d *stubDB) Unlock() error {
	if d.unlockFn != nil {
		return d.unlockFn()
	}
	return nil
}

func (d *stubDB) Run(migration io.Reader) error {
	if d.runFn != nil {
		return d.runFn(migration)
	}
	return nil
}

func (d *stubDB) SetVersion(version int, dirty bool) error {
	if d.setVersionFn != nil {
		return d.setVersionFn(version, dirty)
	}
	return nil
}

func (d *stubDB) Version() (int, bool, error) {
	if d.versionFn != nil {
		return d.versionFn()
	}
	return migratedb.NilVersion, false, nil
}

func (d *stubDB) Drop() error {
	if d.dropFn != nil {
		return d.dropFn()
	}
	return nil
}

func newTestMigrator(t *testing.T, src source.Driver, db migratedb.Driver) *Migrator {
	t.Helper()

	m, err := migrate.NewWithInstance("stub", src, "stub", db)
	if err != nil {
		t.Fatalf("unexpected migrate.NewWithInstance error: %v", err)
	}
	return &Migrator{m: m}
}

func TestMigratorUp_NoChangeIgnored(t *testing.T) {
	src := &stubSource{
		readUpFn: func(uint) (io.ReadCloser, string, error) {
			return nil, "", os.ErrExist
		},
		readDownFn: func(uint) (io.ReadCloser, string, error) {
			return nil, "", os.ErrExist
		},
		nextFn: func(uint) (uint, error) {
			return 0, os.ErrNotExist
		},
	}
	db := &stubDB{
		versionFn: func() (int, bool, error) {
			return 1, false, nil
		},
	}

	m := newTestMigrator(t, src, db)
	if err := m.Up(); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestMigratorDown_NoChangeIgnored(t *testing.T) {
	db := &stubDB{
		versionFn: func() (int, bool, error) {
			return migratedb.NilVersion, false, nil
		},
	}

	m := newTestMigrator(t, &stubSource{}, db)
	if err := m.Down(); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestMigratorUp_ErrorWrapped(t *testing.T) {
	db := &stubDB{
		lockFn: func() error {
			return errors.New("lock failed")
		},
	}

	m := newTestMigrator(t, &stubSource{}, db)
	err := m.Up()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "running migrations") || !strings.Contains(err.Error(), "lock failed") {
		t.Fatalf("expected wrapped error, got %v", err)
	}
}

func TestMigratorVersion_NilVersion(t *testing.T) {
	db := &stubDB{
		versionFn: func() (int, bool, error) {
			return migratedb.NilVersion, false, nil
		},
	}

	m := newTestMigrator(t, &stubSource{}, db)
	version, dirty, err := m.Version()
	if !errors.Is(err, migrate.ErrNilVersion) {
		t.Fatalf("expected ErrNilVersion, got %v", err)
	}
	if version != 0 || dirty {
		t.Fatalf("expected zero version and clean state, got %d dirty=%t", version, dirty)
	}
}

func TestMigratorClose_SourceErrorWins(t *testing.T) {
	srcErr := errors.New("source close failed")
	dbErr := errors.New("db close failed")

	src := &stubSource{
		closeFn: func() error {
			return srcErr
		},
	}
	db := &stubDB{
		closeFn: func() error {
			return dbErr
		},
	}

	m := newTestMigrator(t, src, db)
	if err := m.Close(); err != srcErr {
		t.Fatalf("expected source error, got %v", err)
	}
}

func TestMigratorClose_DatabaseError(t *testing.T) {
	dbErr := errors.New("db close failed")

	src := &stubSource{}
	db := &stubDB{
		closeFn: func() error {
			return dbErr
		},
	}

	m := newTestMigrator(t, src, db)
	if err := m.Close(); err != dbErr {
		t.Fatalf("expected database error, got %v", err)
	}
}

func TestNewMigrator_InvalidDSN(t *testing.T) {
	_, err := NewMigrator("not-a-dsn", "migrations")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "creating migrator") {
		t.Fatalf("expected wrapped error, got %v", err)
	}
}

func TestMigratorDown_ErrorWrapped(t *testing.T) {
	db := &stubDB{
		lockFn: func() error {
			return errors.New("lock failed")
		},
	}

	m := newTestMigrator(t, &stubSource{}, db)
	err := m.Down()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "rolling back migrations") || !strings.Contains(err.Error(), "lock failed") {
		t.Fatalf("expected wrapped error, got %v", err)
	}
}

func TestNewMigrator_Success(t *testing.T) {
	registerStubDBOnce.Do(func() {
		migratedb.Register("stubdbtest", &stubDB{})
	})

	dir := t.TempDir()
	m, err := NewMigrator("stubdbtest://example", dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m == nil {
		t.Fatal("expected migrator")
	}
	if err := m.Close(); err != nil {
		t.Fatalf("unexpected close error: %v", err)
	}
}
