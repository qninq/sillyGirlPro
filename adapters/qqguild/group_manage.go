package qqguild

// 群管理能力：基于官方 2026-08/09 更新的群管理接口
// （禁言、成员、黑名单、入群申请审批与自动审批策略）。
// 注意：成员列表/成员信息/批量移除/黑名单接口当前处于官方内邀阶段，
// 未开通的机器人会收到 11253 错误码，动作返回中会透出该信息。

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/qninq/sillyGirlPro/core"
	"github.com/tencent-connect/botgo/dto"
)

const (
	manageActionTimeout = 12 * time.Second

	// GROUP_MEMBER intent：群成员进退事件（GROUP_MEMBER_ADD / GROUP_MEMBER_REMOVE）。
	intentGroupMember = 1 << 24
)

// qqAPIError 表示 HTTP 成功但响应体携带 err_code 的业务失败。
type qqAPIError struct {
	Code    int
	Message string
}

func (e *qqAPIError) Error() string {
	return fmt.Sprintf("err_code=%d %s", e.Code, e.Message)
}

// restRequest 通过 botgo Transport 调用官方 REST 接口，统一处理 err_code 业务错误。
func (b *bot) restRequest(ctx context.Context, method, endpoint string, body interface{}) (map[string]interface{}, error) {
	requestCtx, cancel := context.WithTimeout(ctx, manageActionTimeout)
	defer cancel()
	raw, err := b.api.Transport(requestCtx, method, b.apiBase()+endpoint, body)
	if err != nil {
		return nil, err
	}
	resp := map[string]interface{}{}
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		return resp, nil
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("解析群管理响应失败：%w", err)
	}
	if code := intValue(resp["err_code"]); code != 0 && code != 201 && code != 202 {
		return resp, &qqAPIError{Code: code, Message: stringValue(resp["message"])}
	}
	return resp, nil
}

func intValue(value interface{}) int {
	switch current := value.(type) {
	case float64:
		return int(current)
	case int:
		return current
	case json.Number:
		result, _ := current.Int64()
		return int(result)
	case string:
		return atoiDefault(current)
	default:
		return 0
	}
}

func atoiDefault(text string) int {
	value := 0
	for _, r := range strings.TrimSpace(text) {
		if r < '0' || r > '9' {
			return 0
		}
		value = value*10 + int(r-'0')
	}
	return value
}

func firstString(data map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(stringValue(data[key])); value != "" {
			return value
		}
	}
	return ""
}

func encodePath(value string) string {
	return url.PathEscape(strings.TrimSpace(value))
}

// ---- REST 封装 ----

// groupInfo GET /v2/groups/{group_openid}/info（30 QPM，内邀）
func (b *bot) groupInfo(ctx context.Context, groupOpenid string) (map[string]interface{}, error) {
	return b.restRequest(ctx, http.MethodGet, "/v2/groups/"+encodePath(groupOpenid)+"/info", nil)
}

// groupBotState GET /v2/groups/{group_openid}/bot_state
func (b *bot) groupBotState(ctx context.Context, groupOpenid string) (map[string]interface{}, error) {
	return b.restRequest(ctx, http.MethodGet, "/v2/groups/"+encodePath(groupOpenid)+"/bot_state", nil)
}

// groupMembers GET /v2/groups/{group_openid}/members（每页最多 30 条，60 QPM，内邀）
func (b *bot) groupMembers(ctx context.Context, groupOpenid, cursor string) (map[string]interface{}, error) {
	endpoint := "/v2/groups/" + encodePath(groupOpenid) + "/members"
	if cursor != "" {
		endpoint += "?cursor=" + url.QueryEscape(cursor)
	}
	return b.restRequest(ctx, http.MethodGet, endpoint, nil)
}

// groupMemberInfo GET /v2/groups/{group_openid}/members/{member_openid}（内邀）
func (b *bot) groupMemberInfo(ctx context.Context, groupOpenid, memberOpenid string) (map[string]interface{}, error) {
	return b.restRequest(ctx, http.MethodGet,
		"/v2/groups/"+encodePath(groupOpenid)+"/members/"+encodePath(memberOpenid), nil)
}

