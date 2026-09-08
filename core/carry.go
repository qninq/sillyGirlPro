package core

import (
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/goccy/go-json"
	"github.com/qninq/sillyGirlPro/core/common"
	"github.com/qninq/sillyGirlPro/core/storage"
	"github.com/qninq/sillyGirlPro/utils"
)

var cgs []CarryGroup

type CarryGroupsResult struct {
	Data  []CarryGroup `json:"data"`
	Page  int          `json:"page"`
	Total int          `json:"total"`
	Time  time.Time    `json:"time"`
}

var CarryGroups = MakeBucket("CarryGroups")

var carryCounter int64

func carryGroupName(cg CarryGroup) string {
	if cg.Remark != "" {
		return cg.Remark
	}
	return cg.ID
}

func syncCarryGroupListen(cg CarryGroup) {
	name := carryGroupName(cg)
	if cg.ID != "" && cg.Enable {
		AddListenOnGroup(cg.ID, fmt.Sprintf("已为搬运群(%s)开启监听模式", name), cg.Platform)
		return
	}
	RemListenOnGroup(cg.ID, fmt.Sprintf("已为搬运群(%s)关闭监听模式", name))
}

func canUseAsCarryScript(function *common.Function) bool {
	return function.UUID != "" && (function.Type == NODE || function.Type == PYTHON) && function.Carry && pluginExecutionEnabled(function) && !function.Module && !function.OnStart && !function.Web
}

type QMessage struct {
	UserID    string        `json:"user_id"`
	Content   string        `json:"content"`
	MessageID string        `json:"message_id"`
	From      common.Sender `json:"-"`
	To        *Factory      `json:"-"`
}

// LOGIC
func initCarry() {
	AddCommand([]*common.Function{
		{
			Rules:    []string{`raw [\s\S]*`},
			Hidden:   true,
			Priority: 9999,
			Handle: func(s common.Sender) interface{} {
				botID := s.GetBotID()
				platform := s.GetImType()
				chatID := s.GetChatID()
				localGroups := cgs
				traceID := fmt.Sprintf("%d. ", atomic.AddInt64(&carryCounter, 1))
				var event = s.Event()
				if event != nil {
					if event["type"] == "delete_message" {
						queues.Range(func(key, value any) bool {
							q := value.(*Queue)
							for _, qm := range q.GetValues() {
								if qm.From != nil && qm.From.GetMessageID() == event["message_id"] {
									qm.To.Sender2(nil).RecallMessage(qm.MessageID)
								}
							}
							return true
						})
					}
					s.Continue()
					return nil
				}
				// 群聊按 chat_id 匹配；单聊会话（Web 会话、C2C 等，无
				// chat_id）按 user_id 匹配，这样私聊渠道也能被监控转发。
				group := matchCarryGroup(platform, chatID, s.GetUserID(), localGroups)
				if group == nil {
					s.Continue()
					return nil
				}
				if len(group.BotsID) != 0 && !Contains(group.BotsID, botID) {
					console.Debug("%s 忽略机器人(%s)消息，搬运群(%s)限定工作机器人%v", traceID, botID, chatID, group.BotsID)
					return nil
				}
				forwardCarryTargets(s, *group)
				if len(group.Scripts) == 0 {
					if len(group.Targets) == 0 {
						console.Debug("%s 搬运群(%s)未配置转发目标或处理脚本", traceID, chatID)
					}
					s.Continue()
					return nil
				}
				console.Debug("%s 搬运群(%s)执行处理脚本%v", traceID, chatID, group.Scripts)
				executed := []string{}
				for _, scriptID := range group.Scripts {
					for _, function := range Functions {
						if function.UUID == scriptID && canUseAsCarryScript(function) && !Contains(executed, function.UUID) {
							function.Handle(s)
							executed = append(executed, function.UUID)
							break
						}
					}
				}
				s.Continue()
				return nil
			},
		},
	})

	setCgs()
	storage.Watch(CarryGroups, nil, func(old, new, key string) *storage.Final {
		console.Log("已更新搬运数据")
		ocg := CarryGroup{}
		ncg := CarryGroup{}
		json.Unmarshal([]byte(old), &ocg)
		json.Unmarshal([]byte(new), &ncg)
		tmp := cgs
		if old != "" {
			if new == "" { // 删除
				if ocg.ID != "" {
					for i, cg := range tmp {
						if cg.ID == ocg.ID {
							tmp = append(tmp[:i], tmp[i+1:]...)
							syncCarryGroupListen(CarryGroup{ID: cg.ID, Remark: carryGroupName(cg)})
							break
						}
					}
				} else {
					return nil
				}
			} else { // 修改
				if ocg.ID != "" {
					for i, cg := range tmp {
						if cg.ID == ocg.ID {
							tmp[i] = ncg
							syncCarryGroupListen(ncg)
							break
						}
					}
				} else {
					return nil
				}
			}
		} else { //创建
			if ncg.ID != "" {
				tmp = append(tmp, ncg)
				syncCarryGroupListen(ncg)
			} else {
				return nil
			}
		}
		sort.Sort(byCreatedAt(tmp))
		for i := range tmp {
			tmp[i].Index = i + 1
		}
		cgs = tmp
		return nil
	})
}

