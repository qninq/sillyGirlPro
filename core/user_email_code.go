package core

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/smtp"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// 注册邮箱验证码：内存态，5 分钟有效，同邮箱（含同 IP）60 秒内只能发送一次。
// 验证码按用途（注册 / 重置密码）隔离存储，注册用途的验证码不能用于重置密码。
const (
	emailCodeTTL         = 5 * time.Minute
	emailCodeSendWindow  = time.Minute
	maxTrackedEmailCodes = 4096
	emailCodeScope       = "email-code"
	// 连续错误达到该次数即作废验证码，防止 6 位数字码在有效期内被暴力枚举。
	maxEmailCodeVerifyAttempts = 5

	EmailCodePurposeRegister      = "register"
	EmailCodePurposePasswordReset = "password-reset"
)

// EmailCodePurposeValid 校验验证码用途是否受支持。
func EmailCodePurposeValid(purpose string) bool {
	return purpose == EmailCodePurposeRegister || purpose == EmailCodePurposePasswordReset
}

type emailCodeState struct {
	Code      string
	ExpiresAt time.Time
	FirstSent time.Time
	Attempts  int
}

var (
	emailCodeLock sync.Mutex
	emailCodes    = map[string]*emailCodeState{}
)

// smtpSettings 读取基础设置里的邮箱验证配置；任一必填项为空视为未配置。
func smtpSettings() (host, port, password, sender string, ok bool) {
	host = strings.TrimSpace(sillyGirl.GetString("smtp_host"))
	port = strings.TrimSpace(sillyGirl.GetString("smtp_port"))
	password = strings.TrimSpace(sillyGirl.GetString("smtp_password"))
	sender = strings.TrimSpace(sillyGirl.GetString("smtp_sender"))
	if host == "" || port == "" || password == "" || sender == "" {
		return "", "", "", "", false
	}
	return host, port, password, sender, true
}

func emailCodeKeys(ctx *gin.Context, email, purpose string) []string {
	email = strings.ToLower(strings.TrimSpace(email))
	return []string{
		emailCodeScope + "|" + purpose + "|email:" + email,
		// IP 限频跨用途共享：同一 IP 一分钟内最多发一封验证邮件。
		emailCodeScope + "|ip:" + ctx.ClientIP(),
	}
}

func pruneEmailCodesLocked(now time.Time) {
	for key, state := range emailCodes {
		if state == nil || (state.ExpiresAt.Before(now) && now.Sub(state.FirstSent) > emailCodeSendWindow) {
			delete(emailCodes, key)
		}
	}
	for len(emailCodes) >= maxTrackedEmailCodes {
		oldestKey := ""
		var oldest time.Time
		for key, state := range emailCodes {
			if state != nil && (oldestKey == "" || state.FirstSent.Before(oldest)) {
				oldestKey, oldest = key, state.FirstSent
			}
		}
		if oldestKey == "" {
			break
		}
		delete(emailCodes, oldestKey)
	}
}

// verifyUserEmailCode 校验指定用途的验证码；成功后立即作废，连续错误超限同样作废，防止暴力枚举。
func verifyUserEmailCode(email, code, purpose string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	code = strings.TrimSpace(code)
	if code == "" {
		return errors.New("请输入邮箱验证码")
	}
	emailCodeLock.Lock()
	defer emailCodeLock.Unlock()
	pruneEmailCodesLocked(time.Now())
	state := emailCodes[emailCodeScope+"|"+purpose+"|email:"+email]
	if state == nil || time.Now().After(state.ExpiresAt) {
		return errors.New("验证码不存在或已过期，请重新获取")
	}
	if state.Code == "" {
		return errors.New("验证码已失效，请重新获取")
	}
	if state.Code != code {
		state.Attempts++
		if state.Attempts >= maxEmailCodeVerifyAttempts {
			// 作废验证码但保留条目至自然过期，同邮箱的发码限频窗口不被重置。
			state.Code = ""
			return errors.New("验证码错误次数过多，请重新获取")
		}
		return errors.New("验证码错误")
	}
	delete(emailCodes, emailCodeScope+"|"+purpose+"|email:"+email)
	return nil
}

