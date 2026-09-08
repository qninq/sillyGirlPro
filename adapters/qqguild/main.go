package qqguild

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/smallfawn/sillyGirl/core"
	"github.com/smallfawn/sillyGirl/core/storage"
	"github.com/smallfawn/sillyGirl/utils"
	"github.com/tencent-connect/botgo"
	"github.com/tencent-connect/botgo/dto"
	qqmessage "github.com/tencent-connect/botgo/dto/message"
	"github.com/tencent-connect/botgo/event"
	"github.com/tencent-connect/botgo/interaction/signature"
	"github.com/tencent-connect/botgo/interaction/webhook"
	"github.com/tencent-connect/botgo/openapi/options"
	"github.com/tencent-connect/botgo/sessions/manager"
	"github.com/tencent-connect/botgo/token"
	botwebsocket "github.com/tencent-connect/botgo/websocket"
	"golang.org/x/oauth2"
)

const (
	platform          = "qqguild"
	webhookPath       = "/qqguild/webhook"
	maxWebhookBody    = 1 << 20
	directSessionTTL  = 24 * time.Hour
	replySequenceTTL  = 5 * time.Minute
	cacheCleanupEvery = time.Minute
	tokenRetryInitial = 3 * time.Second
	tokenRetryMax     = time.Minute
	modeWebhook       = "webhook"
	modeWebsocket     = "websocket"
	wsBackoffMin      = 2 * time.Second
	wsBackoffMax      = 30 * time.Second
	msgContextTTL     = 30 * time.Minute
	seenMessageTTL    = 10 * time.Second
	chatSceneLimit    = 2048
	qqAPIHostDefault  = "https://api.sgroup.qq.com"
	qqSandboxAPIHost  = "https://sandbox.api.sgroup.qq.com"
	sceneChannel      = "channel"
	sceneDirect       = "direct"
	sceneGroup        = "group"
	sceneC2C          = "c2c"
)

var settings = core.MakeBucket(platform)
var websocketIntents dto.Intent
var configRestart = restartDebouncer{delay: 150 * time.Millisecond, action: restart}
var botGoSensitivePatterns = []struct {
	pattern     *regexp.Regexp
	replacement string
}{
	{regexp.MustCompile(`(?i)("clientSecret"\s*:\s*")[^"]*(")`), `${1}<redacted>${2}`},
	{regexp.MustCompile(`(?i)("access_token"\s*:\s*")[^"]*(")`), `${1}<redacted>${2}`},
	{regexp.MustCompile(`(?i)(QQBot\s+)[A-Za-z0-9._~\-]+`), `${1}<redacted>`},
}

type restartDebouncer struct {
	sync.Mutex
	timer  *time.Timer
	delay  time.Duration
	action func()
}

type botGoLogger struct{}

func (botGoLogger) Debug(v ...interface{}) {
	if settings.GetBool("debug", false) {
		core.Logs.Debug("BotGo: %s", sanitizeBotGoLog(fmt.Sprint(v...)))
	}
}

func (botGoLogger) Info(v ...interface{}) {
	msg := sanitizeBotGoLog(fmt.Sprint(v...))
	if isBotGoHeartbeat(msg) {
		return
	}
	core.Logs.Info("BotGo: %s", msg)
}
func (botGoLogger) Warn(v ...interface{}) {
	core.Logs.Warn("BotGo: %s", sanitizeBotGoLog(fmt.Sprint(v...)))
}
func (botGoLogger) Error(v ...interface{}) {
	core.Logs.Error("BotGo: %s", sanitizeBotGoLog(fmt.Sprint(v...)))
}

func (botGoLogger) Debugf(format string, v ...interface{}) {
	if settings.GetBool("debug", false) {
		core.Logs.Debug("BotGo: %s", sanitizeBotGoLog(fmt.Sprintf(format, v...)))
	}
}

func (botGoLogger) Infof(format string, v ...interface{}) {
	msg := sanitizeBotGoLog(fmt.Sprintf(format, v...))
	if isBotGoHeartbeat(msg) {
		return
	}
	core.Logs.Info("BotGo: %s", msg)
}

func (botGoLogger) Warnf(format string, v ...interface{}) {
	core.Logs.Warn("BotGo: %s", sanitizeBotGoLog(fmt.Sprintf(format, v...)))
}

func (botGoLogger) Errorf(format string, v ...interface{}) {
	core.Logs.Error("BotGo: %s", sanitizeBotGoLog(fmt.Sprintf(format, v...)))
}

func (botGoLogger) Sync() error { return nil }

func sanitizeBotGoLog(value string) string {
	if secret := strings.TrimSpace(settings.GetString("app_secret")); secret != "" {
		value = strings.ReplaceAll(value, secret, "<redacted>")
	}
	for _, item := range botGoSensitivePatterns {
		value = item.pattern.ReplaceAllString(value, item.replacement)
	}
	return value
}

func isBotGoHeartbeat(msg string) bool {
	return strings.Contains(msg, "Heartbeat")
}

func (d *restartDebouncer) schedule() {
	d.Lock()
	defer d.Unlock()
	if d.timer != nil {
		d.timer.Stop()
	}
	d.timer = time.AfterFunc(d.delay, func() {
		d.Lock()
		d.timer = nil
		action := d.action
		d.Unlock()
		if action != nil {
			action()
		}
	})
}

