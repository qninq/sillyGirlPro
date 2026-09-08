package core

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/smallfawn/sillyGirl/core/common"
	"github.com/smallfawn/sillyGirl/core/storage"
)

// commandAdminBucket persists per-command admin overrides so that the
// restriction can be toggled from the web console without editing scripts.
// Key: commandKey(function); value: "true"/"false".
var commandAdminBucket = MakeBucket("command_admin")

// commandAdminSeen remembers which command keys already had overrides applied,
// so repeated registrations (plugin reloads) stay cheap.
var commandAdminSeen sync.Map

// commandKey returns a stable identifier for a command. Plugins use their
// UUID; built-in Go commands derive a deterministic key from title and rules.
func commandKey(f *common.Function) string {
	if f == nil {
		return ""
	}
	if f.UUID != "" {
		return f.UUID
	}
	sum := sha1.Sum([]byte(f.Title + "\x00" + strings.Join(f.Rules, "\x1f")))
	return "go_" + hex.EncodeToString(sum[:])
}

// applyCommandAdminOverride applies a persisted admin restriction override to
// a freshly registered command. It runs on every registration path
// (AddCommand for Go built-ins and addNodePluginLocked for scripts), so
// overrides survive restarts and plugin reloads.
func applyCommandAdminOverride(f *common.Function) {
	if f == nil {
		return
	}
	key := commandKey(f)
	if key == "" {
		return
	}
	if _, loaded := commandAdminSeen.Load(key); loaded {
		return
	}
	value := strings.TrimSpace(commandAdminBucket.GetString(key))
	if value == "" {
		commandAdminSeen.Store(key, struct{}{})
		return
	}
	f.Admin = value == "true"
	commandAdminSeen.Store(key, struct{}{})
}

func commandOrigin(f *common.Function) string {
	switch strings.ToLower(strings.TrimSpace(f.Type)) {
	case NODE, PYTHON:
		return "plugin"
	default:
		return "builtin"
	}
}

func commandSourceLabel(f *common.Function) string {
	if commandOrigin(f) == "builtin" {
		return "内置"
	}
	identity := nodePluginIdentityFromPath(f.Path)
	parts := strings.Split(filepath.ToSlash(identity), "/")
	name := parts[len(parts)-1]
	publisher := ""
	if len(parts) >= 2 {
		publisher = parts[0]
	}
	if publisher == "" || publisher == "local" {
		return name + filepath.Ext(f.Path)
	}
	return publisher + " · " + name + filepath.Ext(f.Path)
}

func commandPluginID(f *common.Function) string {
	if commandOrigin(f) != "plugin" {
		return ""
	}
	return nodePluginIdentityFromPath(f.Path)
}

type commandListItem struct {
	Key       string   `json:"key"`
	Title     string   `json:"title"`
	Desc      string   `json:"desc"`
	Rules     []string `json:"rules"`
	Admin     bool     `json:"admin"`
	Status    bool     `json:"status"`
	Origin    string   `json:"origin"`
	Source    string   `json:"source"`
	PluginID  string   `json:"plugin_id,omitempty"`
	Type      string   `json:"type"`
	Class     string   `json:"class"`
	Author    string   `json:"author"`
	Version   string   `json:"version"`
	Carry     bool     `json:"carry"`
	CronCount int      `json:"cron_count"`
	Readonly  bool     `json:"readonly"`
}

// builtinStaticCommands mirrors the hardcoded admin commands in the message
// consumer loop (function.go switch on content). They bypass the Functions
// registry, so the command list must declare them explicitly; their admin
// restriction is hardcoded and cannot be toggled.
var builtinStaticCommands = []commandListItem{
	{
		Key: "builtin_group_listen", Title: "listen", Admin: true, Status: true,
		Origin: "builtin", Source: "内置", Readonly: true,
		Desc:  "群聊：开启当前群的消息监听",
		Rules: []string{"^listen$"},
	},
	{
		Key: "builtin_group_unlisten", Title: "unlisten / nolisten", Admin: true, Status: true,
		Origin: "builtin", Source: "内置", Readonly: true,
		Desc:  "群聊：关闭当前群的消息监听",
		Rules: []string{"^unlisten$", "^nolisten$"},
	},
	{
		Key: "builtin_group_reply", Title: "reply", Admin: true, Status: true,
		Origin: "builtin", Source: "内置", Readonly: true,
		Desc:  "群聊：开启当前群的回复",
		Rules: []string{"^reply$"},
	},
	{
		Key: "builtin_group_noreply", Title: "noreply / unreply", Admin: true, Status: true,
		Origin: "builtin", Source: "内置", Readonly: true,
		Desc:  "群聊：关闭当前群的回复",
		Rules: []string{"^noreply$", "^unreply$"},
	},
	{
		Key: "builtin_user_listen", Title: "listen", Admin: true, Status: true,
		Origin: "builtin", Source: "内置", Readonly: true,
		Desc:  "私聊：取消屏蔽当前用户的消息",
		Rules: []string{"^listen$"},
	},
	{
		Key: "builtin_user_unlisten", Title: "unlisten / nolisten", Admin: true, Status: true,
		Origin: "builtin", Source: "内置", Readonly: true,
		Desc:  "私聊：屏蔽当前用户的消息",
		Rules: []string{"^unlisten$", "^nolisten$"},
	},
}

