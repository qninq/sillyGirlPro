package pagermaid

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/qninq/sillyGirlPro/core"
	"github.com/qninq/sillyGirlPro/core/logs"
	"github.com/qninq/sillyGirlPro/core/storage"
	"github.com/qninq/sillyGirlPro/utils"
)

const platform = "pagermaid"

var pagermaid = core.MakeBucket(platform)

type callAPI struct {
	Action string      `json:"action"`
	Data   interface{} `json:"data,omitempty"`
	Echo   string      `json:"echo,omitempty"`
}

type replySegment struct {
	kind  string
	value string
}

const (
	replySegmentText  = "text"
	replySegmentImage = "image"
)

type pagermaidConn struct {
	conn *websocket.Conn
	sync.RWMutex
	id    int
	chans map[string]chan string
}

var (
	debug          = pagermaid.GetBool("debug", false)
	connections    sync.Map
	newlinePattern = regexp.MustCompile(`[\n\r]+`)
	cqImagePattern = regexp.MustCompile(`\[CQ:image,([^\]]+)\]`)
)

func init() {
	storage.Watch(pagermaid, "enable", func(old, new, key string) *storage.Final {
		if !enabledValue(new) {
			closeConnections()
			core.DestroyAdaptersByPlatform(platform)
		}
		return nil
	})
	storage.Watch(pagermaid, "token", func(old, new, key string) *storage.Final {
		if strings.TrimSpace(old) != strings.TrimSpace(new) {
			closeConnections()
			core.DestroyAdaptersByPlatform(platform)
		}
		return nil
	})
	storage.Watch(pagermaid, "debug", func(old, new, key string) *storage.Final {
		debug = truthyValue(new)
		return nil
	})

	go func() {
		core.GinApi(core.GET, "/pagermaid/receive", func(ctx *gin.Context) {
			serveWebSocket(ctx)
		})
	}()
}

