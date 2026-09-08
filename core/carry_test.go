package core

import (
	"testing"

	"github.com/smallfawn/sillyGirl/core/common"
)

func TestMatchCarryGroup(t *testing.T) {
	groups := []CarryGroup{
		{ID: "group-1", Platform: "qq", Enable: true},
		{ID: "session-rid", Platform: "web", Enable: true},
		{ID: "disabled-group", Platform: "qq", Enable: false},
	}
	if got := matchCarryGroup("qq", "group-1", "user-x", groups); got == nil || got.ID != "group-1" {
		t.Fatalf("matchCarryGroup(group chat) = %#v, want group-1", got)
	}
	if got := matchCarryGroup("web", "", "session-rid", groups); got == nil || got.ID != "session-rid" {
		t.Fatalf("matchCarryGroup(private session) = %#v, want session-rid", got)
	}
	if got := matchCarryGroup("web", "", "unknown-rid", groups); got != nil {
		t.Fatalf("matchCarryGroup(unknown) = %#v, want nil", got)
	}
	if got := matchCarryGroup("qq", "group-1", "session-rid", groups); got == nil || got.ID != "group-1" {
		t.Fatalf("group chat must match by chat_id even when user_id differs, got %#v", got)
	}
	if got := matchCarryGroup("qq", "disabled-group", "", groups); got != nil {
		t.Fatalf("disabled group should not match, got %#v", got)
	}
}

func TestParseCarryTargets(t *testing.T) {
	targets := parseCarryTargets([]interface{}{
		map[string]interface{}{"platform": "qq", "chat_id": "10086"},
		map[string]interface{}{"platform": " qqguild ", "chat_id": " open_1 ", "type": " private "},
		map[string]interface{}{"platform": "", "chat_id": "123"},
		map[string]interface{}{"platform": "qq", "chat_id": ""},
		map[string]interface{}{"platform": nil, "chat_id": nil},
		map[string]interface{}{"platform": "qq", "chat_id": "10086", "type": "group"},
		map[string]interface{}{"platform": "qq", "chat_id": "20001", "type": "bogus"},
		"bad-entry",
	})
	if len(targets) != 3 {
		t.Fatalf("parseCarryTargets() = %#v, want 3 targets", targets)
	}
	if targets[0].Platform != "qq" || targets[0].ChatID != "10086" || targets[0].Type != CarryTargetGroup {
		t.Fatalf("targets[0] = %#v, want qq/10086/group", targets[0])
	}
	if targets[1].Platform != "qqguild" || targets[1].ChatID != "open_1" || targets[1].Type != CarryTargetPrivate {
		t.Fatalf("targets[1] = %#v, want qqguild/open_1/private", targets[1])
	}
	if targets[2].ChatID != "20001" || targets[2].Type != CarryTargetGroup {
		t.Fatalf("targets[2] = %#v, want qq/20001/group (bogus type normalized)", targets[2])
	}
}

func TestParseCarryTargetsRejectsInvalidPayload(t *testing.T) {
	if got := parseCarryTargets("bad"); got != nil {
		t.Fatalf("parseCarryTargets(\"bad\") = %#v, want nil", got)
	}
	if got := parseCarryTargets(nil); got != nil {
		t.Fatalf("parseCarryTargets(nil) = %#v, want nil", got)
	}
	if got := parseCarryTargets([]interface{}{map[string]interface{}{"platform": "qq"}}); got != nil {
		t.Fatalf("parseCarryTargets(missing chat_id) = %#v, want nil", got)
	}
}

func TestCanUseAsCarryScriptRejectsRegularNodePluginWithoutCarryMeta(t *testing.T) {
	fn := &common.Function{
		UUID:  "script.js",
		Type:  NODE,
		Rules: []string{"^hello$"},
	}
	if canUseAsCarryScript(fn) {
		t.Fatal("regular Node plugin without @carry should not be available as carry script")
	}
}

func TestCanUseAsCarryScriptAllowsCarryNodePlugin(t *testing.T) {
	fn := &common.Function{
		UUID:  "script.js",
		Type:  NODE,
		Carry: true,
		Rules: []string{"^hello$"},
	}
	if !canUseAsCarryScript(fn) {
		t.Fatal("Node plugin with @carry should be available as carry script")
	}
}

func TestCanUseAsCarryScriptAllowsCarryPythonPlugin(t *testing.T) {
	fn := &common.Function{
		UUID:  "script.py",
		Type:  PYTHON,
		Carry: true,
		Rules: []string{"^hello$"},
	}
	if !canUseAsCarryScript(fn) {
		t.Fatal("Python plugin with @carry should be available as carry script")
	}
}

func TestCanUseAsCarryScriptRejectsLongRunningPlugin(t *testing.T) {
	fn := &common.Function{
		UUID:    "web.js",
		Type:    NODE,
		OnStart: true,
		Web:     true,
	}
	if canUseAsCarryScript(fn) {
		t.Fatal("long-running Node plugin should not be available as carry script")
	}
}

func TestGetAdapterBotsIDReturnsAllWhenPlatformEmpty(t *testing.T) {
	BotsLocker.Lock()
	original := Bots
	Bots = map[Bot]*Factory{
		{"qq", "10001"}:       {},
		{"telegram", "20002"}: {},
	}
	BotsLocker.Unlock()
	defer func() {
		BotsLocker.Lock()
		Bots = original
		BotsLocker.Unlock()
	}()

	all := GetAdapterBotsID("")
	if !Contains(all, "10001") || !Contains(all, "20002") {
		t.Fatalf("GetAdapterBotsID(\"\") = %#v, want all bot IDs", all)
	}
	qq := GetAdapterBotsID("qq")
	if len(qq) != 1 || qq[0] != "10001" {
		t.Fatalf("GetAdapterBotsID(\"qq\") = %#v, want [10001]", qq)
	}
}

func TestGetAdapterAllowsEmptyBotID(t *testing.T) {
	BotsLocker.Lock()
	original := Bots
	qq := &Factory{botplt: "qq", botid: "10001"}
	Bots = map[Bot]*Factory{
		{"qq", "10001"}:       qq,
		{"telegram", "20002"}: {botplt: "telegram", botid: "20002"},
	}
	BotsLocker.Unlock()
	defer func() {
		BotsLocker.Lock()
		Bots = original
		BotsLocker.Unlock()
	}()

	got, err := GetAdapter("qq", "")
	if err != nil {
		t.Fatalf("GetAdapter with empty bot id returned error: %v", err)
	}
	if got != qq {
		t.Fatalf("GetAdapter with empty bot id = %#v, want qq adapter", got)
	}
}

func TestPluginParseCarryMetaWithoutValue(t *testing.T) {
	fn, _ := pluginParse(`/**
 * @title 搬运处理
 * @carry
 */
module.exports = async sender => sender.reply("ok");
`, "carry.js")
	if !fn.Carry {
		t.Fatal("@carry without value should enable carry script")
	}
}

func TestPluginParseCarryFalse(t *testing.T) {
	fn, _ := pluginParse(`/**
 * @title 非搬运处理
 * @carry false
 */
module.exports = async sender => sender.reply("ok");
`, "carry.js")
	if fn.Carry {
		t.Fatal("@carry false should not enable carry script")
	}
}
