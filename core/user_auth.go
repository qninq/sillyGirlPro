package core

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/qninq/sillyGirlPro/utils"
	"golang.org/x/crypto/bcrypt"
)

var userBucket = MakeBucket("users")

const userJWTExpireSeconds = 7 * 24 * 60 * 60

var (
	userNamePattern      = regexp.MustCompile(`^[A-Za-z0-9_\-.]{3,32}$`)
	userQQBindingPattern = regexp.MustCompile(`^\d{5,12}$`)
	userTGBindingPattern = regexp.MustCompile(`^-?\d{5,20}$`)
)

type normalUser struct {
	ID           string `json:"id"`
	Username     string `json:"username"`
	Nickname     string `json:"nickname"`
	PasswordHash string `json:"password_hash"`
	CreatedAt    int64  `json:"created_at"`
	UpdatedAt    int64  `json:"updated_at"`
	Disabled     bool   `json:"disabled"`
}

type publicNormalUser struct {
	ID        string `json:"id"`
	Username  string `json:"username"`
	Nickname  string `json:"nickname"`
	CreatedAt int64  `json:"created_at"`
}

type normalUserBindings struct {
	QQ        string `json:"qq"`
	Telegram  string `json:"telegram"`
	UpdatedAt int64  `json:"updated_at"`
}

type adminNormalUserRow struct {
	publicNormalUser
	Bindings   normalUserBindings `json:"bindings"`
	UpdatedAt  int64              `json:"updated_at"`
	Disabled   bool               `json:"disabled"`
	StorageKey string             `json:"storage_key"`
}

type adminNormalUserPayload struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Nickname string `json:"nickname"`
	Disabled *bool  `json:"disabled"`
	QQ       string `json:"qq"`
	Telegram string `json:"telegram"`
}

type userJWTClaims struct {
	Sub string `json:"sub"`
	UID string `json:"uid"`
	Iat int64  `json:"iat"`
	Exp int64  `json:"exp"`
}