func serveWebSocket(ctx *gin.Context) {
	if !core.AdapterConfigEnabled(platform) {
		core.Logs.Warn("Pagermaid机器人未启动：pagermaid.enable=false")
		ctx.AbortWithStatus(http.StatusForbidden)
		return
	}
	if !validAuthorization(ctx.GetHeader("Authorization"), ctx.Query("token"), pagermaid.GetString("token"), ctx.Request.RemoteAddr) {
		core.Logs.Warn("Pagermaid机器人token不正确")
		ctx.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	if strings.TrimSpace(pagermaid.GetString("token")) == "" {
		core.Logs.Warn("建议在BOT页面配置 pagermaid.token，并在 Pagermaid WebSocket 地址中携带 token")
	}

	upgrader := websocket.Upgrader{CheckOrigin: validOrigin}
	ws, err := upgrader.Upgrade(ctx.Writer, ctx.Request, nil)
	if err != nil {
		_, _ = ctx.Writer.Write([]byte(err.Error()))
		return
	}
	botID := firstNonEmpty(ctx.Query("user_id"), ctx.GetHeader("X-Self-ID"), ctx.ClientIP())
	if botID == "" {
		botID = "default"
	}
	conn := &pagermaidConn{
		conn:  ws,
		chans: make(map[string]chan string),
	}
	if old, ok := connections.Load(botID); ok {
		if oldConn, ok := old.(*pagermaidConn); ok && oldConn.conn != nil {
			_ = oldConn.conn.Close()
		}
	}
	connections.Store(botID, conn)
	defer func() {
		if current, ok := connections.Load(botID); ok && current == conn {
			connections.Delete(botID)
		}
	}()

	adapter := &core.Factory{}
	adapter.Init(platform, botID, nil)
	defer adapter.Destroy()
	adapter.SetReplyHandler(func(msg map[string]interface{}) string {
		return conn.reply(msg)
	})

	_ = conn.writeJSON(callAPI{
		Action: "set_whitelist",
		Data:   adapter.Masters(),
	})

	for {
		_, data, err := ws.ReadMessage()
		if err != nil {
			_ = ws.Close()
			break
		}
		if debug {
			logs.Debug("Pagermaid接收消息：", string(data))
		}
		conn.handle(adapter, botID, data)
	}
}

func (pc *pagermaidConn) reply(msg map[string]interface{}) string {
	chatID := stringValue(msg[core.CHAT_ID])
	if chatID == "" {
		chatID = stringValue(msg[core.USER_ID])
	}
	content := stringValue(msg[core.CONETNT])
	if chatID == "" || strings.TrimSpace(content) == "" {
		return ""
	}
	replyTo := stringValue(msg[core.MESSAGE_ID])
	segments := splitReplySegments(content)
	lastID := ""
	for _, segment := range segments {
		data := map[string]interface{}{
			"chat_id": chatID,
		}
		if replyTo != "" {
			data["reply_to_message_id"] = replyTo
		}
		action := "send_message"
		if segment.kind == replySegmentImage {
			action = "send_photo"
			data["photo"] = segment.value
		} else {
			data["text"] = segment.value
		}
		id, err := pc.call(action, data)
		if err != nil {
			core.Logs.Warn("Pagermaid发送消息失败：%v", err)
			continue
		}
		if id != "" {
			lastID = id
		}
	}
	return lastID
}

func (pc *pagermaidConn) call(action string, data map[string]interface{}) (string, error) {
	ch := make(chan string, 1)
	echo := ""
	pc.Lock()
	pc.id++
	echo = fmt.Sprint(pc.id)
	pc.chans[echo] = ch
	err := pc.conn.WriteJSON(callAPI{
		Action: action,
		Data:   data,
		Echo:   echo,
	})
	pc.Unlock()
	if err != nil {
		pc.deleteChan(echo)
		return "", err
	}
	defer pc.deleteChan(echo)

	select {
	case id := <-ch:
		return id, nil
	case <-time.After(20 * time.Second):
		return "", nil
	}
}

func (pc *pagermaidConn) handle(adapter *core.Factory, botID string, data []byte) {
	payload, err := decodeObject(data)
	if err != nil {
		return
	}
	if echo := stringValue(payload["echo"]); echo != "" {
		pc.complete(echo, messageID(payload))
		return
	}
	if boolValue(payload["outgoing"]) {
		return
	}

	user := objectValue(payload["from_user"])
	if len(user) == 0 {
		user = objectValue(payload["sender_chat"])
	}
	if len(user) == 0 {
		user = objectValue(payload["from"])
	}
	chat := objectValue(payload["chat"])
	if len(chat) == 0 {
		chat = objectValue(payload["peer"])
	}
	userID := stringValue(user["id"])
	chatID := stringValue(chat["id"])
	if userID == "" {
		userID = chatID
	}
	if userID == "" {
		return
	}
	if botID != "" && userID == botID {
		return
	}

	chatType := strings.ToLower(firstNonEmpty(stringValue(chat["type"]), stringValue(payload["chat_type"])))
	if isPrivateChat(chatType) || chatID == userID {
		chatID = ""
	}

	content := normalizeContent(firstNonEmpty(
		stringValue(payload["text"]),
		stringValue(payload["caption"]),
		stringValue(payload["raw_text"]),
		stringValue(payload["message"]),
	))
	if content == "" {
		return
	}

	if chatID != "" {
		core.CreateNickName(&core.Nickname{
			Group:    true,
			Value:    chatName(chat),
			ID:       chatID,
			Platform: platform,
			BotsID:   []string{botID},
		})
	}
	core.CreateNickName(&core.Nickname{
		Value:    userName(user),
		ID:       userID,
		Platform: platform,
		BotsID:   []string{botID},
	})

	msg := map[string]interface{}{
		core.USER_ID:    userID,
		core.CHAT_ID:    core.ChatID(chatID),
		core.MESSAGE_ID: messageID(payload),
		core.CONETNT:    content,
		"user_name":     userName(user),
		"chat_name":     chatName(chat),
	}
	if debug {
		logs.Debug("Pagermaid处理消息：", string(utils.JsonMarshal(msg)))
	}
	adapter.Receive(msg)
}

func (pc *pagermaidConn) complete(echo string, id string) {
	pc.RLock()
	ch, ok := pc.chans[echo]
	pc.RUnlock()
	if !ok {
		return
	}
	select {
	case ch <- id:
	default:
	}
}

func (pc *pagermaidConn) deleteChan(echo string) {
	pc.Lock()
	delete(pc.chans, echo)
	pc.Unlock()
}

func (pc *pagermaidConn) writeJSON(value interface{}) error {
	pc.Lock()
	defer pc.Unlock()
	return pc.conn.WriteJSON(value)
}

func closeConnections() {
	connections.Range(func(_, value interface{}) bool {
		if conn, ok := value.(*pagermaidConn); ok && conn.conn != nil {
			_ = conn.conn.Close()
		}
		return true
	})
}

func validAuthorization(auth string, queryToken string, token string, remoteAddr string) bool {
	token = strings.TrimSpace(token)
	if token == "" {
		return isLoopbackRemote(remoteAddr)
	}
	for _, value := range []string{queryToken, auth} {
		value = strings.TrimSpace(value)
		if value == token {
			return true
		}
		const bearerPrefix = "Bearer "
		if len(value) > len(bearerPrefix) && strings.EqualFold(value[:len(bearerPrefix)], bearerPrefix) {
			if strings.TrimSpace(value[len(bearerPrefix):]) == token {
				return true
			}
		}
	}
	return false
}

func isLoopbackRemote(remoteAddr string) bool {
	remoteAddr = strings.TrimSpace(remoteAddr)
	if remoteAddr == "" {
		return false
	}
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	return ip != nil && ip.IsLoopback()
}

func validOrigin(r *http.Request) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return true
	}
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Host == "" {
		return false
	}
	if strings.EqualFold(parsed.Host, r.Host) {
		return true
	}
	return strings.HasPrefix(origin, "http://127.0.0.1:") ||
		strings.HasPrefix(origin, "http://localhost:") ||
		strings.HasPrefix(origin, "http://[::1]:")
}

