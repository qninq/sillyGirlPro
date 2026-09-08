package core

import (
	"encoding/json"
	"testing"
)

func TestReplyDisabledDefaultsToEnabled(t *testing.T) {
	// 旧数据没有 enable 字段，视为启用。
	if (&Reply{}).Disabled() {
		t.Fatal("legacy reply without enable field should be enabled")
	}
	enabled := true
	if (&Reply{Enable: &enabled}).Disabled() {
		t.Fatal("reply with enable=true should be enabled")
	}
	disabled := false
	if !(&Reply{Enable: &disabled}).Disabled() {
		t.Fatal("reply with enable=false should be disabled")
	}
}

func TestReplyUnmarshalLegacyDataKeepsEnabled(t *testing.T) {
	r := Reply{}
	if err := json.Unmarshal([]byte(`{"id":1,"keyword":"hi","value":"hello"}`), &r); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if r.Disabled() {
		t.Fatal("legacy reply JSON without enable field should stay enabled")
	}
}
