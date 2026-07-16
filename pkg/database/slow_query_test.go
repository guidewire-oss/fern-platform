package database_test

import (
	"strings"
	"sync"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/guidewire-oss/fern-platform/pkg/database"
)

type captureSink struct {
	mu     sync.Mutex
	events []database.SlowQueryEvent
}

func (c *captureSink) RecordSlowQuery(e database.SlowQueryEvent) {
	c.mu.Lock()
	c.events = append(c.events, e)
	c.mu.Unlock()
}

func openSQLite(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	return db
}

func TestSlowQueryLogger_RecordsAboveThreshold(t *testing.T) {
	db := openSQLite(t)
	sink := &captureSink{}
	// Threshold is 0 → every query qualifies as "slow", which is the
	// easiest way to assert plumbing without fragile sleeps.
	if err := db.Use(database.NewSlowQueryPlugin(0, sink)); err != nil {
		t.Fatal(err)
	}

	type row struct{ V int }
	if err := db.AutoMigrate(&row{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&row{V: 1}).Error; err != nil {
		t.Fatal(err)
	}

	if len(sink.events) == 0 {
		t.Fatal("expected at least one slow-query event")
	}
	e := sink.events[0]
	if e.Duration <= 0 {
		t.Errorf("duration not captured: %v", e.Duration)
	}
	if !strings.Contains(strings.ToLower(e.SQL), "insert") {
		t.Errorf("SQL not captured: %q", e.SQL)
	}
}

func TestSlowQueryLogger_SkipsBelowThreshold(t *testing.T) {
	db := openSQLite(t)
	sink := &captureSink{}
	// 1h threshold: nothing should trip.
	if err := db.Use(database.NewSlowQueryPlugin(time.Hour, sink)); err != nil {
		t.Fatal(err)
	}

	type row struct{ V int }
	_ = db.AutoMigrate(&row{})
	_ = db.Create(&row{V: 1}).Error

	if len(sink.events) != 0 {
		t.Errorf("nothing should be slow at 1h threshold; got %d events: %+v",
			len(sink.events), sink.events)
	}
}

func TestSlowQueryLogger_NilSinkSafe(t *testing.T) {
	db := openSQLite(t)
	// Nil sink must not panic — the plugin just runs as a no-op.
	if err := db.Use(database.NewSlowQueryPlugin(0, nil)); err != nil {
		t.Fatal(err)
	}
	type row struct{ V int }
	_ = db.AutoMigrate(&row{})
	if err := db.Create(&row{V: 1}).Error; err != nil {
		t.Fatal(err)
	}
}
