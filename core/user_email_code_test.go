package core

import (
	"strings"
	"testing"
	"time"
)

func TestVerifyUserEmailCodeLifecycle(t *testing.T) {
	emailCodeLock.Lock()
	emailCodes = map[string]*emailCodeState{}
	emailCodeLock.Unlock()

	email := "12345678@qq.com"
	if err := verifyUserEmailCode(email, "123456", EmailCodePurposeRegister); err == nil {
		t.Fatal("empty store should reject any code")
	}

	// 写入一个有效验证码（模拟已发送）。
	emailCodeLock.Lock()
	emailCodes[emailCodeScope+"|"+EmailCodePurposeRegister+"|email:"+email] = &emailCodeState{
		Code:      "654321",
		ExpiresAt: time.Now().Add(emailCodeTTL),
		FirstSent: time.Now(),
	}
	emailCodeLock.Unlock()

	if err := verifyUserEmailCode(email, "000000", EmailCodePurposeRegister); err == nil || !strings.Contains(err.Error(), "验证码错误") {
		t.Fatalf("wrong code should be rejected: %v", err)
	}
	if err := verifyUserEmailCode(email, "654321", EmailCodePurposeRegister); err != nil {
		t.Fatalf("correct code should pass: %v", err)
	}
	// 验证成功后立即作废。
	if err := verifyUserEmailCode(email, "654321", EmailCodePurposeRegister); err == nil {
		t.Fatal("code should be consumed after successful verification")
	}

	// 过期验证码应被拒绝。
	emailCodeLock.Lock()
	emailCodes[emailCodeScope+"|"+EmailCodePurposeRegister+"|email:"+email] = &emailCodeState{
		Code:      "111222",
		ExpiresAt: time.Now().Add(-time.Second),
		FirstSent: time.Now().Add(-emailCodeTTL),
	}
	emailCodeLock.Unlock()
	if err := verifyUserEmailCode(email, "111222", EmailCodePurposeRegister); err == nil || !strings.Contains(err.Error(), "过期") {
		t.Fatalf("expired code should be rejected: %v", err)
	}
}

func TestVerifyUserEmailCodeAttemptLimit(t *testing.T) {
	emailCodeLock.Lock()
	emailCodes = map[string]*emailCodeState{}
	emailCodes[emailCodeScope+"|"+EmailCodePurposeRegister+"|email:test-limit@qq.com"] = &emailCodeState{
		Code:      "654321",
		ExpiresAt: time.Now().Add(emailCodeTTL),
		FirstSent: time.Now(),
	}
	emailCodeLock.Unlock()

	// 未达上限前，错误码按普通错误拒绝。
	for i := 1; i < maxEmailCodeVerifyAttempts; i++ {
		if err := verifyUserEmailCode("test-limit@qq.com", "000000", EmailCodePurposeRegister); err == nil {
			t.Fatalf("wrong attempt %d should fail", i)
		}
	}
	// 达到上限的错误尝试后验证码作废。
	if err := verifyUserEmailCode("test-limit@qq.com", "000000", EmailCodePurposeRegister); err == nil || !strings.Contains(err.Error(), "次数过多") {
		t.Fatalf("too many wrong attempts should invalidate the code: %v", err)
	}
	// 作废后正确验证码也不再通过。
	if err := verifyUserEmailCode("test-limit@qq.com", "654321", EmailCodePurposeRegister); err == nil {
		t.Fatal("invalidated code must not verify even with correct value")
	}
	// 作废条目保留至自然过期，同邮箱发码限频窗口不被重置。
	emailCodeLock.Lock()
	_, emailKeyAlive := emailCodes[emailCodeScope+"|"+EmailCodePurposeRegister+"|email:test-limit@qq.com"]
	emailCodeLock.Unlock()
	if !emailKeyAlive {
		t.Fatal("invalidated entry must be kept for the per-email send window")
	}
}

func TestSmtpSettingsRequiresAllFields(t *testing.T) {
	host, _, _, _, ok := smtpSettings()
	_ = host
	// 测试环境未配置 SMTP 时应返回未配置。
	if ok && host == "" {
		t.Fatal("smtpSettings must not report ok with empty host")
	}
}

func TestVerifyUserEmailCodePurposeIsolation(t *testing.T) {
	emailCodeLock.Lock()
	emailCodes = map[string]*emailCodeState{}
	emailCodes[emailCodeScope+"|"+EmailCodePurposeRegister+"|email:iso@qq.com"] = &emailCodeState{
		Code:      "111222",
		ExpiresAt: time.Now().Add(emailCodeTTL),
		FirstSent: time.Now(),
	}
	emailCodeLock.Unlock()

	// 注册用途的验证码不能用于重置密码，反之亦然。
	if err := verifyUserEmailCode("iso@qq.com", "111222", EmailCodePurposePasswordReset); err == nil {
		t.Fatal("register-purpose code must not verify for password reset")
	}
	if err := verifyUserEmailCode("iso@qq.com", "111222", EmailCodePurposeRegister); err != nil {
		t.Fatalf("register-purpose code should verify for register: %v", err)
	}
}
