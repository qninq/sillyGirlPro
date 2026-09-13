package core

import (
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/qninq/sillyGirlPro/utils"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
)

// 系统信息：概览页「系统信息」卡片的数据源——运行时长、CPU/内存占用（带历史
// 迷你曲线）、版本、Go 版本、平台架构、PID、端口、存储后端与数据目录。
const systemInfoHistoryLimit = 30

type systemInfoSample struct {
	CPU    float64 `json:"cpu"`
	Memory float64 `json:"memory"` // 内存占用百分比
}

var (
	systemInfoMu      sync.Mutex
	systemInfoHistory []systemInfoSample
)

func init() {
	GinApi(GET, "/api/admin/system-info", RequireAuth, func(ctx *gin.Context) {
		now := time.Now()
		bootAt := int64(0)
		uptime := int64(0)
		if started := strings.TrimSpace(sillyGirl.GetString("started_at")); started != "" {
			if boot, err := time.ParseInLocation("2006-01-02 15:04:05", started, time.Local); err == nil {
				bootAt = boot.Unix()
				uptime = int64(now.Sub(boot).Seconds())
			}
		}
		var memTotal, memUsed uint64
		if vm, err := mem.VirtualMemory(); err == nil {
			memTotal, memUsed = vm.Total, vm.Used
		}
		port := sillyGirl.GetInt("port", 8080)

		systemInfoMu.Lock()
		latest := systemInfoSample{}
		if len(systemInfoHistory) > 0 {
			latest = systemInfoHistory[len(systemInfoHistory)-1]
		}
		history := append([]systemInfoSample(nil), systemInfoHistory...)
		systemInfoMu.Unlock()

		ApiOK(ctx, gin.H{
			"now":             now.Unix(),
			"boot_at":         bootAt,
			"uptime":          uptime,
			"cpu_percent":     latest.CPU,
			"memory_used":     memUsed,
			"memory_total":    memTotal,
			"memory_percent":  latest.Memory,
			"cpu_history":     cpuHistoryPercents(history),
			"memory_history":  memoryHistoryPercents(history),
			"version":         currentAppVersion(),
			"go_version":      runtime.Version(),
			"os":              runtime.GOOS,
			"arch":            runtime.GOARCH,
			"pid":             os.Getpid(),
			"port":            port,
			"storage_backend": MakeBucket("").Type(),
			"data_dir":        utils.GetDataHome(),
		})
	})
	go systemInfoSampler()
}

// systemInfoSampler 后台每 5 秒采样一次 CPU/内存占用，供曲线与最新值查询。
func systemInfoSampler() {
	for {
		percent, err := cpu.Percent(time.Second, false)
		cpuPercent := -1.0
		if err == nil && len(percent) > 0 {
			cpuPercent = percent[0]
		}
		memPercent := -1.0
		if vm, err := mem.VirtualMemory(); err == nil {
			memPercent = vm.UsedPercent
		}
		systemInfoMu.Lock()
		systemInfoHistory = append(systemInfoHistory, systemInfoSample{
			CPU:    cpuPercent,
			Memory: memPercent,
		})
		if len(systemInfoHistory) > systemInfoHistoryLimit {
			systemInfoHistory = systemInfoHistory[len(systemInfoHistory)-systemInfoHistoryLimit:]
		}
		systemInfoMu.Unlock()
		time.Sleep(5 * time.Second)
	}
}

func cpuHistoryPercents(history []systemInfoSample) []float64 {
	values := make([]float64, 0, len(history))
	for _, sample := range history {
		if sample.CPU >= 0 {
			values = append(values, sample.CPU)
		}
	}
	return values
}

func memoryHistoryPercents(history []systemInfoSample) []float64 {
	values := make([]float64, 0, len(history))
	for _, sample := range history {
		if sample.Memory >= 0 {
			values = append(values, sample.Memory)
		}
	}
	return values
}