func setCgs() {
	CarryGroups.Foreach(func(b1, b2 []byte) error {
		cg := CarryGroup{}
		err := json.Unmarshal(b2, &cg)
		if err != nil {
			return nil
		}
		syncCarryGroupListen(cg)
		cgs = append(cgs, cg)
		return nil
	})
	sort.Sort(byCreatedAt(cgs))
	for i := range cgs {
		cgs[i].Index = i + 1
	}
}

type CarryGroup struct {
	Index          int      `json:"id"`             //编号 顺序编号
	In             bool     `json:"in"`             //搬进来 勾选按钮
	Out            bool     `json:"out"`            //运出去 勾选按钮
	From           []string `json:"from"`           //采集源
	Allowed        []string `json:"allowed"`        //白名单模式
	Prohibited     []string `json:"prohibited"`     //黑名单模式 Select选择器多选
	ID             string   `json:"chat_id"`        //群组ID 文字表单
	ChatName       string   `json:"chat_name"`      //群昵称 文字表单
	Remark         string   `json:"remark"`         //备注
	Platform       string   `json:"platform"`       //平台 Select选择器单选
	Enable         bool     `json:"enable"`         //启用状态 开关
	Include        []string `json:"include"`        //包含关键词 多个关键词用逗号隔开 用户复制粘贴过去后自动转换成多彩标签
	Exclude        []string `json:"exclude"`        //排除关键词 包含关键词
	CreatedAt      int      `json:"created_at"`     //创建时间戳(秒)转换成日期
	BotsID         []string      `json:"bots_id"`        //工作机器人 多选
	Scripts        []string      `json:"scripts"`        //处理脚本
	Targets        []CarryTarget `json:"targets"`        //转发目标（配置后消息直接转发，无需处理脚本）
	Deduplication  bool          `json:"deduplication"`  //文本去重
	Deduplication2 bool          `json:"deduplication2"` //图片去重
}

// 转发目标类型：群聊走 chat_id 推送，私聊走 user_id 推送。
const (
	CarryTargetGroup   = "group"
	CarryTargetPrivate = "private"
)

type CarryTarget struct {
	Platform string `json:"platform"` //目标平台
	ChatID   string `json:"chat_id"`  //目标 ID（群聊为群号/群 openid，私聊为用户 openid）
	Type     string `json:"type"`     //目标类型：group 群聊 / private 私聊，默认群聊
}

// parseCarryTargets 解析请求里的转发目标列表，过滤空项并去重。
func parseCarryTargets(raw interface{}) []CarryTarget {
	items, ok := raw.([]interface{})
	if !ok {
		return nil
	}
	targets := []CarryTarget{}
	seen := map[string]bool{}
	for _, item := range items {
		entry, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		platform := carryTargetField(entry["platform"])
		chatID := carryTargetField(entry["chat_id"])
		if platform == "" || chatID == "" {
			continue
		}
		targetType := carryTargetField(entry["type"])
		if targetType != CarryTargetPrivate {
			targetType = CarryTargetGroup
		}
		key := targetType + "|" + platform + "|" + chatID
		if seen[key] {
			continue
		}
		seen[key] = true
		targets = append(targets, CarryTarget{Platform: platform, ChatID: chatID, Type: targetType})
	}
	if len(targets) == 0 {
		return nil
	}
	return targets
}

func carryTargetField(value interface{}) string {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case float64:
		return strings.TrimSuffix(fmt.Sprintf("%.0f", v), ".")
	case int:
		return fmt.Sprint(v)
	case int64:
		return fmt.Sprint(v)
	default:
		return ""
	}
}

// matchCarryGroup 返回命中的转发群组：群聊按 chat_id 匹配；chat_id 为空的
// 单聊会话（Web 会话、C2C 私聊等）按 user_id 匹配。
func matchCarryGroup(platform, chatID, userID string, groups []CarryGroup) *CarryGroup {
	for i := range groups {
		g := &groups[i]
		if !g.Enable {
			continue
		}
		if g.Platform != "" && g.Platform != platform {
			continue
		}
		if g.ID == chatID || (chatID == "" && g.ID == userID) {
			return g
		}
	}
	return nil
}