func init() {
	GinApi(GET, "/api/admin/users", RequireAuth, func(ctx *gin.Context) {
		rows, err := listNormalUsers()
		if err != nil {
			ApiInternalError(ctx, err.Error())
			return
		}
		ApiOK(ctx, gin.H{
			"list":  rows,
			"total": len(rows),
		})
	})

	GinApi(POST, "/api/admin/users", RequireAuth, func(ctx *gin.Context) {
		payload := adminNormalUserPayload{}
		if err := json.NewDecoder(ctx.Request.Body).Decode(&payload); err != nil {
			ApiFail(ctx, "请求体不是有效 JSON")
			return
		}
		user, err := createNormalUser(payload.Username, payload.Password, payload.Nickname)
		if err != nil {
			if strings.Contains(err.Error(), "已存在") {
				ApiConflict(ctx, err.Error())
			} else {
				ApiUnprocessable(ctx, err.Error())
			}
			return
		}
		bindings, err := replaceNormalUserBindings(user.Username, payload.QQ, payload.Telegram)
		if err != nil {
			_ = deleteNormalUser(user.Username)
			ApiUnprocessable(ctx, err.Error())
			return
		}
		if payload.Disabled != nil && user.Disabled != *payload.Disabled {
			user.Disabled = *payload.Disabled
			user.UpdatedAt = time.Now().Unix()
			if _, _, err := userBucket.Set(normalUserStorageKey(user.Username), utils.JsonMarshal(user)); err != nil {
				_ = deleteNormalUser(user.Username)
				ApiInternalError(ctx, err.Error())
				return
			}
		}
		ApiCreated(ctx, "/api/admin/users/"+url.PathEscape(user.Username), adminNormalUserRowFor(user, bindings))
	})

	GinApi(POST, "/api/admin/users/:username", RequireAuth, func(ctx *gin.Context) {
		payload := adminNormalUserPayload{}
		if err := json.NewDecoder(ctx.Request.Body).Decode(&payload); err != nil {
			ApiFail(ctx, "请求体不是有效 JSON")
			return
		}
		payload.Username = ctx.Param("username")
		user, bindings, err := updateNormalUserByAdmin(payload)
		if err != nil {
			if strings.Contains(err.Error(), "不存在") {
				ApiNotFound(ctx, err.Error())
			} else {
				ApiUnprocessable(ctx, err.Error())
			}
			return
		}
		ApiOK(ctx, adminNormalUserRowFor(user, bindings))
	})

	GinApi(POST, "/api/admin/users/:username/deletions", RequireAuth, func(ctx *gin.Context) {
		username := ctx.Param("username")
		if err := deleteNormalUser(username); err != nil {
			if strings.Contains(err.Error(), "不存在") {
				ApiNotFound(ctx, err.Error())
			} else {
				ApiUnprocessable(ctx, err.Error())
			}
			return
		}
		ApiOK(ctx, nil)
	})

	GinApi(POST, "/api/user/accounts", func(ctx *gin.Context) {
		payload := struct {
			Username string `json:"username"`
			Password string `json:"password"`
			Nickname string `json:"nickname"`
		}{}
		if err := json.NewDecoder(ctx.Request.Body).Decode(&payload); err != nil {
			ApiFail(ctx, "请求体不是有效 JSON")
			return
		}
		user, err := createNormalUser(payload.Username, payload.Password, payload.Nickname)
		if err != nil {
			if strings.Contains(err.Error(), "已存在") {
				ApiConflict(ctx, err.Error())
			} else {
				ApiUnprocessable(ctx, err.Error())
			}
			return
		}
		token, err := createUserJWT(user)
		if err != nil {
			ApiInternalError(ctx, err.Error())
			return
		}
		ApiCreated(ctx, "/api/user/profile", gin.H{
			"token":     token,
			"expiresIn": userJWTExpireSeconds,
			"user":      toPublicNormalUser(user),
		})
	})

	GinApi(POST, "/api/user/sessions", func(ctx *gin.Context) {
		payload := struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}{}
		if err := json.NewDecoder(ctx.Request.Body).Decode(&payload); err != nil {
			ApiFail(ctx, "请求体不是有效 JSON")
			return
		}
		attemptKey := "user:" + normalizeNormalUsername(payload.Username)
		if loginAttemptBlocked(ctx, attemptKey) {
			ApiError(ctx, http.StatusTooManyRequests, "登录失败次数过多，请稍后再试")
			return
		}
		user, err := verifyNormalUser(payload.Username, payload.Password)
		if err != nil {
			recordFailedLoginAttempt(ctx, attemptKey)
			ApiUnauthorized(ctx, "账号或密码错误")
			return
		}
		clearLoginAttempts(ctx, attemptKey)
		token, err := createUserJWT(user)
		if err != nil {
			ApiInternalError(ctx, err.Error())
			return
		}
		ApiCreated(ctx, "/api/user/sessions/current", gin.H{
			"token":     token,
			"expiresIn": userJWTExpireSeconds,
			"user":      toPublicNormalUser(user),
		})
	})

	GinApi(GET, "/api/user/profile", RequireUserAuth, func(ctx *gin.Context) {
		user := currentNormalUser(ctx)
		if user == nil {
			ApiError(ctx, http.StatusUnauthorized, "请先登录")
			return
		}
		announcement := strings.TrimSpace(sillyGirl.GetString("user_announcement"))
		announcementEnabledValue := GetBucketKeyValue(sillyGirl, "user_announcement_enable")
		announcementEnabled := announcementEnabledValue == true || fmt.Sprint(announcementEnabledValue) == "true"
		ApiOK(ctx, gin.H{
			"user":     toPublicNormalUser(user),
			"bindings": loadNormalUserBindings(user.Username),
			"announcement": gin.H{
				"enabled": announcementEnabled,
				"content": announcement,
				"format":  normalizeUserAnnouncementFormat(sillyGirl.GetString("user_announcement_format")),
			},
		})
	})

	GinApi(POST, "/api/user/bindings/:platform", RequireUserAuth, func(ctx *gin.Context) {
		user := currentNormalUser(ctx)
		if user == nil {
			ApiError(ctx, http.StatusUnauthorized, "请先登录")
			return
		}
		payload := struct {
			Value string `json:"value"`
		}{}
		if err := json.NewDecoder(ctx.Request.Body).Decode(&payload); err != nil {
			ApiFail(ctx, "请求体不是有效 JSON")
			return
		}
		platform := ctx.Param("platform")
		if !isPublicUserBindingPlatform(platform) {
			ApiUnprocessable(ctx, "普通用户只能绑定 QQ 或 Telegram")
			return
		}
		bindings, err := updateNormalUserBinding(user.Username, platform, payload.Value)
		if err != nil {
			ApiUnprocessable(ctx, err.Error())
			return
		}
		ApiOK(ctx, bindings)
	})

	GinApi(POST, "/api/user/bindings/:platform/deletions", RequireUserAuth, func(ctx *gin.Context) {
		user := currentNormalUser(ctx)
		if user == nil {
			ApiError(ctx, http.StatusUnauthorized, "请先登录")
			return
		}
		platform := ctx.Param("platform")
		if !isPublicUserBindingPlatform(platform) {
			ApiUnprocessable(ctx, "普通用户只能解绑 QQ 或 Telegram")
			return
		}
		bindings, err := updateNormalUserBinding(user.Username, platform, "")
		if err != nil {
			ApiUnprocessable(ctx, err.Error())
			return
		}
		ApiOK(ctx, bindings)
	})

	GinApi(POST, "/api/user/sessions/current/deletions", RequireUserAuth, func(ctx *gin.Context) {
		ApiOK(ctx, nil)
	})
}