// groupRemoveMembers POST /v2/groups/{group_openid}/batch_remove_members（单次最多 20 人，内邀）
func (b *bot) groupRemoveMembers(ctx context.Context, groupOpenid string, memberOpenids []string, addToBlacklist bool) (map[string]interface{}, error) {
	return b.restRequest(ctx, http.MethodPost, "/v2/groups/"+encodePath(groupOpenid)+"/batch_remove_members", map[string]interface{}{
		"member_openids":           memberOpenids,
		"add_to_member_blacklist":  addToBlacklist,
	})
}

// groupBlacklist GET /v2/groups/{group_openid}/member_blacklist（默认 20 条，最大 100，内邀）
func (b *bot) groupBlacklist(ctx context.Context, groupOpenid, cursor string, limit int) (map[string]interface{}, error) {
	query := url.Values{}
	if cursor != "" {
		query.Set("cursor", cursor)
	}
	if limit > 0 {
		query.Set("limit", fmt.Sprint(limit))
	}
	endpoint := "/v2/groups/" + encodePath(groupOpenid) + "/member_blacklist"
	if encoded := query.Encode(); encoded != "" {
		endpoint += "?" + encoded
	}
	return b.restRequest(ctx, http.MethodGet, endpoint, nil)
}

// groupBlacklistUpdate POST /v2/groups/{group_openid}/member_blacklist（op: add/remove，单次最多 20 人，内邀）
func (b *bot) groupBlacklistUpdate(ctx context.Context, groupOpenid, op string, memberOpenids []string) (map[string]interface{}, error) {
	return b.restRequest(ctx, http.MethodPost, "/v2/groups/"+encodePath(groupOpenid)+"/member_blacklist", map[string]interface{}{
		"op":            op,
		"member_openids": memberOpenids,
	})
}

// groupJoinRequests GET /v2/groups/{group_openid}/join_request_list（limit 默认 20，最大 100）
func (b *bot) groupJoinRequests(ctx context.Context, groupOpenid, cursor string, limit int) (map[string]interface{}, error) {
	query := url.Values{}
	if cursor != "" {
		query.Set("cursor", cursor)
	}
	if limit > 0 {
		query.Set("limit", fmt.Sprint(limit))
	}
	endpoint := "/v2/groups/" + encodePath(groupOpenid) + "/join_request_list"
	if encoded := query.Encode(); encoded != "" {
		endpoint += "?" + encoded
	}
	return b.restRequest(ctx, http.MethodGet, endpoint, nil)
}

// groupApproveJoinRequest POST /v2/groups/{group_openid}/approval_join_request/{member_openid}
func (b *bot) groupApproveJoinRequest(ctx context.Context, groupOpenid, memberOpenid, op, requestID, reason string, addToBlacklist bool) (map[string]interface{}, error) {
	body := map[string]interface{}{"op": op}
	if requestID != "" {
		body["join_request_id"] = requestID
	}
	if reason != "" {
		body["reject_reason"] = reason
	}
	if addToBlacklist {
		body["add_to_member_blacklist"] = true
	}
	return b.restRequest(ctx, http.MethodPost,
		"/v2/groups/"+encodePath(groupOpenid)+"/approval_join_request/"+encodePath(memberOpenid), body)
}

// groupMuteSetting GET /v2/groups/{group_openid}/restrict_chat_setting（查询禁言状态）
func (b *bot) groupMuteSetting(ctx context.Context, groupOpenid string) (map[string]interface{}, error) {
	return b.restRequest(ctx, http.MethodGet, "/v2/groups/"+encodePath(groupOpenid)+"/restrict_chat_setting", nil)
}

// groupSetMute POST /v2/groups/{group_openid}/restrict_chat_setting
// members: [{op: add|update|del, member_openid, mute_expire_at(RFC3339)}]，单次最多 10 人，仅普通成员。
func (b *bot) groupSetMute(ctx context.Context, groupOpenid string, members []interface{}) (map[string]interface{}, error) {
	return b.restRequest(ctx, http.MethodPost, "/v2/groups/"+encodePath(groupOpenid)+"/restrict_chat_setting", map[string]interface{}{
		"members": members,
	})
}

