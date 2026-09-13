package core

import "testing"

func TestAdapterConfigEnabledDefaults(t *testing.T) {
	// 内置适配器未写入 enable 开关时默认关闭（首次启动不拉起全部适配器）。
	for _, platform := range []string{"clawbot", "dingtalk", "flowbot", "pagermaid", "qq", "qqguild", "telegram", "web"} {
		if AdapterConfigEnabled(platform) {
			t.Errorf("AdapterConfigEnabled(%q) = true for unset builtin platform; want false", platform)
		}
	}
	// 插件等自定义平台未配置时默认启用。
	if !AdapterConfigEnabled("custom-plugin-platform") {
		t.Error("custom platform should default to enabled")
	}
	if !AdapterConfigEnabled("") {
		t.Error("empty platform should default to enabled")
	}
}

func TestAdapterEnabledValueDecodesStorageEncoding(t *testing.T) {
	// 用自定义平台验证，避免污染 TestAdapterConfigEnabledDefaults 断言的内置平台状态。
	const platform = "custom-plugin-platform"
	bucket := MakeBucket(platform)

	// 管理后台开关经 SetBucketKeyValue 写入 JSON 布尔值，存储为 b:true/b:false。
	if _, _, err := SetBucketKeyValue(bucket, "enable", true); err != nil {
		t.Fatalf("set enable: %v", err)
	}
	if got := bucket.GetString("enable"); got != "b:true" {
		t.Fatalf("stored boolean = %q; want b:true", got)
	}
	if !AdapterConfigEnabled(platform) {
		t.Error("b:true should enable the platform")
	}

	// 回归：b:false 必须判为关闭，否则界面关闭适配器永远不生效。
	if _, _, err := SetBucketKeyValue(bucket, "enable", false); err != nil {
		t.Fatalf("set enable: %v", err)
	}
	if got := bucket.GetString("enable"); got != "b:false" {
		t.Fatalf("stored boolean = %q; want b:false", got)
	}
	if AdapterConfigEnabled(platform) {
		t.Error("b:false must disable the platform")
	}

	// 存储页手工编辑写入的是裸字符串，同样必须生效。
	for raw, want := range map[string]bool{"false": false, "0": false, "true": true, "b:false": false} {
		if _, _, err := bucket.Set("enable", raw); err != nil {
			t.Fatalf("set enable: %v", err)
		}
		if got := AdapterConfigEnabled(platform); got != want {
			t.Errorf("AdapterEnabledValue(%q) = %v; want %v", raw, got, want)
		}
	}
}

func TestAdapterConfigManageable(t *testing.T) {
	for _, platform := range []string{"clawbot", "dingtalk", "flowbot", "pagermaid", "qq", "qqguild", "telegram", "web"} {
		if !AdapterConfigManageable(platform) {
			t.Errorf("AdapterConfigManageable(%q) = false; want true", platform)
		}
	}
	if AdapterConfigManageable("custom-plugin-platform") {
		t.Error("unknown platform should not be manageable")
	}
}