func createNormalUser(username string, password string, nickname string) (*normalUser, error) {
	username = normalizeNormalUsername(username)
	nickname = strings.TrimSpace(nickname)
	if err := validateNormalUsername(username); err != nil {
		return nil, err
	}
	if err := validateNormalPassword(password); err != nil {
		return nil, err
	}
	if existing, _ := loadNormalUser(username); existing != nil {
		return nil, errors.New("账号已存在")
	}
	if nickname == "" {
		nickname = username
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), adminPasswordHashCost)
	if err != nil {
		return nil, err
	}
	now := time.Now().Unix()
	user := &normalUser{
		ID:           utils.GenUUID(),
		Username:     username,
		Nickname:     nickname,
		PasswordHash: string(hash),
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if _, _, err := userBucket.Set(normalUserStorageKey(username), utils.JsonMarshal(user)); err != nil {
		return nil, err
	}
	return user, nil
}

func updateNormalUserByAdmin(payload adminNormalUserPayload) (*normalUser, normalUserBindings, error) {
	username := normalizeNormalUsername(payload.Username)
	if err := validateNormalUsername(username); err != nil {
		return nil, normalUserBindings{}, err
	}
	user, err := loadNormalUser(username)
	if err != nil {
		return nil, normalUserBindings{}, err
	}
	bindings, err := normalizedReplacementBindings(payload.QQ, payload.Telegram)
	if err != nil {
		return nil, normalUserBindings{}, err
	}
	nickname := strings.TrimSpace(payload.Nickname)
	if nickname == "" {
		nickname = user.Username
	}
	if len([]rune(nickname)) > 64 {
		return nil, normalUserBindings{}, errors.New("昵称不能超过 64 位")
	}
	passwordHash := user.PasswordHash
	if payload.Password != "" {
		if err := validateNormalPassword(payload.Password); err != nil {
			return nil, normalUserBindings{}, err
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(payload.Password), adminPasswordHashCost)
		if err != nil {
			return nil, normalUserBindings{}, err
		}
		passwordHash = string(hash)
	}
	user.Nickname = nickname
	user.PasswordHash = passwordHash
	if payload.Disabled != nil {
		user.Disabled = *payload.Disabled
	}
	user.UpdatedAt = time.Now().Unix()
	bindings.UpdatedAt = user.UpdatedAt
	if _, _, err := userBucket.Set(normalUserStorageKey(username), utils.JsonMarshal(user)); err != nil {
		return nil, normalUserBindings{}, err
	}
	if _, _, err := userBucket.Set(normalUserBindingsStorageKey(username), utils.JsonMarshal(bindings)); err != nil {
		return nil, normalUserBindings{}, err
	}
	return user, bindings, nil
}

func replaceNormalUserBindings(username, qq, telegram string) (normalUserBindings, error) {
	if _, err := loadNormalUser(username); err != nil {
		return normalUserBindings{}, err
	}
	bindings, err := normalizedReplacementBindings(qq, telegram)
	if err != nil {
		return normalUserBindings{}, err
	}
	bindings.UpdatedAt = time.Now().Unix()
	if _, _, err := userBucket.Set(normalUserBindingsStorageKey(username), utils.JsonMarshal(bindings)); err != nil {
		return normalUserBindings{}, err
	}
	return bindings, nil
}

func normalizedReplacementBindings(qq, telegram string) (normalUserBindings, error) {
	bindings := normalUserBindings{
		QQ:       strings.TrimSpace(qq),
		Telegram: strings.TrimSpace(telegram),
	}
	if bindings.QQ != "" && !userQQBindingPattern.MatchString(bindings.QQ) {
		return normalUserBindings{}, errors.New("QQ 号格式不正确")
	}
	if bindings.Telegram != "" && !userTGBindingPattern.MatchString(bindings.Telegram) {
		return normalUserBindings{}, errors.New("Telegram ID 格式不正确")
	}
	return normalizeNormalUserBindings(bindings), nil
}

