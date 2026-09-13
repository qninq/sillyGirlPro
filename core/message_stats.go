package core

import (
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/qninq/sillyGirlPro/core/storage"
	"github.com/qninq/sillyGirlPro/utils"
)

// 消息量统计：按天累计收到的消息数（总览 + 分平台）。内存累计、每分钟落盘一次，
// 启动时加载当日计数继续累计；保留最近 90 天，供概览页绘制趋势。
const messageStatsRetentionDays = 90

type messageStatsDay struct {
	Date      string           `json:"date"`
	Total     int64            `json:"total"`
	Platforms map[string]int64 `json:"platforms"`
}

var (
	messageStatsMu      sync.Mutex
	messageStatsStorage storage.Bucket
	messageStatsToday   *messageStatsDay
)

func init() {
	messageStatsStorage = MakeBucket("message_stats")
	loadMessageStatsToday()
	go func() {
		for {
			time.Sleep(time.Minute)
			flushMessageStats()
		}
	}()
	pruneMessageStats()
}

func messageStatsDateKey(now time.Time) string {
	return now.Format("2006-01-02")
}

func loadMessageStatsToday() {
	date := messageStatsDateKey(time.Now())
	raw := strings.TrimSpace(messageStatsStorage.GetString(date))
	if raw == "" {
		return
	}
	day := &messageStatsDay{}
	if err := json.Unmarshal([]byte(raw), day); err != nil || day.Date != date {
		return
	}
	messageStatsMu.Lock()
	messageStatsToday = day
	messageStatsMu.Unlock()
}

func flushMessageStats() {
	messageStatsMu.Lock()
	day := messageStatsToday
	messageStatsMu.Unlock()
	if day == nil {
		return
	}
	_, _, _ = messageStatsStorage.Set(day.Date, utils.JsonMarshal(day))
}

func recordMessageStats(platform string) {
	if platform == "" {
		return
	}
	now := time.Now()
	date := messageStatsDateKey(now)
	messageStatsMu.Lock()
	defer messageStatsMu.Unlock()
	if messageStatsToday == nil || messageStatsToday.Date != date {
		// 跨天：先落盘旧数据，再开新的一天。
		if messageStatsToday != nil {
			_, _, _ = messageStatsStorage.Set(messageStatsToday.Date, utils.JsonMarshal(messageStatsToday))
		}
		messageStatsToday = &messageStatsDay{Date: date, Platforms: map[string]int64{}}
	}
	if messageStatsToday.Platforms == nil {
		messageStatsToday.Platforms = map[string]int64{}
	}
	messageStatsToday.Total++
	messageStatsToday.Platforms[platform]++
}

func pruneMessageStats() {
	cutoff := time.Now().AddDate(0, 0, -messageStatsRetentionDays).Format("2006-01-02")
	keys, err := messageStatsStorage.Keys()
	if err != nil {
		return
	}
	for _, key := range keys {
		if strings.TrimSpace(key) < cutoff {
			_, _, _ = messageStatsStorage.Set(key, "")
		}
	}
}

// messageStatsRange 返回最近 days 天（含今天）的消息量，缺失的日期补零。
func messageStatsRange(days int) []messageStatsDay {
	if days <= 0 || days > messageStatsRetentionDays {
		days = 14
	}
	rows := make([]messageStatsDay, 0, days)
	for i := days - 1; i >= 0; i-- {
		date := messageStatsDateKey(time.Now().AddDate(0, 0, -i))
		day := messageStatsDay{Date: date, Platforms: map[string]int64{}}
		if raw := strings.TrimSpace(messageStatsStorage.GetString(date)); raw != "" {
			_ = json.Unmarshal([]byte(raw), &day)
			if day.Platforms == nil {
				day.Platforms = map[string]int64{}
			}
		}
		// 今天的实时计数以内存为准（可能尚未落盘）。
		messageStatsMu.Lock()
		if messageStatsToday != nil && messageStatsToday.Date == date {
			day.Total = messageStatsToday.Total
			day.Platforms = messageStatsToday.Platforms
		}
		messageStatsMu.Unlock()
		rows = append(rows, day)
	}
	return rows
}