// ---- 入群自动审批策略（平台托管，最多 20 个策略，每个策略最多关联 100 个群）----

func (b *bot) joinStrategyList(ctx context.Context, limit int) (map[string]interface{}, error) {
	endpoint := "/v2/groups/join_approval_strategy"
	if limit > 0 {
		endpoint += "?limit=" + fmt.Sprint(limit)
	}
	return b.restRequest(ctx, http.MethodGet, endpoint, nil)
}

func (b *bot) joinStrategyCreate(ctx context.Context, groupOpenids []string, enable, remark string) (map[string]interface{}, error) {
	body := map[string]interface{}{"group_openids": groupOpenids, "is_enable": enable}
	if remark != "" {
		body["remark"] = remark
	}
	return b.restRequest(ctx, http.MethodPost, "/v2/groups/join_approval_strategy", body)
}

func (b *bot) joinStrategyUpdate(ctx context.Context, strategyID string, body map[string]interface{}) (map[string]interface{}, error) {
	return b.restRequest(ctx, http.MethodPatch, "/v2/groups/join_approval_strategy/"+encodePath(strategyID), body)
}

func (b *bot) joinStrategyDelete(ctx context.Context, strategyID string) (map[string]interface{}, error) {
	return b.restRequest(ctx, http.MethodDelete, "/v2/groups/join_approval_strategy/"+encodePath(strategyID), nil)
}

func (b *bot) joinStrategyExecute(ctx context.Context, strategyID string) (map[string]interface{}, error) {
	return b.restRequest(ctx, http.MethodPost, "/v2/groups/join_approval_strategy/"+encodePath(strategyID)+"/execute", map[string]interface{}{})
}

func (b *bot) joinStrategyWhitelist(ctx context.Context, strategyID, op string, users []string) (map[string]interface{}, error) {
	return b.restRequest(ctx, http.MethodPost, "/v2/groups/join_approval_strategy/"+encodePath(strategyID)+"/whitelist_users", map[string]interface{}{
		"op":             op,
		"whitelist_users": users,
	})
}

// ---- 动作分发（插件经 adapter.action 调用，返回 JSON 字符串）----

func stringSlice(value interface{}) []string {
	items, _ := value.([]interface{})
	result := make([]string, 0, len(items))
	for _, item := range items {
		if text := strings.TrimSpace(stringValue(item)); text != "" {
			result = append(result, text)
		}
	}
	return result
}

