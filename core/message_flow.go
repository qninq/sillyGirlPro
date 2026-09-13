package core

import (
	"sync"
	"time"
)

// 消息流水：内存态环形缓冲（重启即清空），记录进入处理的消息、命中的规则/插件
// 与对外回复，供管理后台排查「机器人为什么没回」。内容截断存储，不落盘。
const (
	messageFlowLimit      = 200
	messageFlowContentMax = 120
)

type MessageFlowEntry struct {
	Time     int64  `json:"time"`
	Kind     string `json:"kind"` // in：收到；match：命中规则/插件；out：回复
	Platform string `json:"platform"`
	User     string `json:"user"`
	Chat     string `json:"chat"`
	Content  string `json:"content"`
	Handler  string `json:"handler,omitempty"`
}

var (
	messageFlowMu   sync.Mutex
	messageFlowList []MessageFlowEntry
)

func recordMessageFlow(kind, platform, user, chat, content, handler string) {
	runes := []rune(content)
	if len(runes) > messageFlowContentMax {
		runes = runes[:messageFlowContentMax]
	}
	entry := MessageFlowEntry{
		Time:     time.Now().UnixMilli(),
		Kind:     kind,
		Platform: platform,
		User:     user,
		Chat:     chat,
		Content:  string(runes),
		Handler:  handler,
	}
	messageFlowMu.Lock()
	messageFlowList = append(messageFlowList, entry)
	if len(messageFlowList) > messageFlowLimit {
		messageFlowList = messageFlowList[len(messageFlowList)-messageFlowLimit:]
	}
	messageFlowMu.Unlock()
}

// messageFlowEntries 按时间倒序返回流水（最新在前）。
func messageFlowEntries() []MessageFlowEntry {
	messageFlowMu.Lock()
	defer messageFlowMu.Unlock()
	rows := make([]MessageFlowEntry, 0, len(messageFlowList))
	for i := len(messageFlowList) - 1; i >= 0; i-- {
		rows = append(rows, messageFlowList[i])
	}
	return rows
}
