package core

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/goccy/go-json"
	"github.com/qninq/sillyGirlPro/utils"
)

// bindCode 表示一条手动生成、有效 5 分钟、一次性使用的渠道绑定码。
// 用户在用户中心生成绑定码后，到对应渠道（Telegram / QQ 频道）发送
// 「绑定 <code>」，系统将该渠道发送者的身份自动关联到生成此码的账号。
type bindCode struct {
	Code      string `json:"code"`
	Username  string `json:"username"`
	CreatedAt int64  `json:"created_at"`
	ExpiresAt int64  `json:"expires_at"`
}

var bindCodesBucket = MakeBucket("bindCodes")

// bindCodeLifetime 绑定码有效时长：5 分钟。
const bindCodeLifetime = 5 * 60

func bindCodeStorageKey(code string) string {
	return strings.TrimSpace(code)
}

func loadBindCode(code string) (bindCode, error) {
	bc := bindCode{}
	raw := strings.TrimSpace(bindCodesBucket.GetString(bindCodeStorageKey(code)))
	if raw == "" {
		return bc, errors.New("绑定码无效")
	}
	if json.Unmarshal([]byte(raw), &bc) != nil {
		return bc, errors.New("绑定码无效")
	}
	return bc, nil
}

func deleteBindCode(code string) {
	bindCodesBucket.Set(bindCodeStorageKey(code), "")
}

// randomBindCode 用密码学随机源生成 6 位数字绑定码，避免时间戳取模带来的可预测性。
func randomBindCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

// scanBindCodes 遍历绑定码桶：顺手清理已过期的残留码，并返回该账号
// 尚未过期、可复用的绑定码。
func scanBindCodes(username string, now int64) (bindCode, bool) {
	username = strings.TrimSpace(username)
	var reusable bindCode
	bindCodesBucket.Foreach(func(key, value []byte) error {
		bc := bindCode{}
		if json.Unmarshal(value, &bc) != nil {
			return nil
		}
		if bc.ExpiresAt <= now {
			deleteBindCode(bc.Code)
			return nil
		}
		if reusable.Code == "" && bc.Username == username {
			reusable = bc
		}
		return nil
	})
	return reusable, reusable.Code != ""
}

// generateBindCode 为指定账号生成一个未使用的 6 位数字绑定码并写入存储，
// 返回绑定码与过期时间戳（Unix 秒）。已有未过期绑定码时优先复用，
// 复用时返回该码真实的过期时间。
func generateBindCode(username string) (string, int64, error) {
	if strings.TrimSpace(username) == "" {
		return "", 0, errors.New("缺少账号")
	}
	now := time.Now().Unix()
	if bc, ok := scanBindCodes(username, now); ok {
		return bc.Code, bc.ExpiresAt, nil
	}
	for i := 0; i < 20; i++ {
		code, err := randomBindCode()
		if err != nil {
			return "", 0, err
		}
		if _, err := loadBindCode(code); err != nil {
			bc := bindCode{
				Code:      code,
				Username:  strings.TrimSpace(username),
				CreatedAt: now,
				ExpiresAt: now + bindCodeLifetime,
			}
			if _, _, err := bindCodesBucket.Set(bindCodeStorageKey(code), utils.JsonMarshal(bc)); err != nil {
				return "", 0, err
			}
			return code, bc.ExpiresAt, nil
		}
	}
	return "", 0, errors.New("绑定码生成失败，请重试")
}

// consumeBindCode 校验并消费一条绑定码：校验存在、未过期后立即删除（一次性），
// 返回该绑定码所属账号。任何失败均返回对应错误提示，绑定码过期即失效。
func consumeBindCode(code string) (string, error) {
	bc, err := loadBindCode(strings.TrimSpace(code))
	if err != nil {
		return "", err
	}
	if bc.Username == "" {
		return "", errors.New("绑定码无效")
	}
	if time.Now().Unix() > bc.ExpiresAt {
		deleteBindCode(bc.Code)
		return "", errors.New("绑定码已过期")
	}
	deleteBindCode(bc.Code)
	return bc.Username, nil
}