// handleGroupManageAction 处理群管理动作；handled=false 表示动作类型不归本模块。
func (b *bot) handleGroupManageAction(ctx context.Context, options map[string]interface{}) (bool, string) {
	actionType := strings.ToLower(strings.TrimSpace(stringValue(options["type"])))
	if !strings.HasPrefix(actionType, "group_") && !strings.HasPrefix(actionType, "join_") {
		return false, ""
	}
	ctx, cancel := context.WithTimeout(ctx, manageActionTimeout)
	defer cancel()

	groupID := firstString(options, "group_id", "group_openid", "groupid")
	requestResult := func(data map[string]interface{}, err error) string {
		if err != nil {
			return manageResult(false, err.Error(), data)
		}
		return manageResult(true, "", data)
	}

	switch actionType {
	case "group_info":
		if groupID == "" {
			return true, manageResult(false, "缺少 group_id", nil)
		}
		return true, requestResult(b.groupInfo(ctx, groupID))
	case "group_bot_state":
		if groupID == "" {
			return true, manageResult(false, "缺少 group_id", nil)
		}
		return true, requestResult(b.groupBotState(ctx, groupID))
	case "group_member_list":
		if groupID == "" {
			return true, manageResult(false, "缺少 group_id", nil)
		}
		return true, requestResult(b.groupMembers(ctx, groupID, firstString(options, "cursor")))
	case "group_member_info":
		member := firstString(options, "member_openid", "member_id")
		if groupID == "" || member == "" {
			return true, manageResult(false, "缺少 group_id 或 member_openid", nil)
		}
		return true, requestResult(b.groupMemberInfo(ctx, groupID, member))
	case "group_member_remove":
		members := stringSlice(options["member_openids"])
		if groupID == "" || len(members) == 0 {
			return true, manageResult(false, "缺少 group_id 或 member_openids", nil)
		}
		if len(members) > 20 {
			return true, manageResult(false, "单次最多移除 20 名成员", nil)
		}
		addBlacklist := strings.EqualFold(stringValue(options["add_to_blacklist"]), "true") || options["add_to_blacklist"] == true
		return true, requestResult(b.groupRemoveMembers(ctx, groupID, members, addBlacklist))
	case "group_blacklist":
		if groupID == "" {
			return true, manageResult(false, "缺少 group_id", nil)
		}
		limit := intValue(options["limit"])
		return true, requestResult(b.groupBlacklist(ctx, groupID, firstString(options, "cursor"), limit))
	case "group_blacklist_update":
		members := stringSlice(options["member_openids"])
		op := strings.ToLower(firstString(options, "op"))
		if groupID == "" || len(members) == 0 || (op != "add" && op != "remove") {
			return true, manageResult(false, "缺少 group_id、member_openids 或 op（add/remove）", nil)
		}
		if len(members) > 20 {
			return true, manageResult(false, "单次最多操作 20 名成员", nil)
		}
		return true, requestResult(b.groupBlacklistUpdate(ctx, groupID, op, members))
	case "group_join_requests":
		if groupID == "" {
			return true, manageResult(false, "缺少 group_id", nil)
		}
		return true, requestResult(b.groupJoinRequests(ctx, groupID, firstString(options, "cursor"), intValue(options["limit"])))
	case "group_join_approve":
		member := firstString(options, "member_openid", "user_id")
		op := strings.ToLower(firstString(options, "op"))
		if groupID == "" || member == "" || (op != "approve" && op != "decline") {
			return true, manageResult(false, "缺少 group_id、member_openid 或 op（approve/decline）", nil)
		}
		data, err := b.groupApproveJoinRequest(ctx, groupID, member, op,
			firstString(options, "join_request_id"), firstString(options, "reject_reason"),
			options["add_to_blacklist"] == true || strings.EqualFold(stringValue(options["add_to_blacklist"]), "true"))
		if err == nil {
			coreLogInfo("qqguild入群申请已处理：group=%s member=%s op=%s", groupID, member, op)
		}
		return true, requestResult(data, err)
	case "group_mute":
		members, _ := options["members"].([]interface{})
		if groupID == "" || len(members) == 0 {
			return true, manageResult(false, "缺少 group_id 或 members", nil)
		}
		if len(members) > 10 {
			return true, manageResult(false, "单次最多设置 10 名成员禁言", nil)
		}
		return true, requestResult(b.groupSetMute(ctx, groupID, members))
	case "group_mute_setting":
		if groupID == "" {
			return true, manageResult(false, "缺少 group_id", nil)
		}
		return true, requestResult(b.groupMuteSetting(ctx, groupID))
	case "join_strategy_list":
		return true, requestResult(b.joinStrategyList(ctx, intValue(options["limit"])))
	case "join_strategy_create":
		groups := stringSlice(options["group_openids"])
		if len(groups) == 0 {
			return true, manageResult(false, "缺少 group_openids", nil)
		}
		enable := firstString(options, "is_enable")
		if enable == "" {
			enable = "on"
		}
		return true, requestResult(b.joinStrategyCreate(ctx, groups, enable, firstString(options, "remark")))
	case "join_strategy_update":
		strategyID := firstString(options, "strategy_id")
		if strategyID == "" {
			return true, manageResult(false, "缺少 strategy_id", nil)
		}
		body := map[string]interface{}{}
		if enable := firstString(options, "is_enable"); enable != "" {
			body["is_enable"] = enable
		}
		if remark := firstString(options, "remark"); remark != "" {
			body["remark"] = remark
		}
		return true, requestResult(b.joinStrategyUpdate(ctx, strategyID, body))
	case "join_strategy_delete":
		strategyID := firstString(options, "strategy_id")
		if strategyID == "" {
			return true, manageResult(false, "缺少 strategy_id", nil)
		}
		return true, requestResult(b.joinStrategyDelete(ctx, strategyID))
	case "join_strategy_execute":
		strategyID := firstString(options, "strategy_id")
		if strategyID == "" {
			return true, manageResult(false, "缺少 strategy_id", nil)
		}
		return true, requestResult(b.joinStrategyExecute(ctx, strategyID))
	case "join_strategy_whitelist":
		strategyID := firstString(options, "strategy_id")
		users := stringSlice(options["whitelist_users"])
		op := strings.ToLower(firstString(options, "op"))
		if strategyID == "" || len(users) == 0 || (op != "add" && op != "remove") {
			return true, manageResult(false, "缺少 strategy_id、whitelist_users 或 op（add/remove）", nil)
		}
		return true, requestResult(b.joinStrategyWhitelist(ctx, strategyID, op, users))
	default:
		return false, ""
	}
}