func init() {
	GinApi(GET, "/api/admin/command-list", RequireAuth, func(ctx *gin.Context) {
		functions := installedPluginSnapshot()
		items := make([]commandListItem, 0, len(functions))
		for _, f := range functions {
			if f == nil || f.Module || f.Hidden || len(f.Rules) == 0 {
				continue
			}
			status := f.Status == nil || *f.Status
			item := commandListItem{
				Key:       commandKey(f),
				Title:     strings.TrimSpace(f.Title),
				Desc:      strings.TrimSpace(f.Desc),
				Rules:     append([]string(nil), f.Rules...),
				Admin:     f.Admin,
				Status:    status,
				Origin:    commandOrigin(f),
				Source:    commandSourceLabel(f),
				PluginID:  commandPluginID(f),
				Type:      strings.TrimSpace(f.Type),
				Class:     strings.TrimSpace(f.Class),
				Author:    strings.TrimSpace(f.Author),
				Version:   strings.TrimSpace(f.Version),
				Carry:     f.Carry,
				CronCount: len(f.Cron),
			}
			if item.Title == "" {
				item.Title = item.Source
			}
			items = append(items, item)
		}
		items = append(items, builtinStaticCommands...)
		sort.SliceStable(items, func(i, j int) bool {
			if items[i].Origin != items[j].Origin {
				return items[i].Origin == "builtin"
			}
			return strings.ToLower(items[i].Title) < strings.ToLower(items[j].Title)
		})
		builtin := 0
		plugin := 0
		adminOnly := 0
		for _, item := range items {
			if item.Origin == "builtin" {
				builtin++
			} else {
				plugin++
			}
			if item.Admin {
				adminOnly++
			}
		}
		ApiOK(ctx, map[string]interface{}{
			"list": items,
			"stats": map[string]int{
				"builtin":   builtin,
				"plugin":    plugin,
				"adminOnly": adminOnly,
			},
		})
	})

	GinApi(POST, "/api/admin/command-list/:key/admin", RequireAuth, func(ctx *gin.Context) {
		key := strings.TrimSpace(ctx.Param("key"))
		if key == "" {
			ApiUnprocessable(ctx, "指令标识不能为空")
			return
		}
		payload := map[string]interface{}{}
		if err := ctx.ShouldBindJSON(&payload); err != nil {
			ApiFail(ctx, err.Error())
			return
		}
		admin, ok := payload["admin"].(bool)
		if !ok {
			ApiUnprocessable(ctx, "admin 字段必须为布尔值")
			return
		}
		for _, static := range builtinStaticCommands {
			if static.Key == key {
				ApiUnprocessable(ctx, "内置命令的管理员限制由核心硬编码，不可修改")
				return
			}
		}
		target := findFunctionByKey(key)
		if target == nil {
			ApiNotFound(ctx, "指令不存在或已卸载")
			return
		}
		target.Admin = admin
		commandAdminBucket.Set(key, fmt.Sprint(admin))
		commandAdminSeen.Store(key, struct{}{})
		ApiOK(ctx, map[string]interface{}{"key": key, "admin": admin})
	})
}

// findFunctionByKey locates the live Function pointer matching a command key.
// The returned pointer backs message dispatch, so mutating Admin takes effect
// immediately.
func findFunctionByKey(key string) *common.Function {
	pluginLock.Lock()
	defer pluginLock.Unlock()
	for _, f := range Functions {
		if f != nil && commandKey(f) == key {
			return f
		}
	}
	return nil
}

func init() {
	storage.Watch(commandAdminBucket, nil, func(old, new, key string) *storage.Final {
		commandAdminSeen.Delete(key)
		return nil
	})
}
