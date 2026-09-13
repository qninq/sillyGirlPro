package core

import (
	"strings"
	"testing"
)

func TestMessageFlowRingBufferAndOrder(t *testing.T) {
	messageFlowMu.Lock()
	messageFlowList = nil
	messageFlowMu.Unlock()

	recordMessageFlow("in", "qq", "10000", "", "hello", "")
	recordMessageFlow("match", "qq", "10000", "", "hello", "签到插件")
	recordMessageFlow("out", "qq", "10000", "", "签到成功", "")

	rows := messageFlowEntries()
	if len(rows) != 3 {
		t.Fatalf("entries = %d; want 3", len(rows))
	}
	// 最新在前
	if rows[0].Kind != "out" || rows[2].Kind != "in" {
		t.Fatalf("unexpected order: %s / %s", rows[0].Kind, rows[2].Kind)
	}
	if rows[1].Handler != "签到插件" {
		t.Fatalf("handler = %q; want 签到插件", rows[1].Handler)
	}

	// 内容超长截断
	long := strings.Repeat("测", messageFlowContentMax+50)
	recordMessageFlow("in", "qq", "10000", "", long, "")
	rows = messageFlowEntries()
	if got := len([]rune(rows[0].Content)); got != messageFlowContentMax {
		t.Fatalf("truncated content length = %d; want %d", got, messageFlowContentMax)
	}

	// 超过上限后丢弃最旧的
	for i := 0; i < messageFlowLimit; i++ {
		recordMessageFlow("in", "qq", "10000", "", "fill", "")
	}
	rows = messageFlowEntries()
	if len(rows) != messageFlowLimit {
		t.Fatalf("entries = %d; want %d", len(rows), messageFlowLimit)
	}

	messageFlowMu.Lock()
	messageFlowList = nil
	messageFlowMu.Unlock()
}