func manageResult(ok bool, errMsg string, data map[string]interface{}) string {
	result := map[string]interface{}{"ok": ok}
	if errMsg != "" {
		result["error"] = errMsg
	}
	if data != nil {
		result["data"] = data
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		return `{"ok":false,"error":"结果序列化失败"}`
	}
	return string(encoded)
}

func coreLogInfo(format string, args ...interface{}) {
	core.Logs.Info(fmt.Sprintf(format, args...))
}

func coreLogWarn(format string, args ...interface{}) {
	core.Logs.Warn(fmt.Sprintf(format, args...))
}

// ---- 入群申请人工审核（1 通过 / 0 拒绝）----

type joinReviewItem struct {
	groupOpenid   string
	memberOpenid  string
	joinRequestID string
	username      string
	expiresAt     int64
}

var joinReviewQueues sync.Map // group_openid → *[]joinReviewItem（FIFO）

const (
	joinReviewTTL      = 2 * time.Hour // 待审申请有效期，超时自动作废
	joinReviewMaxQueue = 10            // 每群最多排队条数
)

// enqueueJoinReview 将申请加入群待审队列，返回当前待审条数（已清理过期项）。
func enqueueJoinReview(item joinReviewItem) int {
	value, _ := joinReviewQueues.LoadOrStore(item.groupOpenid, &[]joinReviewItem{})
	queue := value.(*[]joinReviewItem)
	now := time.Now().UnixMilli()
	kept := (*queue)[:0]
	for _, current := range *queue {
		if now < current.expiresAt {
			kept = append(kept, current)
		}
	}
	kept = append(kept, item)
	if len(kept) > joinReviewMaxQueue {
		kept = kept[len(kept)-joinReviewMaxQueue:]
	}
	*queue = kept
	return len(kept)
}

// popJoinReview 弹出最早一条待审申请。
func popJoinReview(groupOpenid string) (joinReviewItem, bool) {
	value, ok := joinReviewQueues.Load(groupOpenid)
	if !ok {
		return joinReviewItem{}, false
	}
	queue := value.(*[]joinReviewItem)
	if len(*queue) == 0 {
		joinReviewQueues.Delete(groupOpenid)
		return joinReviewItem{}, false
	}
	item := (*queue)[0]
	*queue = (*queue)[1:]
	if len(*queue) == 0 {
		joinReviewQueues.Delete(groupOpenid)
	}
	return item, true
}

// sendGroupNotice 主动发送群文本（无 msg_id，占主动消息额度，仅用于申请到达提示）。
func (b *bot) sendGroupNotice(ctx context.Context, groupOpenid, content string) {
	requestCtx, cancel := context.WithTimeout(ctx, manageActionTimeout)
	defer cancel()
	if _, err := b.api.PostGroupMessage(requestCtx, groupOpenid, &dto.MessageToCreate{
		Content: content,
		MsgType: dto.TextMsg,
	}); err != nil {
		coreLogWarn("qqguild发送入群申请提示失败：group=%s %v", groupOpenid, err)
	}
}