type messageAPI interface {
	PostMessage(context.Context, string, *dto.MessageToCreate, ...options.Option) (*dto.Message, error)
	PostDirectMessage(context.Context, *dto.DirectMessage, *dto.MessageToCreate, ...options.Option) (*dto.Message, error)
	PostGroupMessage(context.Context, string, dto.APIMessage, ...options.Option) (*dto.Message, error)
	PostC2CMessage(context.Context, string, dto.APIMessage, ...options.Option) (*dto.Message, error)
	RetractMessage(context.Context, string, string, ...options.Option) error
	RetractDMMessage(context.Context, string, string, ...options.Option) error
	RetractGroupMessage(context.Context, string, string, ...options.Option) error
	RetractC2CMessage(context.Context, string, string, ...options.Option) error
	Transport(context.Context, string, string, interface{}) ([]byte, error)
}

type bot struct {
	appID          string
	debug          bool
	sandbox        bool
	api            messageAPI
	adapter        *core.Factory
	direct         sync.Map
	replySeq       sync.Map
	msgCtx         sync.Map
	seen           sync.Map
	chatScenes     sync.Map
	chatSceneCount atomic.Int64
}

// msgContext remembers where an inbound message came from so later recall
// requests carrying only a message ID can pick the right retract endpoint.
type msgContext struct {
	scene     string
	target    string
	expiresAt int64
}

type directSession struct {
	guildID   string
	expiresAt int64
}

type replyCounter struct {
	sequence  atomic.Uint32
	expiresAt int64
}

var runtime = struct {
	sync.RWMutex
	generation  uint64
	cancel      context.CancelFunc
	credentials *token.QQBotCredentials
	bot         *bot
	mode        string
}{}

func init() {
	botgo.SetLogger(botGoLogger{})
	core.GinApi(core.POST, webhookPath, receiveWebhook)
	initOnboard()
	for _, key := range []string{"enable", "app_id", "app_secret", "mode", "sandbox", "public_bot", "markdown", "at", "debug"} {
		key := key
		storage.Watch(settings, key, func(old, new, key string) *storage.Final {
			return &storage.Final{EndFunc: configRestart.schedule}
		})
	}
	go func() {
		time.Sleep(2 * time.Second)
		restart()
	}()
}

func buildQQGuildHandlers() dto.Intent {
	handlers := []interface{}{
		event.ATMessageEventHandler(handleATMessage),
		event.DirectMessageEventHandler(handleDirectMessage),
		event.GroupATMessageEventHandler(handleGroupATMessage),
		event.C2CMessageEventHandler(handleC2CMessage),
		// BotGo SDK v0.2.1 does not have a built-in handler for
		// GROUP_MESSAGE_CREATE. Use PlainEventHandler as a fallback.
		event.PlainEventHandler(handlePlainEvent),
	}
	if settings.GetBool("public_bot", false) {
		handlers = append(handlers, event.MessageEventHandler(handleGuildMessage))
	}
	return event.RegisterHandlers(handlers...)
}

