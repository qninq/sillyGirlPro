package core

import (
	"strings"

	"github.com/qninq/sillyGirlPro/core/storage"
)

// builtinAdapterPlatforms 是 BOT 管理页展示的内置适配器平台；
// 这些平台未写入 enable 开关时默认关闭，避免首次启动即拉起全部适配器占用资源。
// 插件等自定义平台不受此影响，未配置时仍默认启用。
var builtinAdapterPlatforms = map[string]bool{
	"clawbot":   true,
	"dingtalk":  true,
	"flowbot":   true,
	"pagermaid": true,
	"qq":        true,
	"qqguild":   true,
	"telegram":  true,
	"web":       true,
}

// adapterPlatformLabels 是内置平台的展示名。
var adapterPlatformLabels = map[string]string{
	"clawbot":   "微信 ClawBot",
	"dingtalk":  "钉钉机器人",
	"flowbot":   "FlowBot 微信",
	"pagermaid": "Pagermaid",
	"qq":        "QQ",
	"qqguild":   "QQ 官方频道机器人",
	"web":       "Web Bot",
	"telegram":  "Telegram Bot",
}

func init() {
	for _, platform := range []string{"clawbot", "dingtalk", "pagermaid", "qqguild"} {
		platform := platform
		storage.Watch(MakeBucket(platform), "enable", func(old, new, key string) *storage.Final {
			if !AdapterEnabledValue(new) {
				DestroyAdaptersByPlatform(platform)
			}
			return nil
		})
	}
	// 开关变更事件：全平台记录，供 BOT 页事件面板展示。
	for platform := range builtinAdapterPlatforms {
		platform := platform
		storage.Watch(MakeBucket(platform), "enable", func(old, new, key string) *storage.Final {
			state := "启用"
			if !AdapterEnabledValue(new) {
				state = "停用"
			}
			recordAdapterEvent(platform, "enable", "开关变更为：%s", state)
			return nil
		})
	}
}

func AdapterConfigEnabled(platform string) bool {
	platform = strings.ToLower(strings.TrimSpace(platform))
	if platform == "" {
		return true
	}
	value := MakeBucket(platform).GetString("enable")
	if strings.TrimSpace(value) == "" {
		return !builtinAdapterPlatforms[platform]
	}
	return AdapterEnabledValue(value)
}

// AdapterEnabledValue 解读存储中的 enable 开关值。管理后台写入的是 JSON 布尔值，
// 经 encodeBucketValue 编码为 b:true/b:false（整数编码为 d:0/d:1），判断前必须剥掉
// 类型前缀——否则界面写入的 b:false 会被当成启用，开关永远关不掉。
func AdapterEnabledValue(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.TrimPrefix(value, "b:")
	value = strings.TrimPrefix(value, "d:")
	switch value {
	case "false", "0", "off", "no":
		return false
	default:
		return true
	}
}

func DestroyAdaptersByPlatform(platform string) {
	platform = strings.ToLower(strings.TrimSpace(platform))
	if platform == "" {
		return
	}
	BotsLocker.RLock()
	items := make([]*Factory, 0)
	for key, bot := range Bots {
		if strings.EqualFold(key[0], platform) {
			items = append(items, bot)
		}
	}
	BotsLocker.RUnlock()
	for _, bot := range items {
		bot.Destroy()
	}
}

func AdapterConfigManageable(platform string) bool {
	switch strings.ToLower(strings.TrimSpace(platform)) {
	case "clawbot", "dingtalk", "flowbot", "qq", "qqguild", "telegram", "pagermaid", "web":
		return true
	default:
		return false
	}
}