// replyGroupNotice 以被动回复方式发送群文本（带 msg_id 不占主动消息额度）。
func (b *bot) replyGroupNotice(ctx context.Context, groupOpenid, content, msgID string) {
	requestCtx, cancel := context.WithTimeout(ctx, manageActionTimeout)
	defer cancel()
	if _, err := b.api.PostGroupMessage(requestCtx, groupOpenid, &dto.MessageToCreate{
		Content: content,
		MsgID:   msgID,
		MsgSeq:  b.nextReplySeq(msgID),
		MsgType: dto.TextMsg,
	}); err != nil {
		coreLogWarn("qqguild发送审核结果失败：group=%s %v", groupOpenid, err)
	}
}

// handleGroupJoinRequest 处理用户申请加群事件（GROUP_JOIN_REQUEST，intents 1<<25 群域）。
// 配置 qqguild.group_join_auto_approve = approve / decline 时自动审批；
// 默认（off）将申请发到群里，管理员回复 1 通过 / 0 拒绝。
func handleGroupJoinRequest(message []byte) error {
	b := currentBot()
	data := groupEventData(message)
	groupID := firstString(data, "group_openid", "group_id")
	member := firstString(data, "member_openid", "user_openid", "user_id")
	requestID := firstString(data, "join_request_id")
	username := firstString(data, "username")
	if username == "" {
		username = member
	}
	mode := strings.ToLower(strings.TrimSpace(settings.GetString("group_join_auto_approve")))
	if mode != "approve" && mode != "decline" {
		mode = "off"
	}
	coreLogInfo("qqguild收到入群申请：group=%s member=%s name=%s mode=%s", b.groupDisplayName(groupID), member, username, mode)
	if b == nil || groupID == "" || member == "" {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), manageActionTimeout)
	defer cancel()
	if mode == "approve" || mode == "decline" {
		if _, err := b.groupApproveJoinRequest(ctx, groupID, member, mode, requestID, "", false); err != nil {
			coreLogWarn("qqguild入群申请自动审批失败：group=%s member=%s %v", groupID, member, err)
			return nil
		}
		coreLogInfo("qqguild入群申请已自动%s：group=%s member=%s", map[string]string{"approve": "通过", "decline": "拒绝"}[mode], groupID, member)
		return nil
	}

	// 人工模式：入队并在群里提示，管理员回复 1 通过 / 0 拒绝。
	item := joinReviewItem{
		groupOpenid:   groupID,
		memberOpenid:  member,
		joinRequestID: requestID,
		username:      username,
		expiresAt:     time.Now().Add(joinReviewTTL).UnixMilli(),
	}
	pending := enqueueJoinReview(item)
	b.sendGroupNotice(ctx, groupID, fmt.Sprintf("收到「%s」的入群申请，回复 1 通过 / 0 拒绝（当前待审 %d 条）", username, pending))
	return nil
}

// handleJoinReviewReply 消费群里回复的 1/0 审核指令；返回 true 表示消息已处理，
// 不再进入核心消息流。仅当该群有待审申请时才消费。
func (b *bot) handleJoinReviewReply(groupOpenid, content, msgID string) bool {
	if content != "1" && content != "0" {
		return false
	}
	if _, ok := joinReviewQueues.Load(groupOpenid); !ok {
		return false
	}
	item, ok := popJoinReview(groupOpenid)
	if !ok {
		return false
	}
	op := "approve"
	actionText := "已通过"
	if content == "0" {
		op = "decline"
		actionText = "已拒绝"
	}
	name := item.username
	if name == "" {
		name = item.memberOpenid
	}
	requestCtx, cancel := context.WithTimeout(context.Background(), manageActionTimeout)
	defer cancel()
	if _, err := b.groupApproveJoinRequest(requestCtx, groupOpenid, item.memberOpenid, op, item.joinRequestID, "", false); err != nil {
		coreLogWarn("qqguild入群审核失败：group=%s member=%s op=%s %v", groupOpenid, item.memberOpenid, op, err)
		b.replyGroupNotice(requestCtx, groupOpenid, fmt.Sprintf("入群审核失败：%v", err), msgID)
		return true
	}
	coreLogInfo("qqguild入群审核完成：group=%s member=%s op=%s name=%s", groupOpenid, item.memberOpenid, op, name)
	resultText := fmt.Sprintf("%s「%s」的入群申请。", actionText, name)
	// 队列里可能还有未过期的剩余申请
	if value, ok := joinReviewQueues.Load(groupOpenid); ok {
		if queue, ok := value.(*[]joinReviewItem); ok && len(*queue) > 0 {
			resultText += fmt.Sprintf("待审还剩 %d 条，回复 1 通过 / 0 拒绝。", len(*queue))
		}
	}
	b.replyGroupNotice(requestCtx, groupOpenid, resultText, msgID)
	return true
}

