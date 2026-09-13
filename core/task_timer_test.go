package core

import (
	"testing"
	"time"

	"github.com/qninq/sillyGirlPro/core/common"
)

func TestCronNextRun(t *testing.T) {
	now := time.Now()

	// 6 位（含秒）
	next := cronNextRun("0 0 1 * *")
	if next <= int(now.Unix()) {
		t.Fatalf("next = %d; want future", next)
	}
	// 5 位自动补秒域
	next = cronNextRun("30 8,16 * * *")
	if next <= int(now.Unix()) {
		t.Fatalf("next = %d; want future", next)
	}
	// 非法表达式返回 0
	if got := cronNextRun("not-a-cron"); got != 0 {
		t.Fatalf("cronNextRun(invalid) = %d; want 0", got)
	}
	if got := cronNextRun(""); got != 0 {
		t.Fatalf("cronNextRun(empty) = %d; want 0", got)
	}
}

func TestTaskLastRunRecord(t *testing.T) {
	id := "task-last-run-test"
	if got := taskLastRun(id); got != 0 {
		t.Fatalf("fresh task last run = %d; want 0", got)
	}
	recordTaskRun(id)
	if got := taskLastRun(id); got == 0 {
		t.Fatal("recordTaskRun should store the timestamp")
	}
	recordTaskRun("  ")
	if got := taskLastRun(""); got != 0 {
		t.Fatalf("empty id last run = %d; want 0", got)
	}
}

func TestTaskRuleMatched(t *testing.T) {
	old := Functions
	t.Cleanup(func() { Functions = old })
	Functions = []*common.Function{
		{Title: "测试规则", Rules: []string{`^\s*time\s*$`}},
	}
	if !taskRuleMatched(" time ") {
		t.Fatal("taskRuleMatched(` time `) = false; want true")
	}
	if taskRuleMatched("nope") {
		t.Fatal("taskRuleMatched(nope) = true; want false")
	}
	disabled := false
	Functions = []*common.Function{
		{Title: "停用规则", Rules: []string{"time"}, Status: &disabled},
	}
	if taskRuleMatched("time") {
		t.Fatal("disabled plugin rules must not match")
	}
}
