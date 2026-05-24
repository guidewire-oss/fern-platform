package database

import (
	"time"

	"gorm.io/gorm"
)

// SlowQueryEvent is a single slow query observation. Sinks log it,
// emit metrics, or both — the plugin doesn't care.
type SlowQueryEvent struct {
	SQL      string
	RowCount int64
	Duration time.Duration
}

// SlowQuerySink consumes slow-query events. Implementations must be
// safe for concurrent use; the GORM plugin invokes this from whatever
// goroutine ran the query.
type SlowQuerySink interface {
	RecordSlowQuery(SlowQueryEvent)
}

// SlowQueryPlugin is a GORM v2 plugin that observes the
// (query, duration) tuple at the end of every operation and forwards
// it to sink when duration >= threshold.
//
// Implementation note: GORM 2 exposes hook points via Callbacks().
// We register an "after" hook on each of the four operation kinds
// (query, create, update, delete) so all SQL is covered.
type SlowQueryPlugin struct {
	threshold time.Duration
	sink      SlowQuerySink
}

// NewSlowQueryPlugin returns a plugin that records queries slower
// than threshold. A nil sink disables the plugin (still installs, so
// callers can wire it unconditionally).
func NewSlowQueryPlugin(threshold time.Duration, sink SlowQuerySink) *SlowQueryPlugin {
	return &SlowQueryPlugin{threshold: threshold, sink: sink}
}

// Name returns the plugin identifier expected by GORM's Use().
func (p *SlowQueryPlugin) Name() string { return "fern:slow_query" }

// Initialize wires before/after hooks on every operation kind.
// The before hook timestamps the request; the after hook computes
// elapsed time and forwards to the sink when it exceeds threshold.
func (p *SlowQueryPlugin) Initialize(db *gorm.DB) error {
	if p.sink == nil {
		return nil
	}
	type hookPair struct {
		kind   string
		before func(name string, fn func(*gorm.DB)) error
		after  func(name string, fn func(*gorm.DB)) error
	}
	pairs := []hookPair{
		{"query",  db.Callback().Query().Before("gorm:query").Register,   db.Callback().Query().After("gorm:after_query").Register},
		{"create", db.Callback().Create().Before("gorm:create").Register, db.Callback().Create().After("gorm:after_create").Register},
		{"update", db.Callback().Update().Before("gorm:update").Register, db.Callback().Update().After("gorm:after_update").Register},
		{"delete", db.Callback().Delete().Before("gorm:delete").Register, db.Callback().Delete().After("gorm:after_delete").Register},
	}
	for _, p2 := range pairs {
		if err := p2.before("fern:slow_query:before_"+p2.kind, markStart); err != nil {
			return err
		}
		if err := p2.after("fern:slow_query:after_"+p2.kind, p.report); err != nil {
			return err
		}
	}
	return nil
}

const startKey = "fern:slow_query:start"

func markStart(tx *gorm.DB) {
	tx.InstanceSet(startKey, time.Now())
}

func (p *SlowQueryPlugin) report(tx *gorm.DB) {
	startVal, ok := tx.InstanceGet(startKey)
	if !ok {
		return
	}
	start, ok := startVal.(time.Time)
	if !ok {
		return
	}
	duration := time.Since(start)
	if duration < p.threshold {
		return
	}
	p.sink.RecordSlowQuery(SlowQueryEvent{
		SQL:      tx.Statement.SQL.String(),
		RowCount: tx.RowsAffected,
		Duration: duration,
	})
}