func decodeObject(data []byte) (map[string]interface{}, error) {
	payload := map[string]interface{}{}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func objectValue(value interface{}) map[string]interface{} {
	if value == nil {
		return nil
	}
	if result, ok := value.(map[string]interface{}); ok {
		return result
	}
	return nil
}

func stringValue(value interface{}) string {
	return strings.TrimSpace(utils.Itoa(value))
}

func boolValue(value interface{}) bool {
	switch v := value.(type) {
	case bool:
		return v
	case string:
		return truthyValue(v)
	default:
		return false
	}
}

func truthyValue(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true", "1", "on", "yes":
		return true
	default:
		return false
	}
}

func enabledValue(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "false", "0", "off", "no":
		return false
	default:
		return true
	}
}

func isPrivateChat(chatType string) bool {
	chatType = strings.ToLower(strings.TrimSpace(chatType))
	return strings.Contains(chatType, "private") || strings.Contains(chatType, "bot")
}

func messageID(payload map[string]interface{}) string {
	return firstNonEmpty(stringValue(payload["message_id"]), stringValue(payload["id"]))
}

func normalizeContent(content string) string {
	content = strings.ReplaceAll(content, "\\r", "\n")
	content = newlinePattern.ReplaceAllString(content, "\n")
	return strings.TrimSpace(content)
}

func userName(user map[string]interface{}) string {
	return firstNonEmpty(
		stringValue(user["username"]),
		strings.TrimSpace(strings.Join([]string{stringValue(user["first_name"]), stringValue(user["last_name"])}, " ")),
		stringValue(user["id"]),
	)
}

func chatName(chat map[string]interface{}) string {
	return firstNonEmpty(
		stringValue(chat["title"]),
		stringValue(chat["username"]),
		strings.TrimSpace(strings.Join([]string{stringValue(chat["first_name"]), stringValue(chat["last_name"])}, " ")),
		stringValue(chat["id"]),
	)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func splitReplySegments(text string) []replySegment {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	segments := make([]replySegment, 0, 2)
	last := 0
	matches := cqImagePattern.FindAllStringSubmatchIndex(text, -1)
	for _, match := range matches {
		if match[0] > last {
			appendReplyTextSegment(&segments, text[last:match[0]])
		}
		attrs := parseCQParams(text[match[2]:match[3]])
		image := firstNonEmpty(attrs["file"], attrs["url"])
		if image != "" {
			segments = append(segments, replySegment{kind: replySegmentImage, value: image})
		}
		last = match[1]
	}
	if last < len(text) {
		appendReplyTextSegment(&segments, text[last:])
	}
	return segments
}

func appendReplyTextSegment(segments *[]replySegment, text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	*segments = append(*segments, replySegment{kind: replySegmentText, value: text})
}

func parseCQParams(raw string) map[string]string {
	params := map[string]string{}
	for _, item := range strings.Split(raw, ",") {
		key, value, ok := strings.Cut(item, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		params[key] = decodeCQValue(value)
	}
	return params
}

func decodeCQValue(value string) string {
	replacer := strings.NewReplacer(
		"&#44;", ",",
		"&#91;", "[",
		"&#93;", "]",
		"&amp;", "&",
	)
	return replacer.Replace(strings.TrimSpace(value))
}
