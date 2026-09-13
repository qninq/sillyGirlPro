package core

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// 适配器掉线看门狗：已启用的内置适配器连续多个周期没有任何在线实例时，
// 通过其余在线平台推送给各平台 masters，并在配置了邮箱验证服务时同步发一封
// 告警邮件（所有适配器都离线时邮件是唯一可达的渠道）。每个平台 10 分钟最多告警一次。
const (
	adapterWatchdogInterval  = 60 * time.Second
	adapterWatchdogThreshold = 2
	adapterAlertCooldown     = 10 * time.Minute
)

var (
	adapterWatchdogMu     sync.Mutex
	adapterWatchdogMisses = map[string]int{}
	adapterWatchdogAlerts = map[string]time.Time{}
)

func init() {
	go func() {
		// 等待适配器完成首轮连接（开机拉起、重连退避），再开始巡检，避免启动期误报。
		time.Sleep(3 * adapterWatchdogInterval)
		for {
			checkAdaptersAlive()
			time.Sleep(adapterWatchdogInterval)
		}
	}()
}

func checkAdaptersAlive() {
	for platform := range builtinAdapterPlatforms {
		adapterWatchdogMu.Lock()
		if !AdapterConfigEnabled(platform) || len(GetAdapterBotsID(platform)) > 0 {
			delete(adapterWatchdogMisses, platform)
			adapterWatchdogMu.Unlock()
			continue
		}
		adapterWatchdogMisses[platform]++
		shouldAlert := adapterWatchdogMisses[platform] >= adapterWatchdogThreshold
		if last, ok := adapterWatchdogAlerts[platform]; ok && time.Since(last) < adapterAlertCooldown {
			shouldAlert = false
		}
		if shouldAlert {
			adapterWatchdogAlerts[platform] = time.Now()
		}
		adapterWatchdogMu.Unlock()
		if shouldAlert {
			go alertAdapterOffline(platform)
		}
	}
}

func alertAdapterOffline(platform string) {
	content := fmt.Sprintf(
		"⚠️ 适配器掉线告警：%s 已启用但当前没有任何在线实例，请检查后台 BOT 页与适配器日志。",
		adapterPlatformLabel(platform),
	)
	delivered := notifyAdmins(content, "SillyGirl 适配器掉线告警", platform)
	recordAdapterEvent(platform, "alert", "掉线告警已发出（送达 %d 个渠道）", delivered)
	Logs.Warn("适配器掉线告警：%s（送达 %d 个渠道）", platform, delivered)
}

// notifyAdmins 通过其余在线平台向各平台 masters 推送告警，并在配置了邮箱验证服务时
// 向管理员邮箱（「邮箱主体」）同步发一封告警邮件；skipPlatform 用于排除掉线平台自身。
// 返回送达渠道数。
func notifyAdmins(content, subject, skipPlatform string) int {
	delivered := 0
	for _, plt := range GetAdapterBotPlts() {
		if skipPlatform != "" && strings.EqualFold(plt, skipPlatform) {
			continue
		}
		adapter, err := GetAdapter(plt)
		if err != nil || adapter == nil {
			continue
		}
		for _, master := range strings.Split(strings.Trim(MakeBucket(plt).GetString("masters"), "&"), "&") {
			master = strings.TrimSpace(master)
			if master == "" {
				continue
			}
			if result := adapter.Push(map[string]string{"content": content, "user_id": master}); result["error"] == "" {
				delivered++
			}
		}
	}
	if host, port, password, sender, ok := smtpSettings(); ok {
		body := content + "\r\n\r\n（系统邮件，请勿回复）\r\n"
		if err := sendMail(host, port, password, sender, sender, subject, body); err == nil {
			delivered++
		}
	}
	return delivered
}

func adapterPlatformLabel(platform string) string {
	if label := adapterPlatformLabels[strings.ToLower(strings.TrimSpace(platform))]; label != "" {
		return label
	}
	return platform
}
