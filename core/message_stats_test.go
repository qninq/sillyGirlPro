package core

import (
	"testing"
	"time"
)

func TestMessageStatsRangeFillsDays(t *testing.T) {
	rows := messageStatsRange(7)
	if len(rows) != 7 {
		t.Fatalf("rows = %d; want 7", len(rows))
	}
	// 倒序：最后一行是今天，且日期严格递增
	for i := 1; i < len(rows); i++ {
		if rows[i-1].Date >= rows[i].Date {
			t.Fatalf("dates not ascending: %s >= %s", rows[i-1].Date, rows[i].Date)
		}
	}
	today := messageStatsDateKey(time.Now())
	if rows[len(rows)-1].Date != today {
		t.Fatalf("last date = %s; want today %s", rows[len(rows)-1].Date, today)
	}
}
