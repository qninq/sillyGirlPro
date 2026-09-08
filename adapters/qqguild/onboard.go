package qqguild

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/smallfawn/sillyGirl/core"
)

// QR onboarding drives the official q.qq.com bind service: a random client
// key creates a bind task, the user scans the QR page with mobile QQ, and
// polling returns the bot credentials encrypted with that key (AES-256-GCM).
const (
	qqBindHostDefault   = "q.qq.com"
	onboardKeyTTL       = 10 * time.Minute
	onboardMaxKeys      = 256
	onboardPollTotal    = 50 * time.Second
	onboardPollStep     = 2 * time.Second
	onboardHTTPTimeout  = 20 * time.Second
	onboardMaxBodyBytes = 2 << 20
)

type onboardTask struct {
	bindKey   string
	createdAt time.Time
}

var onboardKeys = struct {
	sync.Mutex
	tasks map[string]onboardTask
}{tasks: map[string]onboardTask{}}

func onboardBindBaseURL() string {
	if host := strings.TrimSpace(os.Getenv("QQGUILD_BIND_HOST")); host != "" {
		host = strings.TrimRight(strings.TrimPrefix(strings.TrimPrefix(host, "https://"), "http://"), "/")
		return "https://" + host
	}
	return "https://" + qqBindHostDefault
}

func onboardQRCodeURL(taskID string) string {
	return onboardBindBaseURL() + "/qqbot/openclaw/connect.html?task_id=" + url.PathEscape(taskID) + "&_wv=2"
}

func onboardPruneLocked(now time.Time) {
	for key, task := range onboardKeys.tasks {
		if now.Sub(task.createdAt) >= onboardKeyTTL {
			delete(onboardKeys.tasks, key)
		}
	}
	for len(onboardKeys.tasks) > onboardMaxKeys {
		oldestKey := ""
		var oldest time.Time
		for key, task := range onboardKeys.tasks {
			if oldestKey == "" || task.createdAt.Before(oldest) {
				oldestKey, oldest = key, task.createdAt
			}
		}
		delete(onboardKeys.tasks, oldestKey)
	}
}

func onboardPost(ctx *gin.Context, endpoint string, payload map[string]interface{}) (map[string]interface{}, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	requestCtx, cancel := context.WithTimeout(ctx.Request.Context(), onboardHTTPTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(requestCtx, http.MethodPost, onboardBindBaseURL()+endpoint, bytes.NewReader(encoded))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, onboardMaxBodyBytes))
	if err != nil {
		return nil, err
	}
	body := map[string]interface{}{}
	_ = json.Unmarshal(raw, &body)
	retcode := 0
	if value, ok := body["retcode"].(float64); ok {
		retcode = int(value)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 || retcode != 0 {
		detail := firstNonEmpty(
			stringValue(body["msg"]),
			stringValue(body["message"]),
			fmt.Sprintf("HTTP %d", resp.StatusCode),
		)
		return nil, errors.New("QQ 绑定服务返回失败：" + detail)
	}
	return body, nil
}

// decryptOnboardSecret unwraps bot_encrypt_secret: base64(nonce || ciphertext+tag)
// AES-256-GCM with the client bind key.
func decryptOnboardSecret(encrypted, bindKey string) (string, error) {
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(bindKey))
	if err != nil || len(key) != 32 {
		return "", errors.New("扫码凭证密钥格式异常")
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encrypted))
	if err != nil {
		return "", errors.New("扫码凭证密文格式异常")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil || len(raw) <= gcm.NonceSize()+gcm.Overhead() {
		return "", errors.New("扫码凭证密文格式异常")
	}
	plain, err := gcm.Open(nil, raw[:gcm.NonceSize()], raw[gcm.NonceSize():], nil)
	if err != nil {
		return "", errors.New("扫码凭证解密失败")
	}
	return string(plain), nil
}

