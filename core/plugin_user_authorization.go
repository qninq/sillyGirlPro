package core

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/qninq/sillyGirlPro/utils"
)

const (
	pluginUserRuntimeBucket  = "__plugin_users__"
	pluginUserRuntimeListKey = "list"
)

func isPluginRuntimeBucket(name string) bool {
	return name == pluginUserRuntimeBucket
}

func init() {
	GinApi(GET, "/api/user/plugins", RequireUserAuth, func(ctx *gin.Context) {
		user := currentNormalUser(ctx)
		if user == nil {
			ApiError(ctx, http.StatusUnauthorized, "请先登录")
			return
		}
		ApiOK(ctx, openPluginRecords(Functions))
	})
}

type pluginRuntimeUserBindings struct {
	QQ       string `json:"qq"`
	Telegram string `json:"telegram"`
}

// pluginRuntimeUser is the public, read-only user view exposed to a running
// plugin. Password hashes, storage keys and internal timestamps never cross the
// runtime boundary. Authorized always refers to the current plugin's user-form
// access grant; plugins cannot query another plugin's authorization.
type pluginRuntimeUser struct {
	ID         string                    `json:"id"`
	Username   string                    `json:"username"`
	Nickname   string                    `json:"nickname"`
	Disabled   bool                      `json:"disabled"`
	Authorized bool                      `json:"authorized"`
	Bindings   pluginRuntimeUserBindings `json:"bindings"`
	Records    []pluginUserFormRecord    `json:"records,omitempty"`
}

func pluginRuntimeUsers(pluginID string) []pluginRuntimeUser {
	pluginID = strings.TrimSpace(pluginID)
	plugin := installedPluginByUUID(pluginID)
	if pluginID == "" || plugin == nil {
		return []pluginRuntimeUser{}
	}
	grantEnabled := plugin.Open && pluginExecutionEnabled(plugin) && plugin.HasUserForm
	rows, err := listNormalUsers()
	if err != nil {
		return []pluginRuntimeUser{}
	}
	result := make([]pluginRuntimeUser, 0, len(rows))
	for _, row := range rows {
		authorized := grantEnabled && !row.Disabled
		result = append(result, pluginRuntimeUser{
			ID:         row.ID,
			Username:   row.Username,
			Nickname:   row.Nickname,
			Disabled:   row.Disabled,
			Authorized: authorized,
			Bindings: pluginRuntimeUserBindings{
				QQ:       row.Bindings.QQ,
				Telegram: row.Bindings.Telegram,
			},
			Records: pluginUserRecords(row.ID, pluginID),
		})
	}
	return result
}

func pluginUserRuntimeValue(pluginID string, key string) string {
	if strings.TrimSpace(key) != pluginUserRuntimeListKey {
		return ""
	}
	return "o:" + string(utils.JsonMarshal(pluginRuntimeUsers(pluginID)))
}