// forwardCarryTargets 把来源消息直接推送到转发目标，返回投递的条数。
func forwardCarryTargets(s common.Sender, group CarryGroup) int {
	if len(group.Targets) == 0 {
		return 0
	}
	content := s.GetContent()
	if strings.TrimSpace(content) == "" {
		return 0
	}
	platform := s.GetImType()
	chatID := s.GetChatID()
	delivered := 0
	for _, target := range group.Targets {
		if target.Type != CarryTargetPrivate && target.Platform == platform && target.ChatID == chatID {
			console.Debug("搬运群(%s)目标与来源相同，跳过 %s/%s", chatID, target.Platform, target.ChatID)
			continue
		}
		adapter, err := GetAdapter(target.Platform)
		if err != nil {
			console.Error("搬运群(%s)转发到 %s/%s(%s) 失败：%v", chatID, target.Platform, target.ChatID, target.Type, err)
			continue
		}
		msg := map[string]string{"content": content, "chat_type": target.Type}
		if target.Type == CarryTargetPrivate {
			msg["user_id"] = target.ChatID
		} else {
			msg["chat_id"] = target.ChatID
		}
		result := adapter.Push(msg)
		if result["error"] != "" {
			console.Error("搬运群(%s)转发到 %s/%s(%s) 失败：%s", chatID, target.Platform, target.ChatID, target.Type, result["error"])
			continue
		}
		delivered++
		if result["message_id"] == "" {
			console.Warn("搬运群(%s)转发到 %s/%s(%s) 未返回消息ID，可能未送达，请检查目标 ID 是否正确及适配器日志", chatID, target.Platform, target.ChatID, target.Type)
			continue
		}
		console.Debug("搬运群(%s)已转发到 %s/%s(%s)（message_id=%s）", chatID, target.Platform, target.ChatID, target.Type, result["message_id"])
	}
	return delivered
}