func deleteNormalUser(username string) error {
	username = normalizeNormalUsername(username)
	if err := validateNormalUsername(username); err != nil {
		return err
	}
	user, err := loadNormalUser(username)
	if err != nil {
		return err
	}
	if err := deletePluginUserRecordsForUser(user.ID); err != nil {
		return err
	}
	if _, _, err := userBucket.Set(normalUserBindingsStorageKey(username), ""); err != nil {
		return err
	}
	if _, _, err := userBucket.Set(normalUserStorageKey(username), ""); err != nil {
		return err
	}
	return nil
}

func adminNormalUserRowFor(user *normalUser, bindings normalUserBindings) adminNormalUserRow {
	return adminNormalUserRow{
		publicNormalUser: toPublicNormalUser(user),
		Bindings:         normalizeNormalUserBindings(bindings),
		UpdatedAt:        user.UpdatedAt,
		Disabled:         user.Disabled,
		StorageKey:       normalUserStorageKey(user.Username),
	}
}

func verifyNormalUser(username string, password string) (*normalUser, error) {
	username = normalizeNormalUsername(username)
	user, err := loadNormalUser(username)
	if err != nil {
		return nil, errors.New("账号或密码错误")
	}
	if user.Disabled {
		return nil, errors.New("账号已禁用")
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		return nil, errors.New("账号或密码错误")
	}
	return user, nil
}

func loadNormalUser(username string) (*normalUser, error) {
	raw := strings.TrimSpace(userBucket.GetString(normalUserStorageKey(username)))
	if raw == "" {
		return nil, errors.New("账号不存在")
	}
	user := &normalUser{}
	if err := json.Unmarshal([]byte(raw), user); err != nil {
		return nil, err
	}
	if user.Username == "" || user.ID == "" {
		return nil, errors.New("账号数据无效")
	}
	return user, nil
}

func RequireUserAuth(ctx *gin.Context) {
	token := userAuthTokenFromRequest(ctx)
	claims, err := parseUserJWT(token)
	if err != nil {
		ApiError(ctx, http.StatusUnauthorized, err.Error())
		ctx.Abort()
		return
	}
	user, err := loadNormalUser(claims.Sub)
	if err != nil || user.ID != claims.UID || user.Disabled {
		ApiError(ctx, http.StatusUnauthorized, "登录已失效")
		ctx.Abort()
		return
	}
	ctx.Set("normal_user", user)
}

func createUserJWT(user *normalUser) (string, error) {
	now := time.Now().Unix()
	token, err := signUserJWT(userJWTClaims{
		Sub: user.Username,
		UID: user.ID,
		Iat: now,
		Exp: now + userJWTExpireSeconds,
	})
	if err != nil {
		return "", err
	}
	return token, nil
}

func signUserJWT(claims userJWTClaims) (string, error) {
	header, err := json.Marshal(map[string]string{
		"alg": "HS256",
		"typ": "JWT",
	})
	if err != nil {
		return "", err
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	unsigned := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload)
	return unsigned + "." + signUserJWTPart(unsigned), nil
}

func parseUserJWT(token string) (*userJWTClaims, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, errors.New("请先登录")
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, errors.New("JWT 格式错误")
	}
	unsigned := parts[0] + "." + parts[1]
	if !hmac.Equal([]byte(parts[2]), []byte(signUserJWTPart(unsigned))) {
		return nil, errors.New("JWT 签名无效")
	}
	headerRaw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, errors.New("JWT 头解析失败")
	}
	header := map[string]string{}
	if err := json.Unmarshal(headerRaw, &header); err != nil || header["alg"] != "HS256" {
		return nil, errors.New("JWT 算法不支持")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, errors.New("JWT 内容解析失败")
	}
	claims := &userJWTClaims{}
	if err := json.Unmarshal(payload, claims); err != nil {
		return nil, errors.New("JWT 内容无效")
	}
	if claims.Sub == "" || claims.UID == "" {
		return nil, errors.New("JWT 缺少用户信息")
	}
	if jwtClaimsExpired(time.Now().Unix(), claims.Iat, claims.Exp, userJWTExpireSeconds) {
		return nil, errors.New("JWT 已过期")
	}
	return claims, nil
}