func generateEmailCode() string {
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "000000"
	}
	return fmt.Sprintf("%06d", n.Int64())
}

// sendMail 通过基础设置里的 SMTP 服务发送纯文本邮件。
func sendMail(host, port, password, sender, recipient, subject, body string) error {
	addr := host + ":" + port
	auth := smtp.PlainAuth("", sender, password, host)
	headers := "From: " + sender + "\r\n" +
		"To: " + recipient + "\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: text/plain; charset=UTF-8\r\n" +
		"Subject: =?UTF-8?B?" + base64.StdEncoding.EncodeToString([]byte(subject)) + "?=\r\n"
	return smtp.SendMail(addr, auth, sender, []string{recipient}, []byte(headers+"\r\n"+body))
}

func sendEmailCodeMail(host, port, password, sender, recipient, code string) error {
	body := "你的注册验证码是：" + code + "，5 分钟内有效。如非本人操作请忽略本邮件。\r\n"
	return sendMail(host, port, password, sender, recipient, "SillyGirl 注册验证码", body)
}

func init() {
	// 发送验证码：公开接口（注册 / 重置密码前），依赖发送频率限制防滥用。
	GinApi(POST, "/api/user/email-codes", func(ctx *gin.Context) {
		payload := struct {
			Email   string `json:"email"`
			Purpose string `json:"purpose"`
		}{}
		if err := json.NewDecoder(ctx.Request.Body).Decode(&payload); err != nil {
			ApiFail(ctx, "请求体不是有效 JSON")
			return
		}
		email := strings.ToLower(strings.TrimSpace(payload.Email))
		purpose := strings.ToLower(strings.TrimSpace(payload.Purpose))
		if purpose == "" {
			purpose = EmailCodePurposeRegister
		}
		if !EmailCodePurposeValid(purpose) {
			ApiUnprocessable(ctx, "不支持的验证码用途")
			return
		}
		if _, err := normalUserQQFromEmail(email); err != nil {
			ApiUnprocessable(ctx, err.Error())
			return
		}
		existing, _ := loadNormalUser(strings.SplitN(email, "@", 2)[0])
		if purpose == EmailCodePurposeRegister && existing != nil {
			ApiConflict(ctx, "该 QQ 号已注册，请直接登录")
			return
		}
		host, port, password, sender, ok := smtpSettings()
		if !ok {
			ApiError(ctx, http.StatusServiceUnavailable, "管理员未配置邮箱验证服务，无法发送验证码")
			return
		}
		if purpose == EmailCodePurposePasswordReset && existing == nil {
			ApiNotFound(ctx, "该邮箱未注册")
			return
		}

		emailCodeLock.Lock()
		now := time.Now()
		pruneEmailCodesLocked(now)
		for _, key := range emailCodeKeys(ctx, email, purpose) {
			if state := emailCodes[key]; state != nil && now.Sub(state.FirstSent) < emailCodeSendWindow {
				emailCodeLock.Unlock()
				ApiError(ctx, http.StatusTooManyRequests, "发送过于频繁，请 1 分钟后再试")
				return
			}
		}
		code := generateEmailCode()
		state := &emailCodeState{Code: code, ExpiresAt: now.Add(emailCodeTTL), FirstSent: now}
		emailCodes[emailCodeScope+"|"+purpose+"|email:"+email] = state
		emailCodes[emailCodeScope+"|ip:"+ctx.ClientIP()] = state
		emailCodeLock.Unlock()

		if err := sendEmailCodeMail(host, port, password, sender, email, code); err != nil {
			emailCodeLock.Lock()
			delete(emailCodes, emailCodeScope+"|"+purpose+"|email:"+email)
			delete(emailCodes, emailCodeScope+"|ip:"+ctx.ClientIP())
			emailCodeLock.Unlock()
			ApiError(ctx, http.StatusBadGateway, "验证码邮件发送失败，请稍后重试")
			return
		}
		ApiCreated(ctx, "/api/user/email-codes", gin.H{
			"expires_in": int(emailCodeTTL.Seconds()),
			"resend_in":  int(emailCodeSendWindow.Seconds()),
		})
	})
}