// ---- 平台托管入群自动审批策略自动同步 ----

// 后台 qqguild 设置表单填 join_strategy_groups（群 openid，逗号分隔）与
// join_strategy_whitelist（QQ 号，逗号分隔）后，适配器自动在官方侧创建/更新
// 一个标识 remark 为 joinStrategyRemark 的托管策略；机器人离线时官方照常审批。

const joinStrategyRemark = "sillygirl-auto"

var joinStrategySync = restartDebouncer{delay: 2 * time.Second, action: syncJoinStrategy}

func splitCSV(value string) []string {
	result := []string{}
	for _, part := range strings.Split(value, ",") {
		if text := strings.TrimSpace(part); text != "" {
			result = append(result, text)
		}
	}
	return result
}

func splitLines(value string) []string {
	result := []string{}
	for _, part := range strings.Split(value, "\n") {
		if text := strings.TrimSpace(part); text != "" && text != "," {
			result = append(result, text)
		}
	}
	return result
}

// syncJoinStrategy 将表单配置同步到官方托管策略（重建式，无本地快照）：
// 每次配置变更删除自有策略后重建，白名单全量 add。
// 官方不提供白名单读取接口，重建是彻底单一数据源（表单 = 官方现状）的代价：
// 重建间隙自动审批有几秒空窗，仅在保存表单时发生。
// 群列表清空则删除自有策略，交回人工 1/0 或本地 approve 模式。
// 注意：适配器启动时不触发同步——官方策略持久存在，与机器人生命周期无关。
func syncJoinStrategy() {
	b := currentBot()
	if b == nil {
		return
	}
	groups := splitCSV(settings.GetString("join_strategy_groups"))
	whitelist := splitLines(settings.GetString("join_strategy_whitelist"))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	list, err := b.joinStrategyList(ctx, 20)
	if err != nil {
		coreLogWarn("qqguild同步入群审批策略失败（拉取策略）：%v", err)
		return
	}
	strategyID := ""
	if entries, ok := list["strategies"].([]interface{}); ok {
		for _, entry := range entries {
			current, _ := entry.(map[string]interface{})
			if current != nil && stringValue(current["remark"]) == joinStrategyRemark {
				strategyID = stringValue(current["strategy_id"])
				break
			}
		}
	}

	if len(groups) == 0 {
		if strategyID != "" {
			if _, err := b.joinStrategyDelete(ctx, strategyID); err != nil {
				coreLogWarn("qqguild删除入群审批策略失败：%v", err)
				return
			}
			coreLogInfo("qqguild已删除入群审批策略（策略群列表已清空）：%s", strategyID)
		}
		return
	}

	if strategyID != "" {
		if _, err := b.joinStrategyDelete(ctx, strategyID); err != nil {
			coreLogWarn("qqguild删除旧入群审批策略失败：%v", err)
			return
		}
	}
	created, err := b.joinStrategyCreate(ctx, groups, "on", joinStrategyRemark)
	if err != nil {
		coreLogWarn("qqguild创建入群审批策略失败：%v", err)
		return
	}
	strategyID = stringValue(created["strategy_id"])
	if strategyID == "" {
		if data, ok := created["data"].(map[string]interface{}); ok {
			strategyID = stringValue(data["strategy_id"])
		}
	}
	if strategyID == "" {
		coreLogWarn("qqguild创建入群审批策略成功但未返回 strategy_id")
		return
	}
	if len(whitelist) > 0 {
		if _, err := b.joinStrategyWhitelist(ctx, strategyID, "add", whitelist); err != nil {
			coreLogWarn("qqguild同步入群审批白名单失败：%v", err)
			return
		}
	}
	coreLogInfo("qqguild入群审批策略已同步：strategy=%s groups=%d whitelist=%d", strategyID, len(groups), len(whitelist))
}