func signUserJWTPart(unsigned string) string {
	mac := hmac.New(sha256.New, userJWTSecret())
	mac.Write([]byte(unsigned))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func userJWTSecret() []byte {
	sum := sha256.Sum256([]byte(GetMachineID() + "|sillyGirl-normal-user-jwt"))
	return sum[:]
}

func userAuthTokenFromRequest(ctx *gin.Context) string {
	return strings.TrimSpace(ctx.GetHeader("token"))
}

func validateNormalUsername(username string) error {
	if !userNamePattern.MatchString(username) {
		return errors.New("账号只能包含 3-32 位字母、数字、下划线、横线或点")
	}
	return nil
}

func validateNormalPassword(password string) error {
	if len([]rune(password)) < 6 {
		return errors.New("密码至少 6 位")
	}
	if len([]rune(password)) > 128 {
		return errors.New("密码不能超过 128 位")
	}
	return nil
}

func normalizeNormalUsername(username string) string {
	return strings.TrimSpace(username)
}

func normalUserStorageKey(username string) string {
	return "user:" + strings.ToLower(strings.TrimSpace(username))
}

func normalUserBindingsStorageKey(username string) string {
	return "bindings:" + strings.ToLower(strings.TrimSpace(username))
}

func currentNormalUser(ctx *gin.Context) *normalUser {
	value, ok := ctx.Get("normal_user")
	if !ok {
		return nil
	}
	user, _ := value.(*normalUser)
	return user
}

func loadNormalUserBindings(username string) normalUserBindings {
	raw := strings.TrimSpace(userBucket.GetString(normalUserBindingsStorageKey(username)))
	if raw == "" {
		return normalUserBindings{}
	}
	bindings := normalUserBindings{}
	if json.Unmarshal([]byte(raw), &bindings) != nil {
		return normalUserBindings{}
	}
	return normalizeNormalUserBindings(bindings)
}

func updateNormalUserBinding(username string, platform string, value string) (normalUserBindings, error) {
	platform = strings.ToLower(strings.TrimSpace(platform))
	value = strings.TrimSpace(value)
	bindings := loadNormalUserBindings(username)
	switch platform {
	case "qq":
		if value != "" && !userQQBindingPattern.MatchString(value) {
			return bindings, errors.New("QQ 号格式不正确")
		}
		bindings.QQ = value
	case "telegram", "tg", "tgid":
		if value != "" && !userTGBindingPattern.MatchString(value) {
			return bindings, errors.New("Telegram ID 格式不正确")
		}
		bindings.Telegram = value
	default:
		return bindings, errors.New("不支持的绑定类型")
	}
	bindings = normalizeNormalUserBindings(bindings)
	bindings.UpdatedAt = time.Now().Unix()
	if _, _, err := userBucket.Set(normalUserBindingsStorageKey(username), utils.JsonMarshal(bindings)); err != nil {
		return bindings, err
	}
	return bindings, nil
}

func isPublicUserBindingPlatform(platform string) bool {
	switch strings.ToLower(strings.TrimSpace(platform)) {
	case "qq", "telegram", "tg", "tgid":
		return true
	default:
		return false
	}
}

func normalizeNormalUserBindings(bindings normalUserBindings) normalUserBindings {
	bindings.QQ = strings.TrimSpace(bindings.QQ)
	bindings.Telegram = strings.TrimSpace(bindings.Telegram)
	return bindings
}

func listNormalUsers() ([]adminNormalUserRow, error) {
	rows := []adminNormalUserRow{}
	var firstErr error
	userBucket.Foreach(func(keyBytes, valueBytes []byte) error {
		key := string(keyBytes)
		if !strings.HasPrefix(key, "user:") {
			return nil
		}
		user := &normalUser{}
		if err := json.Unmarshal(valueBytes, user); err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("用户数据解析失败：%s", key)
			}
			return nil
		}
		if user.Username == "" || user.ID == "" {
			return nil
		}
		rows = append(rows, adminNormalUserRow{
			publicNormalUser: toPublicNormalUser(user),
			Bindings:         loadNormalUserBindings(user.Username),
			UpdatedAt:        user.UpdatedAt,
			Disabled:         user.Disabled,
			StorageKey:       key,
		})
		return nil
	})
	if firstErr != nil {
		return nil, firstErr
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].CreatedAt == rows[j].CreatedAt {
			return rows[i].Username < rows[j].Username
		}
		return rows[i].CreatedAt > rows[j].CreatedAt
	})
	return rows, nil
}

func normalizeUserAnnouncementFormat(format string) string {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "html":
		return "html"
	case "md", "markdown":
		return "markdown"
	default:
		return "text"
	}
}

func toPublicNormalUser(user *normalUser) publicNormalUser {
	return publicNormalUser{
		ID:        user.ID,
		Username:  user.Username,
		Nickname:  user.Nickname,
		CreatedAt: user.CreatedAt,
	}
}