// initOnboard registers the QR onboarding endpoints. Scanning binds the
// developer's QQ account to a bot registered at q.qq.com; on confirm the
// credentials are decrypted server-side and stored, which also restarts the
// adapter through the existing config watcher.
func initOnboard() {
	core.GinApi(core.POST, "/api/admin/qqguild-onboard-tasks", core.RequireAuth, func(ctx *gin.Context) {
		bindKeyRaw := make([]byte, 32)
		if _, err := rand.Read(bindKeyRaw); err != nil {
			core.ApiInternalError(ctx, err.Error())
			return
		}
		bindKey := base64.StdEncoding.EncodeToString(bindKeyRaw)
		body, err := onboardPost(ctx, "/lite/create_bind_task", map[string]interface{}{"key": bindKey})
		if err != nil {
			core.ApiFail(ctx, err.Error())
			return
		}
		data, _ := body["data"].(map[string]interface{})
		taskID := strings.TrimSpace(stringValue(data["task_id"]))
		if taskID == "" {
			core.ApiFail(ctx, "QQ 绑定服务响应缺少 task_id")
			return
		}
		now := time.Now()
		onboardKeys.Lock()
		onboardPruneLocked(now)
		onboardKeys.tasks[taskID] = onboardTask{bindKey: bindKey, createdAt: now}
		onboardKeys.Unlock()
		core.ApiCreated(ctx, "/api/admin/qqguild-onboard-tasks/"+url.PathEscape(taskID)+"/polls", map[string]interface{}{
			"task_id":     taskID,
			"session_key": taskID,
			"qr_code_url": onboardQRCodeURL(taskID),
			"expires_at":  now.Add(onboardKeyTTL).Unix(),
		})
	})

	core.GinApi(core.POST, "/api/admin/qqguild-onboard-tasks/:task_id/polls", core.RequireAuth, func(ctx *gin.Context) {
		taskID := strings.TrimSpace(ctx.Param("task_id"))
		onboardKeys.Lock()
		onboardPruneLocked(time.Now())
		task, ok := onboardKeys.tasks[taskID]
		onboardKeys.Unlock()
		if !ok {
			core.ApiNotFound(ctx, "绑定任务不存在或已过期，请重新生成二维码")
			return
		}
		scanned := false
		deadline := time.Now().Add(onboardPollTotal)
		for {
			body, err := onboardPost(ctx, "/lite/poll_bind_result", map[string]interface{}{"task_id": taskID})
			if err == nil {
				data, _ := body["data"].(map[string]interface{})
				status := 0
				if value, ok := data["status"].(float64); ok {
					status = int(value)
				}
				switch status {
				case 2:
					appID := strings.TrimSpace(stringValue(firstMap(data, "bot_appid", "appid", "app_id")))
					encrypted := strings.TrimSpace(stringValue(firstMap(data, "bot_encrypt_secret", "encrypt_secret", "encrypted_secret")))
					scannerOpenID := strings.TrimSpace(stringValue(data["user_openid"]))
					if appID == "" || encrypted == "" {
						core.ApiFail(ctx, "扫码成功但未返回完整机器人凭证")
						return
					}
					secret, err := decryptOnboardSecret(encrypted, task.bindKey)
					if err != nil {
						core.ApiFail(ctx, err.Error())
						return
					}
					settings.Set("app_id", appID)
					settings.Set("app_secret", secret)
					settings.Set("enable", "true")
					if scannerOpenID != "" {
						rememberOnboardMaster(scannerOpenID)
					}
					onboardKeys.Lock()
					delete(onboardKeys.tasks, taskID)
					onboardKeys.Unlock()
					core.Logs.Info("qqguild扫码绑定成功，已保存机器人配置：%s（扫码人：%s）", appID, scannerOpenID)
					core.ApiOK(ctx, map[string]interface{}{
						"status":        "confirmed",
						"app_id":        appID,
						"client_secret": secret,
						"user_openid":   scannerOpenID,
						"qr_code_url":   onboardQRCodeURL(taskID),
						"message":       "绑定成功，机器人配置已保存并启用，扫码人已加入管理员。",
					})
					return
				case 3:
					onboardKeys.Lock()
					delete(onboardKeys.tasks, taskID)
					onboardKeys.Unlock()
					core.ApiOK(ctx, map[string]interface{}{"status": "expired", "message": "二维码已过期，请重新生成。"})
					return
				case 1:
					scanned = true
				default:
					// status 0: still waiting for a scan.
				}
			}
			if time.Now().After(deadline) || !waitContext(ctx.Request.Context(), onboardPollStep) {
				core.ApiOK(ctx, map[string]interface{}{
					"status": map[bool]string{true: "scanned", false: "wait"}[scanned],
					"message": map[bool]string{
						true:  "已扫码，请在手机 QQ 上确认。",
						false: "等待扫码，请使用手机 QQ 扫描二维码。",
					}[scanned],
				})
				return
			}
		}
	})
}

func firstMap(data map[string]interface{}, keys ...string) interface{} {
	for _, key := range keys {
		if value, ok := data[key]; ok && value != nil {
			return value
		}
	}
	return ""
}

// rememberOnboardMaster adds the QQ account that scanned the QR code to the
// platform masters list: whoever confirmed the binding owns the bot.
func rememberOnboardMaster(openid string) {
	existing := strings.Split(strings.Trim(settings.GetString("masters"), "&"), "&")
	for _, item := range existing {
		if strings.TrimSpace(item) == openid {
			return
		}
	}
	existing = append(existing, openid)
	kept := make([]string, 0, len(existing))
	for _, item := range existing {
		if item = strings.TrimSpace(item); item != "" {
			kept = append(kept, item)
		}
	}
	settings.Set("masters", strings.Join(kept, "&"))
}
