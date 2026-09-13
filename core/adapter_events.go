package core

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// 适配器事件环形缓冲：内存态（重启即清空），记录实例注册/销毁与开关变更，
// 供 BOT 页「事件」面板诊断适配器启停链路。
const adapterEventLimit = 50

type AdapterEvent struct {
	Time int64  `json:"time"`
	Kind string `json:"kind"`
	Text string `json:"text"`
}

var (
	adapterEventsMu sync.Mutex
	adapterEvents   = map[string][]AdapterEvent{}
)

func recordAdapterEvent(platform, kind, format string, args ...interface{}) {
	platform = strings.ToLower(strings.TrimSpace(platform))
	if platform == "" {
		return
	}
	event := AdapterEvent{
		Time: time.Now().Unix(),
		Kind: kind,
		Text: fmt.Sprintf(format, args...),
	}
	adapterEventsMu.Lock()
	events := append(adapterEvents[platform], event)
	if len(events) > adapterEventLimit {
		events = events[len(events)-adapterEventLimit:]
	}
	adapterEvents[platform] = events
	adapterEventsMu.Unlock()
}

func adapterEventRows() map[string][]AdapterEvent {
	adapterEventsMu.Lock()
	defer adapterEventsMu.Unlock()
	rows := make(map[string][]AdapterEvent, len(adapterEvents))
	for platform, events := range adapterEvents {
		copied := make([]AdapterEvent, len(events))
		copy(copied, events)
		rows[platform] = copied
	}
	return rows
}
