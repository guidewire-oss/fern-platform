package database

import (
	"testing"

	"gorm.io/gorm/logger"
)

func TestGormLogLevel(t *testing.T) {
	cases := []struct {
		in   string
		want logger.LogLevel
	}{
		{"", logger.Warn},
		{"warn", logger.Warn},
		{"info", logger.Info},
		{"INFO", logger.Info},
		{" error ", logger.Error},
		{"silent", logger.Silent},
		{"debug", logger.Warn}, // unrecognised: never enable SQL tracing by accident
	}
	for _, tc := range cases {
		if got := gormLogLevel(tc.in); got != tc.want {
			t.Errorf("gormLogLevel(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}
