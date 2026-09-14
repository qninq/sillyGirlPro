package core

import (
	"fmt"

	"github.com/qninq/sillyGirlPro/core/common"
)

func init() {
	AddCommand([]*common.Function{
		{
			Rules:    []string{`绑定 [code]`},
			Hidden:   true,
			Title:    "绑定",
			Priority: 8000,
			Handle: func(s common.Sender) interface{} {
				code := s.Get(0)
				imType := s.GetImType()
				uid := s.GetUserID()

				platform, ok := bindCodePlatform(imType)
				if !ok {
					return fmt.Sprintf("不支持在 %s 渠道使用绑定码，请到 Telegram 或 QQ 频道发送「绑定 <code>」", imType)
				}

				username, err := consumeBindCode(code)
				if err != nil {
					return err.Error()
				}

				if _, err := updateNormalUserBinding(username, platform, uid); err != nil {
					return err.Error()
				}
				return bindSuccessReply(username, platform, uid)
			},
		},
	})
}

// bindCodePlatform 将渠道类型映射到用户中心绑定平台键，仅支持 Telegram 与 QQ 频道。
func bindCodePlatform(imType string) (string, bool) {
	switch imType {
	case "telegram":
		return "telegram", true
	case "qqguild":
		return "qqguild", true
	default:
		return "", false
	}
}

// bindPlatformLabel 渠道在绑定成功回复中的展示名。
func bindPlatformLabel(platform string) string {
	switch platform {
	case "telegram":
		return "TG"
	default:
		return platform
	}
}

// bindSuccessReply 绑定成功的回复：账号不带 @，逐行列出账号与刚绑定的渠道身份。
func bindSuccessReply(username, platform, uid string) string {
	return fmt.Sprintf("绑定成功：\n账号：%s\n%s 账号: %s", username, bindPlatformLabel(platform), uid)
}
