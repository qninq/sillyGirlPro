package core

import (
	"sync"
	"time"
)

// 告警中心：内存态聚合最近触发的系统告警（适配器掉线、插件连续崩溃等），
// 供管理后台顶栏铃铛查看。推送/邮件负责送达，这里负责留痕，重启即清空。
const systemAlertLimit = 100

type SystemAlert struct {
	Time      int64  `json:"time"`
	Kind      string `json:"kind"` // adapter-offline / plugin-crash
	Title     string `json:"title"`
	Content   string `json:"content"`
	Delivered int    `json:"delivered"`
}

var (
	systemAlertMu   sync.Mutex
	systemAlertList []SystemAlert
)

func recordSystemAlert(kind, title, content string, delivered int) {
	alert := SystemAlert{
		Time:      time.Now().Unix(),
		Kind:      kind,
		Title:     title,
		Content:   content,
		Delivered: delivered,
	}
	systemAlertMu.Lock()
	systemAlertList = append(systemAlertList, alert)
	if len(systemAlertList) > systemAlertLimit {
		systemAlertList = systemAlertList[len(systemAlertList)-systemAlertLimit:]
	}
	systemAlertMu.Unlock()
}

// systemAlertEntries 按时间倒序返回（最新在前）。
func systemAlertEntries() []SystemAlert {
	systemAlertMu.Lock()
	defer systemAlertMu.Unlock()
	rows := make([]SystemAlert, 0, len(systemAlertList))
	for i := len(systemAlertList) - 1; i >= 0; i-- {
		rows = append(rows, systemAlertList[i])
	}
	return rows
}
