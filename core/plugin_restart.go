package core

import (
	"fmt"
	"sync"
	"time"

	"github.com/qninq/sillyGirlPro/core/common"
)

// 后台插件（on_start / web，long-lived 进程）崩溃自动重启：
// 进程非零退出视为崩溃，按 5s/10s/20s 退避自动重新拉起，最多连续 3 次；
// 超限后停止重试并通过告警渠道通知管理员。用户停用/重载插件不会触发重启。
const (
	pluginRestartBaseDelay   = 5 * time.Second
	pluginRestartMaxRetries  = 3
	pluginRestartStableAfter = 60 * time.Second
)

var (
	pluginCrashMu     sync.Mutex
	pluginCrashCounts = map[string]int{}
	pluginCrashAlerts = map[string]bool{}
	// backgroundProcesses 记录每个插件当前存活的后台进程，用于判断
	// 退出是否已被重载取代、以及进程是否稳定运行。
	backgroundProcesses sync.Map // uuid -> *exec.Cmd
)

func backgroundProcessStart(uuid string, cmd interface{}) {
	backgroundProcesses.Store(uuid, cmd)
}

func backgroundProcessExit(uuid string, cmd interface{}) {
	if current, ok := backgroundProcesses.Load(uuid); ok && current == cmd {
		backgroundProcesses.Delete(uuid)
	}
}

// backgroundProcessRunning 返回该插件是否仍有后台进程存活（含被重载拉起的新进程）。
func backgroundProcessRunning(uuid string) bool {
	_, ok := backgroundProcesses.Load(uuid)
	return ok
}

// schedulePluginBackgroundRestart 在后台进程异常退出后安排自动重启。
// 用户已停用插件或退出已被重载取代时不动作；连续失败超过上限改为告警。
func schedulePluginBackgroundRestart(f *common.Function, uuid string) {
	if f == nil || !pluginExecutionEnabled(f) {
		return
	}
	if backgroundProcessRunning(uuid) {
		// 已有新的后台进程在跑（重载拉起），本次退出是被取代的旧进程。
		clearPluginCrashState(uuid)
		return
	}

	count, alert, retry := pluginRestartDecision(uuid)
	if !retry {
		if alert {
			content := fmt.Sprintf(
				"⚠️ 插件告警：%s 后台进程连续 %d 次异常退出，已停止自动重启，请到后台插件页检查日志并手动重载。",
				f.Title, pluginRestartMaxRetries,
			)
			delivered := notifyAdmins(content, "SillyGirl 插件告警", "")
			Logs.Warn("插件 %s 连续崩溃告警已发出（送达 %d 个渠道）", f.Title, delivered)
		}
		return
	}

	delay := pluginRestartBaseDelay << (count - 1)
	console.Warn("插件 %s 后台进程异常退出，%v 后自动重启（第 %d/%d 次）", f.Title, delay, count, pluginRestartMaxRetries)
	go func() {
		time.Sleep(delay)
		// 退避期间插件可能被停用或手动重载，重启前重新校验。
		if !pluginExecutionEnabled(f) || backgroundProcessRunning(uuid) {
			return
		}
		console.Log("插件 %s 后台进程已自动重启", f.Title)
		go f.Handle(&CustomSender{
			F: &Factory{
				botplt: "*",
			},
		})
	}()
}

// markPluginBackgroundStable 后台进程稳定运行一段时间后清空崩溃计数，
// 让偶发一次崩溃不会累积到重试上限。
func markPluginBackgroundStable(uuid string, cmd interface{}) {
	go func() {
		time.Sleep(pluginRestartStableAfter)
		if current, ok := backgroundProcesses.Load(uuid); ok && current == cmd {
			clearPluginCrashState(uuid)
		}
	}()
}

func clearPluginCrashState(uuid string) {
	pluginCrashMu.Lock()
	delete(pluginCrashCounts, uuid)
	delete(pluginCrashAlerts, uuid)
	pluginCrashMu.Unlock()
}

// pluginRestartDecision 记录一次崩溃并给出决策：retry（继续退避重启）或
// alert（超过重试上限，发一次告警后不再重试）。
func pluginRestartDecision(uuid string) (count int, alert bool, retry bool) {
	pluginCrashMu.Lock()
	defer pluginCrashMu.Unlock()
	pluginCrashCounts[uuid]++
	count = pluginCrashCounts[uuid]
	if count > pluginRestartMaxRetries {
		alert = !pluginCrashAlerts[uuid]
		pluginCrashAlerts[uuid] = true
		return count, alert, false
	}
	return count, false, true
}