// ---- 事件处理（经 handlePlainEvent 分发）----

// groupEventData 解析 botgo Plain 事件的 envelope（{op,s,t,id,d}），返回其中的 d
// 字段；对直接传事件数据的情形（无 d 字段）兼容回退到顶层。
func groupEventData(message []byte) map[string]interface{} {
	data := map[string]interface{}{}
	if err := json.Unmarshal(message, &data); err != nil {
		return map[string]interface{}{}
	}
	if inner, ok := data["d"].(map[string]interface{}); ok && len(inner) > 0 {
		return inner
	}
	return data
}

// ---- 群名缓存：事件 payload 只带 group_openid，群名通过 info 接口异步回填 ----

type groupNameEntry struct {
	name      string
	expiresAt int64
}

var groupNameCache sync.Map // group_openid → groupNameEntry

const (
	groupNameCacheTTL     = 24 * time.Hour  // 成功缓存 24 小时
	groupNameFailCacheTTL = 10 * time.Minute // 拉取失败（含 11253 内邀未开通）负缓存，避免每次事件都打接口
)

// groupDisplayName 返回「群名(openid)」；群名未缓存时异步回填，本次直接显示 openid。
func (b *bot) groupDisplayName(groupOpenid string) string {
	if groupOpenid == "" || b == nil {
		return groupOpenid
	}
	if value, ok := groupNameCache.Load(groupOpenid); ok {
		if entry, ok := value.(groupNameEntry); ok {
			if time.Now().UnixMilli() < entry.expiresAt {
				if entry.name != "" {
					return entry.name + "(" + groupOpenid + ")"
				}
				return groupOpenid // 负缓存：接口不可用，直接显示 openid
			}
			groupNameCache.Delete(groupOpenid)
		}
	}
	// 未命中：异步拉取回填，不阻塞事件处理。
	go func(openid string) {
		ctx, cancel := context.WithTimeout(context.Background(), manageActionTimeout)
		defer cancel()
		name := ""
		ttl := groupNameCacheTTL
		if data, err := b.groupInfo(ctx, openid); err == nil {
			name = firstString(data, "group_name")
		} else {
			ttl = groupNameFailCacheTTL
		}
		groupNameCache.Store(openid, groupNameEntry{name: name, expiresAt: time.Now().Add(ttl).UnixMilli()})
	}(groupOpenid)
	return groupOpenid
}


// handleGroupMemberEvent 处理群成员进退事件（GROUP_MEMBER_ADD / GROUP_MEMBER_REMOVE，intents 1<<24）。
func handleGroupMemberEvent(eventType string, message []byte) error {
	b := currentBot()
	data := groupEventData(message)
	groupID := firstString(data, "group_openid", "group_id")
	member := firstString(data, "member_openid", "user_openid", "user_id")
	coreLogInfo("qqguild群成员%s：group=%s member=%s",
		map[string]string{"GROUP_MEMBER_ADD": "加入", "GROUP_MEMBER_REMOVE": "退出"}[eventType], b.groupDisplayName(groupID), member)
	return nil
}

// handleGroupManageNoticeEvent 记录机器人进群/退群与主动消息开关事件。
func handleGroupManageNoticeEvent(eventType string, message []byte) error {
	b := currentBot()
	data := groupEventData(message)
	coreLogInfo("qqguild群管理事件 %s：group=%s operator=%s", eventType,
		b.groupDisplayName(firstString(data, "group_openid", "group_id")), firstString(data, "op_member_openid"))
	return nil
}