func restart() {
	runtime.Lock()
	runtime.bot = nil
	if runtime.cancel != nil {
		runtime.cancel()
		runtime.cancel = nil
	}
	runtime.generation++
	generation := runtime.generation
	runtime.credentials = nil
	runtime.mode = ""
	appID := strings.TrimSpace(settings.GetString("app_id"))
	appSecret := strings.TrimSpace(settings.GetString("app_secret"))
	mode := normalizeConnectionMode(settings.GetString("mode"))
	if !core.AdapterConfigEnabled(platform) {
		runtime.Unlock()
		core.Logs.Info("qqguild机器人未启动：qqguild.enable=false")
		return
	}
	if appID == "" || appSecret == "" {
		runtime.Unlock()
		core.Logs.Info("qqguild机器人未启动：未配置 qqguild.app_id/app_secret")
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	runtime.cancel = cancel
	runtime.credentials = &token.QQBotCredentials{AppID: appID, AppSecret: appSecret}
	runtime.mode = mode
	websocketIntents = buildQQGuildHandlers()
	runtime.Unlock()

	go run(ctx, generation, appID, appSecret, mode)
}

func run(ctx context.Context, generation uint64, appID, appSecret, mode string) {
	credentials := &token.QQBotCredentials{AppID: appID, AppSecret: appSecret}
	tokenSource := token.NewQQBotTokenSource(credentials)
	// OpenAPI obtains a fresh token from tokenSource before every request. An
	// explicit first fetch validates credentials without starting BotGo's
	// background refresher, whose repeated-failure path panics in v0.2.1.
	if err := retryToken(ctx, tokenRetryInitial, tokenRetryMax, func() error {
		_, err := tokenSource.Token()
		return err
	}, func(err error) {
		core.Logs.Warn("qqguild获取 access token 失败，稍后重试：%v", err)
	}); err != nil {
		return
	}
	if ctx.Err() != nil {
		return
	}

	api := botgo.NewOpenAPI(appID, tokenSource).WithTimeout(10 * time.Second).SetDebug(settings.GetBool("debug", false))
	if settings.GetBool("sandbox", false) {
		api = botgo.NewSandboxOpenAPI(appID, tokenSource).WithTimeout(10 * time.Second).SetDebug(settings.GetBool("debug", false))
	}
	var websocketAP *dto.WebsocketAP
	if mode == modeWebsocket {
		if err := retryToken(ctx, tokenRetryInitial, tokenRetryMax, func() error {
			var err error
			websocketAP, err = api.WS(ctx, nil, "")
			return err
		}, func(err error) {
			core.Logs.Warn("qqguild获取 WebSocket 接入点失败，稍后重试：%v", err)
		}); err != nil {
			return
		}
	}
	b := &bot{
		appID:   appID,
		debug:   settings.GetBool("debug", false),
		sandbox: settings.GetBool("sandbox", false),
		api:     api,
		adapter: &core.Factory{},
	}
	b.adapter.Init(platform, appID, nil)
	b.adapter.SetReplyHandler(func(msg map[string]interface{}) string {
		return b.reply(ctx, msg)
	})
	b.adapter.SetActionHandler(func(options map[string]interface{}) string {
		b.handleAction(ctx, options)
		return ""
	})
	go b.cleanupCaches(ctx)

	runtime.Lock()
	if runtime.generation != generation || ctx.Err() != nil {
		runtime.Unlock()
		b.adapter.Destroy()
		return
	}
	runtime.bot = b
	runtime.Unlock()
	defer func() {
		runtime.Lock()
		if runtime.bot == b {
			runtime.bot = nil
		}
		runtime.Unlock()
		b.adapter.Destroy()
	}()

	if mode == modeWebsocket {
		core.Logs.Info("qqguild机器人(%s) WebSocket 正在连接：%s", appID, websocketAP.URL)
		if err := runWebsocketGateway(ctx, websocketAP, tokenSource, websocketIntents); err != nil && ctx.Err() == nil {
			core.Logs.Warn("qqguild WebSocket 已停止：%v", err)
		}
		return
	}
	core.Logs.Info("qqguild机器人(%s) Webhook 已就绪：%s", appID, webhookPath)
	<-ctx.Done()
}

func receiveWebhook(c *gin.Context) {
	credentials, mode := currentCredentialsAndMode()
	if credentials == nil || mode != modeWebhook || !core.AdapterConfigEnabled(platform) {
		c.String(http.StatusServiceUnavailable, "qqguild adapter is not ready")
		return
	}
	serveWebhook(c.Writer, c.Request, credentials)
}

func serveWebhook(w http.ResponseWriter, request *http.Request, credentials *token.QQBotCredentials) {
	if credentials == nil || strings.TrimSpace(credentials.AppSecret) == "" {
		http.Error(w, "qqguild credentials are not ready", http.StatusServiceUnavailable)
		return
	}
	defer request.Body.Close()
	reader := http.MaxBytesReader(w, request.Body, maxWebhookBody)
	body, err := io.ReadAll(reader)
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			http.Error(w, "webhook body too large", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "read webhook body failed", http.StatusBadRequest)
		return
	}
	if len(body) == 0 {
		http.Error(w, "empty webhook body", http.StatusBadRequest)
		return
	}
	pass, err := signature.Verify(credentials.AppSecret, request.Header, body)
	if err != nil || !pass {
		http.Error(w, "invalid webhook signature", http.StatusUnauthorized)
		return
	}
	payload := &dto.WSPayload{}
	if err := json.Unmarshal(body, payload); err != nil {
		http.Error(w, "invalid webhook payload", http.StatusBadRequest)
		return
	}
	payload.RawMessage = body
	payload.Session = &dto.Session{AppID: credentials.AppID}
	switch payload.OPCode {
	case dto.HTTPCallbackValidation:
		data, ok := payload.Data.(map[string]interface{})
		if !ok {
			http.Error(w, "invalid validation data", http.StatusBadRequest)
			return
		}
		plainToken, plainOK := data["plain_token"].(string)
		eventTS, timeOK := data["event_ts"].(string)
		if !plainOK || !timeOK || plainToken == "" || eventTS == "" {
			http.Error(w, "invalid validation data", http.StatusBadRequest)
			return
		}
		response := webhook.GenValidationACK(&dto.WHValidationReq{PlainToken: plainToken, EventTs: eventTS}, request.Header, credentials.AppSecret)
		if len(response) == 0 {
			http.Error(w, "generate validation response failed", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = w.Write(response)
	case dto.WSHeartbeat:
		sequence, ok := payload.Data.(float64)
		if !ok || sequence < 0 || sequence > math.MaxUint32 || math.Trunc(sequence) != sequence {
			http.Error(w, "invalid heartbeat sequence", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = io.WriteString(w, webhook.GenHeartbeatACK(uint32(sequence)))
	case dto.WSDispatchEvent:
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = io.WriteString(w, webhook.GenDispatchACK(event.ParseAndHandle(payload) == nil))
	default:
		http.Error(w, "unsupported webhook opcode", http.StatusBadRequest)
	}
}

func currentBot() *bot {
	runtime.RLock()
	defer runtime.RUnlock()
	return runtime.bot
}

func currentCredentialsAndMode() (*token.QQBotCredentials, string) {
	runtime.RLock()
	defer runtime.RUnlock()
	return runtime.credentials, runtime.mode
}

func normalizeConnectionMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "ws", "wss", modeWebsocket:
		return modeWebsocket
	default:
		return modeWebhook
	}
}

type websocketGatewayRunner struct {
	sync.Mutex
	clients map[uint32]botwebsocket.WebSocket
}

func runWebsocketGateway(ctx context.Context, ap *dto.WebsocketAP, tokenSource oauth2.TokenSource, intents dto.Intent) error {
	if ap == nil || strings.TrimSpace(ap.URL) == "" {
		return errors.New("WebSocket 接入点为空")
	}
	if err := manager.CheckSessionLimit(ap); err != nil {
		return err
	}
	shards := ap.Shards
	if shards == 0 {
		shards = 1
	}
	if intents == dto.IntentNone {
		return errors.New("WebSocket intents 为空")
	}
	gatewayCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	runner := &websocketGatewayRunner{clients: map[uint32]botwebsocket.WebSocket{}}
	fatal := make(chan error, 1)
	done := make(chan struct{})
	var wg sync.WaitGroup
	for shardID := uint32(0); shardID < shards; shardID++ {
		wg.Add(1)
		go func(shardID uint32) {
			defer wg.Done()
			if err := runner.runShard(gatewayCtx, ap, tokenSource, intents, shardID, shards); err != nil && gatewayCtx.Err() == nil {
				select {
				case fatal <- err:
				default:
				}
			}
		}(shardID)
	}
	go func() {
		wg.Wait()
		close(done)
	}()
	var result error
	select {
	case <-ctx.Done():
		result = ctx.Err()
	case result = <-fatal:
	case <-done:
		result = errors.New("WebSocket sessions 已全部停止")
	}
	cancel()
	runner.closeAll()
	<-done
	return result
}

func (runner *websocketGatewayRunner) runShard(ctx context.Context, ap *dto.WebsocketAP, tokenSource oauth2.TokenSource, intents dto.Intent, shardID, shardCount uint32) error {
	if delay := time.Duration(shardID) * manager.CalcInterval(ap.SessionStartLimit.MaxConcurrency); delay > 0 {
		if !waitContext(ctx, delay) {
			return ctx.Err()
		}
	}
	session := dto.Session{
		URL:         ap.URL,
		TokenSource: tokenSource,
		Intent:      intents,
		Shards: dto.ShardConfig{
			ShardID:    shardID,
			ShardCount: shardCount,
		},
	}
	attempts := 0
	for ctx.Err() == nil {
		client := botwebsocket.ClientImpl.New(session)
		connected := true
		if err := client.Connect(); err != nil {
			if manager.CanNotIdentify(err) {
				return err
			}
			connected = false
		} else {
			runner.setClient(shardID, client)
			var err error
			if session.ID == "" {
				err = client.Identify()
			} else {
				err = client.Resume()
			}
			if err == nil {
				err = client.Listening()
			} else {
				client.Close()
			}
			session = *client.Session()
			runner.deleteClient(shardID, client)
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if err != nil && manager.CanNotIdentify(err) {
				return err
			}
			if err != nil && manager.CanNotResume(err) {
				session.ID = ""
				session.LastSeq = 0
			}
		}
		attempts = nextRetryAttempt(attempts, connected)
		if !waitContext(ctx, boundedBackoff(wsBackoffMin, wsBackoffMax, attempts)) {
			return ctx.Err()
		}
	}
	return ctx.Err()
}

// boundedBackoff scales the retry delay linearly with the attempt count so a
// flapping gateway does not hammer the QQ endpoint, mirroring the official
// adapter guidance of starting small and capping the wait.
func boundedBackoff(minimum, maximum time.Duration, attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	delay := time.Duration(attempt) * minimum
	if delay > maximum {
		return maximum
	}
	return delay
}

func nextRetryAttempt(previous int, connected bool) int {
	if connected {
		return 1
	}
	return previous + 1
}

func (runner *websocketGatewayRunner) setClient(shardID uint32, client botwebsocket.WebSocket) {
	runner.Lock()
	runner.clients[shardID] = client
	runner.Unlock()
}

func (runner *websocketGatewayRunner) deleteClient(shardID uint32, client botwebsocket.WebSocket) {
	runner.Lock()
	if runner.clients[shardID] == client {
		delete(runner.clients, shardID)
	}
	runner.Unlock()
}

func (runner *websocketGatewayRunner) closeAll() {
	runner.Lock()
	clients := make([]botwebsocket.WebSocket, 0, len(runner.clients))
	for _, client := range runner.clients {
		clients = append(clients, client)
	}
	runner.clients = map[uint32]botwebsocket.WebSocket{}
	runner.Unlock()
	for _, client := range clients {
		client.Close()
	}
}

func waitContext(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func handleATMessage(_ *dto.WSPayload, data *dto.WSATMessageData) error {
	if data == nil {
		return nil
	}
	message := dto.Message(*data)
	return dispatchMessage(&message, sceneChannel)
}

func handleGuildMessage(_ *dto.WSPayload, data *dto.WSMessageData) error {
	if data == nil {
		return nil
	}
	message := dto.Message(*data)
	return dispatchMessage(&message, sceneChannel)
}

func handleDirectMessage(_ *dto.WSPayload, data *dto.WSDirectMessageData) error {
	if data == nil {
		return nil
	}
	message := dto.Message(*data)
	return dispatchMessage(&message, sceneDirect)
}

func handleGroupATMessage(_ *dto.WSPayload, data *dto.WSGroupATMessageData) error {
	if data == nil {
		return nil
	}
	message := dto.Message(*data)
	return dispatchMessage(&message, sceneGroup)
}

func handleC2CMessage(_ *dto.WSPayload, data *dto.WSC2CMessageData) error {
	if data == nil {
		return nil
	}
	message := dto.Message(*data)
	return dispatchMessage(&message, sceneC2C)
}

// handlePlainEvent is a fallback for events without a specific handler in
// BotGo SDK v0.2.1. The QQ Bot API uses GROUP_MESSAGE_CREATE for all group
// messages, which is a different event type from GROUP_AT_MESSAGE_CREATE.
func handlePlainEvent(payload *dto.WSPayload, message []byte) error {
	switch payload.Type {
	case "GROUP_MESSAGE_CREATE":
		// GROUP_MESSAGE_CREATE has the same structure as WSMessageData
		// because it contains full messages, not just at messages.
		data := &dto.WSMessageData{}
		if err := event.ParseData(message, data); err != nil {
			return err
		}
		msg := dto.Message(*data)
		return dispatchMessage(&msg, sceneGroup)
	}
	return nil
}

func dispatchMessage(message *dto.Message, scene string) error {
	b := currentBot()
	if b == nil {
		return fmt.Errorf("qqguild adapter is not ready")
	}
	return b.receive(message, scene)
}

func (b *bot) receive(message *dto.Message, scene string) error {
	if message == nil || message.Author == nil || message.Author.Bot {
		return nil
	}
	// The gateway can replay events after a resume; drop messages seen in the
	// recent window so they are not handled and replied to twice.
	if messageID := strings.TrimSpace(message.ID); messageID != "" && !b.markMessageSeen(messageID) {
		return nil
	}
	content := strings.TrimSpace(qqmessage.ETLInput(message.Content))
	userID := strings.TrimSpace(message.Author.ID)
	if content == "" || userID == "" {
		return nil
	}

	chatID := ""
	chatName := ""
	switch scene {
	case sceneDirect:
		dmGuildID := firstNonEmpty(message.GuildID, message.SrcGuildID)
		if dmGuildID != "" {
			b.rememberDirect(userID, dmGuildID)
		}
	case sceneGroup:
		chatID = strings.TrimSpace(message.GroupID)
		chatName = chatID
		b.rememberChatScene(chatID, sceneGroup)
	case sceneC2C:
		// C2C uses the author openid as the reply target and has no chat id.
		b.rememberChatScene(userID, sceneC2C)
	default:
		chatID = strings.TrimSpace(message.ChannelID)
		chatName = chatID
	}
	userName := firstNonEmpty(memberNick(message.Member), message.Author.Username, userID)
	core.CreateNickName(&core.Nickname{
		Value:    userName,
		ID:       userID,
		Platform: platform,
		BotsID:   []string{b.appID},
	})
	if chatID != "" {
		core.CreateNickName(&core.Nickname{
			Group:    true,
			Value:    chatName,
			ID:       chatID,
			Platform: platform,
			BotsID:   []string{b.appID},
		})
	}

	params := map[string]interface{}{
		core.USER_ID:              userID,
		core.CHAT_ID:              chatID,
		core.CONETNT:              content,
		core.MESSAGE_ID:           message.ID,
		"user_name":               userName,
		"chat_name":               chatName,
		"qqguild_guild_id":        message.GuildID,
		"qqguild_source_guild_id": message.SrcGuildID,
		"qqguild_channel_id":      message.ChannelID,
		"qqguild_group_id":        message.GroupID,
		"qqguild_scene":           scene,
		"qqguild_direct":          scene == sceneDirect,
	}
	b.rememberMsgContext(message, scene, chatID, userID)
	if b.debug {
		core.Logs.Debug("qqguild处理消息：%s", string(utils.JsonMarshal(params)))
	}
	b.adapter.Receive(params)
	return nil
}

// rememberMsgContext stores the routing info needed to retract a message
// later, because the QQ v2 retract endpoints need the scene-specific target
// (group_openid / user_openid / guild_id / channel_id).
func (b *bot) rememberMsgContext(message *dto.Message, scene, chatID, userID string) {
	if message == nil || strings.TrimSpace(message.ID) == "" {
		return
	}
	target := ""
	switch scene {
	case sceneGroup:
		target = chatID
	case sceneC2C:
		target = userID
	case sceneDirect:
		target = firstNonEmpty(message.GuildID, message.SrcGuildID)
	default:
		target = firstNonEmpty(chatID, message.ChannelID)
	}
	if target == "" {
		return
	}
	b.msgCtx.Store(strings.TrimSpace(message.ID), msgContext{
		scene:     scene,
		target:    target,
		expiresAt: time.Now().Add(msgContextTTL).UnixMilli(),
	})
}

// rememberChatScene learns the routing scene of a group/user openid from
// inbound messages so later proactive pushes (carry forward, pushAdmin) that
// carry only a chat/user openid can pick the right QQ endpoint instead of
// failing as an unknown channel.
func (b *bot) rememberChatScene(target, scene string) {
	target = strings.TrimSpace(target)
	if target == "" {
		return
	}
	if _, loaded := b.chatScenes.LoadOrStore(target, scene); loaded {
		b.chatScenes.Store(target, scene)
		return
	}
	if b.chatSceneCount.Add(1) > chatSceneLimit {
		b.chatScenes.Delete(target)
		b.chatSceneCount.Add(-1)
	}
}

func (b *bot) lookupChatScene(target string) string {
	target = strings.TrimSpace(target)
	if target == "" {
		return ""
	}
	if value, ok := b.chatScenes.Load(target); ok {
		if scene, ok := value.(string); ok {
			return scene
		}
	}
	return ""
}

func (b *bot) reply(ctx context.Context, msg map[string]interface{}) string {
	scene := strings.TrimSpace(stringValue(msg["qqguild_scene"]))
	// Keep compatibility with messages created before qqguild_scene existed.
	if scene == "" && boolValue(msg["qqguild_direct"]) {
		scene = sceneDirect
	}
	// Proactive pushes carry only a chat/user openid; route them by the
	// explicit chat_type hint (group/private) from the core push, then by the
	// scene learned from inbound messages, falling back to the channel branch
	// when unknown.
	if scene == "" {
		switch strings.TrimSpace(stringValue(msg["chat_type"])) {
		case "group":
			scene = sceneGroup
		case "private":
			scene = sceneC2C
		}
	}
	if scene == "" {
		if learned := b.lookupChatScene(firstNonEmpty(
			stringValue(msg[core.CHAT_ID]),
			stringValue(msg[core.USER_ID]),
		)); learned != "" {
			scene = learned
		}
	}
	segments := parseReplySegments(stringValue(msg[core.CONETNT]))
	text := joinReplyText(segments)
	media := mediaSegments(segments)
	messageID := strings.TrimSpace(stringValue(msg[core.MESSAGE_ID]))
	if text == "" && len(media) == 0 {
		return ""
	}
	requestCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	switch scene {
	case sceneGroup, sceneC2C:
		if len(media) > 0 {
			return b.sendRichMediaReply(requestCtx, msg, scene, messageID, text, media)
		}
		payload := &dto.MessageToCreate{
			Content: text,
			MsgID:   messageID,
			MsgSeq:  b.nextReplySeq(messageID),
		}
		if settings.GetBool("markdown", false) {
			payload.MsgType = dto.MarkdownMsg
			payload.Markdown = &dto.Markdown{Content: b.markdownReplyText(msg, scene, text)}
			payload.Content = ""
		}
		result, err := b.postGroupOrC2C(requestCtx, msg, scene, payload)
		if err != nil && payload.MsgType == dto.MarkdownMsg {
			core.Logs.Warn("qqguild发送 Markdown 消息失败，回退纯文本：%v", err)
			result, err = b.postGroupOrC2C(requestCtx, msg, scene, &dto.MessageToCreate{
				Content: text,
				MsgID:   messageID,
				MsgSeq:  b.nextReplySeq(messageID),
			})
		}
		if err != nil {
			core.Logs.Warn("qqguild发送消息失败：%v", err)
			return ""
		}
		if result == nil {
			return ""
		}
		return result.ID
	case sceneDirect:
		userID := strings.TrimSpace(stringValue(msg[core.USER_ID]))
		dmGuildID := firstNonEmpty(
			stringValue(msg["qqguild_guild_id"]),
			stringValue(msg["qqguild_source_guild_id"]),
			b.directGuildID(userID),
		)
		if dmGuildID == "" {
			core.Logs.Warn("qqguild发送私信失败：缺少私信 guild_id")
			return ""
		}
		// Guild direct messages have no v2 rich media endpoint; drop media.
		result, err := b.api.PostDirectMessage(requestCtx, &dto.DirectMessage{GuildID: dmGuildID}, &dto.MessageToCreate{
			Content: text,
			MsgID:   messageID,
			MsgSeq:  b.nextReplySeq(messageID),
		})
		if err != nil {
			core.Logs.Warn("qqguild发送消息失败：%v", err)
			return ""
		}
		if result == nil {
			return ""
		}
		return result.ID
	default:
		// Channels have no v2 rich media endpoint either, but the plain
		// message payload accepts a single image URL.
		channelImage := ""
		for _, item := range media {
			if item.kind == "image" {
				channelImage = item.url
				break
			}
		}
		channelID := firstNonEmpty(stringValue(msg[core.CHAT_ID]), stringValue(msg["qqguild_channel_id"]))
		if channelID != "" {
			result, err := b.api.PostMessage(requestCtx, channelID, &dto.MessageToCreate{
				Content: text,
				Image:   channelImage,
				MsgID:   messageID,
				MsgSeq:  b.nextReplySeq(messageID),
			})
			if err != nil {
				core.Logs.Warn("qqguild发送消息失败：%v", err)
				return ""
			}
			if result == nil {
				return ""
			}
			return result.ID
		}
		// Proactive pushes (e.g. pushAdmin) only carry a user openid; fall back
		// to the C2C endpoint instead of failing on a missing channel id.
		userID := strings.TrimSpace(stringValue(msg[core.USER_ID]))
		if userID == "" {
			core.Logs.Warn("qqguild发送消息失败：缺少 channel_id 和 user_id")
			return ""
		}
		result, err := b.api.PostC2CMessage(requestCtx, userID, &dto.MessageToCreate{
			Content: text,
			MsgID:   messageID,
			MsgSeq:  b.nextReplySeq(messageID),
		})
		if err != nil {
			core.Logs.Warn("qqguild发送 C2C 消息失败：%v", err)
			return ""
		}
		if result == nil {
			return ""
		}
		return result.ID
	}
}

// markdownReplyText prepends a QQ markdown at-tag for group replies so the
// sender sees the reply as a mention; the tag only renders in markdown
// messages, so plain-text and fallback sends keep the raw text.
func (b *bot) markdownReplyText(msg map[string]interface{}, scene, text string) string {
	if scene != sceneGroup || !settings.GetBool("at", true) {
		return text
	}
	openid := strings.TrimSpace(stringValue(msg[core.USER_ID]))
	if openid == "" {
		return text
	}
	return fmt.Sprintf("<qqbot-at-user id=\"%s\" />\n%s", openid, text)
}

func (b *bot) postGroupOrC2C(ctx context.Context, msg map[string]interface{}, scene string, payload *dto.MessageToCreate) (*dto.Message, error) {
	switch scene {
	case sceneGroup:
		groupID := firstNonEmpty(stringValue(msg["qqguild_group_id"]), stringValue(msg[core.CHAT_ID]))
		if groupID == "" {
			return nil, fmt.Errorf("缺少 group_id")
		}
		return b.api.PostGroupMessage(ctx, groupID, payload)
	default:
		// C2C proactive pushes may carry the user openid in chat_id.
		userID := firstNonEmpty(stringValue(msg[core.USER_ID]), stringValue(msg[core.CHAT_ID]))
		if userID == "" {
			return nil, fmt.Errorf("缺少 user_id")
		}
		return b.api.PostC2CMessage(ctx, userID, payload)
	}
}

// markMessageSeen records a message ID in the dedupe window and reports
// whether it is new.
func (b *bot) markMessageSeen(messageID string) bool {
	now := time.Now()
	if value, ok := b.seen.Load(messageID); ok {
		if expiry, ok := value.(int64); ok && now.UnixMilli() < expiry {
			return false
		}
	}
	b.seen.Store(messageID, now.Add(seenMessageTTL).UnixMilli())
	return true
}

func (b *bot) directGuildID(userID string) string {
	if value, ok := b.direct.Load(userID); ok {
		item := value.(directSession)
		if time.Now().UnixMilli() < item.expiresAt {
			return item.guildID
		}
		b.direct.CompareAndDelete(userID, item)
	}
	return ""
}

type replySegment struct {
	cqType string // "" for text; image/video/record/file otherwise
	value  string
}

type replyMedia struct {
	kind     string // image / video / voice / file
	url      string
	fileType uint64
}

var qqCQSegmentPattern = regexp.MustCompile(`\[CQ:([a-zA-Z]+),([^\[\]]*)\]`)

var cqValueReplacer = strings.NewReplacer("&#44;", ",", "&#91;", "[", "&#93;", "]", "&amp;", "&")

// parseReplySegments splits reply content into text and CQ media segments.
// Plugins emit media as [CQ:image,url=...]/[CQ:video,url=...] codes.
func parseReplySegments(content string) []replySegment {
	segments := []replySegment{}
	matches := qqCQSegmentPattern.FindAllStringSubmatchIndex(content, -1)
	if len(matches) == 0 {
		if text := strings.TrimSpace(content); text != "" {
			segments = append(segments, replySegment{value: text})
		}
		return segments
	}
	last := 0
	for _, match := range matches {
		if text := strings.TrimSpace(content[last:match[0]]); text != "" {
			segments = append(segments, replySegment{value: text})
		}
		last = match[1]
		attrs := parseCQAttributes(content[match[4]:match[5]])
		source := firstNonEmpty(attrs["url"], attrs["file"])
		if source == "" {
			continue
		}
		segments = append(segments, replySegment{
			cqType: strings.ToLower(content[match[2]:match[3]]),
			value:  source,
		})
	}
	if text := strings.TrimSpace(content[last:]); text != "" {
		segments = append(segments, replySegment{value: text})
	}
	return segments
}

func parseCQAttributes(raw string) map[string]string {
	attrs := map[string]string{}
	for _, part := range strings.Split(raw, ",") {
		key, value, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		attrs[strings.ToLower(strings.TrimSpace(key))] = cqValueReplacer.Replace(strings.TrimSpace(value))
	}
	return attrs
}

// mediaSegments keeps only http(s) media the QQ v2 rich media API accepts;
// local paths cannot be uploaded without a host media resolver.
func mediaSegments(segments []replySegment) []replyMedia {
	items := []replyMedia{}
	for _, segment := range segments {
		if segment.cqType == "" {
			continue
		}
		item, ok := replyMediaItem(segment)
		if !ok {
			core.Logs.Warn("qqguild忽略不支持的媒体链接：%s", segment.value)
			continue
		}
		items = append(items, item)
	}
	return items
}

func replyMediaItem(segment replySegment) (replyMedia, bool) {
	if !isHTTPURL(segment.value) {
		return replyMedia{}, false
	}
	item := replyMedia{url: segment.value}
	switch segment.cqType {
	case "image":
		item.kind, item.fileType = "image", 1
	case "video":
		item.kind, item.fileType = "video", 2
	case "record", "voice", "audio":
		item.kind, item.fileType = "voice", 3
	case "file":
		item.kind, item.fileType = "file", 4
	default:
		return replyMedia{}, false
	}
	return item, true
}

func joinReplyText(segments []replySegment) string {
	texts := []string{}
	for _, segment := range segments {
		if segment.cqType == "" && segment.value != "" {
			texts = append(texts, segment.value)
		}
	}
	return strings.Join(texts, "\n")
}

func isHTTPURL(value string) bool {
	parsed, err := url.Parse(strings.TrimSpace(value))
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != ""
}

// sendRichMediaReply uploads each media item through the QQ v2 files endpoint
// and sends it as a msg_type=7 message; the text rides along on the first one.
func (b *bot) sendRichMediaReply(ctx context.Context, msg map[string]interface{}, scene, messageID, text string, items []replyMedia) string {
	target := richMediaTarget(msg, scene)
	if target == "" {
		core.Logs.Warn("qqguild发送富媒体消息失败：缺少发送目标")
		return ""
	}
	lastID := ""
	for i, item := range items {
		fileInfo, err := b.uploadRichMedia(ctx, scene, target, item)
		if err != nil {
			core.Logs.Warn("qqguild上传富媒体失败：%v", err)
			return lastID
		}
		segmentText := ""
		if i == 0 {
			segmentText = text
		}
		id, err := b.sendRichMedia(ctx, scene, target, fileInfo, segmentText, messageID)
		if err != nil {
			core.Logs.Warn("qqguild发送富媒体消息失败：%v", err)
			return lastID
		}
		lastID = id
	}
	return lastID
}

func richMediaTarget(msg map[string]interface{}, scene string) string {
	switch scene {
	case sceneGroup:
		return firstNonEmpty(stringValue(msg["qqguild_group_id"]), stringValue(msg[core.CHAT_ID]))
	default:
		return strings.TrimSpace(stringValue(msg[core.USER_ID]))
	}
}

func (b *bot) apiBase() string {
	if b.sandbox {
		return qqSandboxAPIHost
	}
	return qqAPIHostDefault
}

// uploadRichMedia posts to /v2/groups|users/{id}/files with srv_send_msg=false
// and returns the file_info needed by the msg_type=7 send body. The SDK's
// []byte-based MediaInfo DTO base64-wraps file_info in both directions, which
// QQ rejects, so these calls go through the authenticated Transport instead.
func (b *bot) uploadRichMedia(ctx context.Context, scene, target string, item replyMedia) (string, error) {
	endpoint := "/v2/users/" + url.PathEscape(target) + "/files"
	if scene == sceneGroup {
		endpoint = "/v2/groups/" + url.PathEscape(target) + "/files"
	}
	body := map[string]interface{}{
		"file_type":    item.fileType,
		"url":          item.url,
		"srv_send_msg": false,
	}
	raw, err := b.api.Transport(ctx, http.MethodPost, b.apiBase()+endpoint, body)
	if err != nil {
		return "", err
	}
	resp := map[string]interface{}{}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return "", fmt.Errorf("解析上传响应失败：%w", err)
	}
	fileInfo := strings.TrimSpace(stringValue(resp["file_info"]))
	if fileInfo == "" {
		return "", fmt.Errorf("上传响应缺少 file_info")
	}
	return fileInfo, nil
}

func (b *bot) sendRichMedia(ctx context.Context, scene, target, fileInfo, content, messageID string) (string, error) {
	endpoint := "/v2/users/" + url.PathEscape(target) + "/messages"
	if scene == sceneGroup {
		endpoint = "/v2/groups/" + url.PathEscape(target) + "/messages"
	}
	body := map[string]interface{}{
		"msg_type": dto.RichMediaMsg,
		"media":    map[string]string{"file_info": fileInfo},
		"msg_id":   messageID,
		"msg_seq":  b.nextReplySeq(messageID),
	}
	if content != "" {
		body["content"] = content
	}
	raw, err := b.api.Transport(ctx, http.MethodPost, b.apiBase()+endpoint, body)
	if err != nil {
		return "", err
	}
	resp := map[string]interface{}{}
	_ = json.Unmarshal(raw, &resp)
	return stringValue(resp["id"]), nil
}

// handleAction serves core Action requests; delete_message retracts a previously
// received message through the scene-specific retract endpoint.
func (b *bot) handleAction(ctx context.Context, options map[string]interface{}) {
	if strings.ToLower(strings.TrimSpace(stringValue(options["type"]))) != "delete_message" {
		return
	}
	messageID := strings.TrimSpace(stringValue(options["message_id"]))
	if messageID == "" {
		return
	}
	value, ok := b.msgCtx.Load(messageID)
	if !ok {
		core.Logs.Warn("qqguild撤回消息失败：未记录消息 %s 的上下文", messageID)
		return
	}
	item, ok := value.(msgContext)
	if !ok || time.Now().UnixMilli() >= item.expiresAt {
		b.msgCtx.Delete(messageID)
		core.Logs.Warn("qqguild撤回消息失败：消息 %s 上下文已过期", messageID)
		return
	}
	requestCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	var err error
	switch item.scene {
	case sceneGroup:
		err = b.api.RetractGroupMessage(requestCtx, item.target, messageID)
	case sceneC2C:
		err = b.api.RetractC2CMessage(requestCtx, item.target, messageID)
	case sceneDirect:
		err = b.api.RetractDMMessage(requestCtx, item.target, messageID)
	default:
		err = b.api.RetractMessage(requestCtx, item.target, messageID)
	}
	if err != nil {
		core.Logs.Warn("qqguild撤回消息 %s 失败：%v", messageID, err)
		return
	}
	b.msgCtx.Delete(messageID)
	core.Logs.Info("qqguild已撤回消息：%s", messageID)
}

func (b *bot) rememberDirect(userID, guildID string) {
	item := directSession{
		guildID:   guildID,
		expiresAt: time.Now().Add(directSessionTTL).UnixMilli(),
	}
	b.direct.Store(userID, item)
}

func (b *bot) nextReplySeq(messageID string) uint32 {
	if messageID == "" {
		return 0
	}
	for {
		now := time.Now()
		counter := &replyCounter{expiresAt: now.Add(replySequenceTTL).UnixMilli()}
		value, loaded := b.replySeq.LoadOrStore(messageID, counter)
		if !loaded {
			return counter.sequence.Add(1)
		}
		current, ok := value.(*replyCounter)
		if ok && now.UnixMilli() < current.expiresAt {
			return current.sequence.Add(1)
		}
		b.replySeq.CompareAndDelete(messageID, value)
	}
}

func (b *bot) cleanupCaches(ctx context.Context) {
	ticker := time.NewTicker(cacheCleanupEvery)
	defer ticker.Stop()
	for {
		select {
		case now := <-ticker.C:
			b.deleteExpiredCaches(now)
		case <-ctx.Done():
			return
		}
	}
}

func (b *bot) deleteExpiredCaches(now time.Time) {
	nowMillis := now.UnixMilli()
	b.direct.Range(func(key, value interface{}) bool {
		item, ok := value.(directSession)
		if !ok || nowMillis >= item.expiresAt {
			b.direct.CompareAndDelete(key, value)
		}
		return true
	})
	b.replySeq.Range(func(key, value interface{}) bool {
		counter, ok := value.(*replyCounter)
		if !ok || nowMillis >= counter.expiresAt {
			b.replySeq.CompareAndDelete(key, value)
		}
		return true
	})
	b.msgCtx.Range(func(key, value interface{}) bool {
		item, ok := value.(msgContext)
		if !ok || nowMillis >= item.expiresAt {
			b.msgCtx.CompareAndDelete(key, value)
		}
		return true
	})
	b.seen.Range(func(key, value interface{}) bool {
		expiry, ok := value.(int64)
		if !ok || nowMillis >= expiry {
			b.seen.CompareAndDelete(key, value)
		}
		return true
	})
}

func retryToken(ctx context.Context, initialDelay, maxDelay time.Duration, operation func() error, onRetry func(error)) error {
	delay := initialDelay
	for {
		err := operation()
		if err == nil {
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if onRetry != nil {
			onRetry(err)
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return ctx.Err()
		case <-timer.C:
		}
		if delay < maxDelay {
			delay *= 2
			if delay > maxDelay {
				delay = maxDelay
			}
		}
	}
}

func memberNick(member *dto.Member) string {
	if member == nil {
		return ""
	}
	return member.Nick
}

func boolValue(value interface{}) bool {
	switch value := value.(type) {
	case bool:
		return value
	default:
		switch strings.ToLower(strings.TrimSpace(stringValue(value))) {
		case "true", "1", "yes", "on":
			return true
		default:
			return false
		}
	}
}

func stringValue(value interface{}) string {
	if value == nil {
		return ""
	}
	return fmt.Sprint(value)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}