// CARRY API
func init() {
	GinApi(GET, "/api/admin/carry-groups", RequireAuth, func(ctx *gin.Context) {
		current := utils.Int(ctx.Query("page"))
		pageSize := utils.Int(ctx.Query("page_size"))
		rr := CarryGroupsResult{}
		cgs := cgs
		rr.Total = len(cgs)
		rr.Time = time.Now()
		current, _, begin, end := paginationBounds(current, pageSize, rr.Total)
		rr.Page = current
		rr.Data = cgs[begin:end]
		for i := range rr.Data {
			gn := &Nickname{
				ID: rr.Data[i].ID,
			}
			nickname.First(gn)
			if gn.Value != "" {
				rr.Data[i].ChatName = gn.Value
			}
		}
		ApiList(ctx, rr.Data, rr.Total, map[string]interface{}{"page": rr.Page, "time": rr.Time})
	})
	GinApi(GET, "/api/admin/carry-group-options", RequireAuth, func(ctx *gin.Context) {
		chat_id := ctx.Query("chat_id")
		platform := ctx.Query("platform")
		cgs := cgs
		var bots_id = []string{}
		for _, cg := range cgs {
			if cg.ID == chat_id {
				if platform == "" {
					platform = cg.Platform
				}
			}
		}
		bots_id = GetAdapterBotsID(platform)
		var scripts = map[string]string{}
		functions := Functions
		for _, function := range functions {
			if canUseAsCarryScript(function) {
				scripts[function.UUID] = function.Title + function.Suffix
			}
		}
		ApiOK(ctx, map[string]interface{}{
			"bots_id":   bots_id,
			"platforms": getPltsArray(),
			"scripts":   scripts,
		})
	})
	saveCarryGroup := func(ctx *gin.Context) {
		// 将请求的 JSON 数据解析为一个 map[string]interface{} 类型的变量
		var updateData map[string]interface{}
		err := ctx.ShouldBindJSON(&updateData)
		if err != nil {
			ApiFail(ctx, err.Error())
			return
		}
		pathID := strings.TrimSpace(strings.TrimPrefix(ctx.Param("chat_id"), "/"))
		creating := pathID == ""
		chat_id := pathID
		if chat_id == "" {
			chat_id = strings.TrimSpace(fmt.Sprint(updateData["chat_id"]))
		}
		if chat_id == "" {
			ApiUnprocessable(ctx, "群号不能为空")
			return
		}
		platform := strings.TrimSpace(fmt.Sprint(updateData["platform"]))
		if platform == "" {
			ApiUnprocessable(ctx, "平台不能为空")
			return
		}
		existing := strings.TrimSpace(CarryGroups.GetString(chat_id)) != ""
		if creating && existing {
			ApiConflict(ctx, "搬运群组已存在")
			return
		}
		if !creating && !existing {
			ApiNotFound(ctx, "搬运群组不存在")
			return
		}
		// 编辑时允许修改群号：改存到新群号并删除旧记录。
		renamed := false
		if !creating {
			desiredID := carryTargetField(updateData["chat_id"])
			if desiredID != "" && desiredID != pathID {
				if strings.TrimSpace(CarryGroups.GetString(desiredID)) != "" {
					ApiConflict(ctx, "搬运群组已存在")
					return
				}
				chat_id = desiredID
				renamed = true
			}
		}
		var cg = CarryGroup{
			ID:       chat_id,
			Platform: platform,
			In:       true,
			Enable:   true,
		}
		CarryGroups.First(&cg)
		cg.ID = chat_id
		cg.Platform = platform
		cg.In = true
		cg.Enable = true
		cg.Out = false
		cg.From = nil
		cg.Allowed = nil
		cg.Prohibited = nil
		cg.ChatName = ""
		cg.Include = nil
		cg.Exclude = nil
		cg.Deduplication = false
		cg.Deduplication2 = false
		for key, value := range updateData {
			switch key {
			case "remark":
				if remark, ok := value.(string); ok {
					cg.Remark = remark
				}
			case "bots_id":
				if botsID, ok := value.([]interface{}); ok {
					cg.BotsID = toStringSlice(botsID)
				}
			case "scripts":
				if scripts, ok := value.([]interface{}); ok {
					cg.Scripts = toStringSlice(scripts)
				}
			case "targets":
				cg.Targets = parseCarryTargets(value)
			case "enable":
				if enable, ok := value.(bool); ok {
					cg.Enable = enable
				}
			}
		}
		if cg.CreatedAt == 0 {
			cg.CreatedAt = int(time.Now().Unix())
		}
		_, _, err = CarryGroups.Set(chat_id, utils.JsonMarshal(cg))
		if err == nil && renamed {
			CarryGroups.Set(pathID, "")
		}
		if err != nil {
			ApiInternalError(ctx, err.Error())
			return
		}
		if creating {
			ApiCreated(ctx, "/api/admin/carry-groups/"+url.PathEscape(chat_id), cg)
			return
		}
		ApiOK(ctx, cg)
	}
	GinApi(POST, "/api/admin/carry-groups", RequireAuth, saveCarryGroup)
	GinApi(POST, "/api/admin/carry-groups/:chat_id", RequireAuth, saveCarryGroup)
	GinApi(POST, "/api/admin/carry-groups/:chat_id/deletions", RequireAuth, func(ctx *gin.Context) {
		cg := &CarryGroup{}
		cg.ID = strings.TrimPrefix(ctx.Param("chat_id"), "/")
		if cg.ID == "" {
			ApiUnprocessable(ctx, "群号不能为空")
			return
		}
		if strings.TrimSpace(CarryGroups.GetString(cg.ID)) == "" {
			ApiNotFound(ctx, "搬运群组不存在")
			return
		}
		CarryGroups.Set(cg.ID, "")
		ApiOK(ctx, nil)
	})
}

type byCreatedAt []CarryGroup

func (s byCreatedAt) Len() int {
	return len(s)
}

func (s byCreatedAt) Less(i, j int) bool {
	return s[i].CreatedAt > s[j].CreatedAt
}

func (s byCreatedAt) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

// 将 []interface{} 转为 []string 的工具函数
func toStringSlice(intfSlice []interface{}) []string {
	stringSlice := make([]string, len(intfSlice))
	for i, intf := range intfSlice {
		if str, ok := intf.(string); ok {
			stringSlice[i] = str
		}
	}
	return stringSlice
}

func Contains(strs []string, str ...string) bool {
	for _, s := range str {
		for _, str := range strs {
			if s == str {
				return true
			}
		}
	}
	return false
}

func Include(content string, includes []string) string {
	for _, include := range includes {
		if len(include) > 2 && include[0] == '/' && include[len(include)-1] == '/' {
			pattern := include[1 : len(include)-1]
			_, err := regexp.Compile(pattern)
			if err != nil {
				console.Error("包含词/排除词正则表达式 %s 错误 %s", include, err.Error())
				continue
			}
			match, err := regexp.MatchString(pattern, content)
			if err == nil && match {
				return include
			}
		} else {
			if strings.Contains(content, include) {
				return include
			}
		}
	}
	return ""
}
