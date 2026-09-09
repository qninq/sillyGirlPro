package core

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/goccy/go-json"
)

type adminPanelList[T any] struct {
	List  []T `json:"list"`
	Total int `json:"total"`
}

type adminPanelsResponse struct {
	Qinglong adminPanelList[QinglongPanel] `json:"qinglong"`
	Daidai   adminPanelList[DaidaiPanel]   `json:"daidai"`
}

func init() {
	GinApi(GET, "/api/admin/panels", RequireAuth, func(ctx *gin.Context) {
		ApiOK(ctx, getAdminPanels(false))
	})
	GinApi(POST, "/api/admin/panels", RequireAuth, handleSaveAdminPanel)
	GinApi(POST, "/api/admin/panels/:id", RequireAuth, handleSaveAdminPanel)
	GinApi(POST, "/api/admin/panels/:id/deletions", RequireAuth, handleDeleteAdminPanel)
	GinApi(POST, "/api/admin/panel-connection-tests", RequireAuth, handleAdminPanelConnectionTest)
	GinApi(POST, "/api/admin/panel-status-checks", RequireAuth, func(ctx *gin.Context) {
		ApiOK(ctx, getAdminPanels(true))
	})
}

func handleSaveAdminPanel(ctx *gin.Context) {
	kind, err := adminPanelKindFromRequest(ctx, strings.TrimSpace(ctx.Param("id")))
	if err != nil {
		respondAdminPanelKindError(ctx, err)
		return
	}
	switch kind {
	case "qinglong":
		handleSaveQinglongPanel(ctx)
	case "daidai":
		handleSaveDaidaiPanel(ctx)
	default:
		ApiUnprocessable(ctx, "面板类型必须是 qinglong 或 daidai")
	}
}

func handleAdminPanelConnectionTest(ctx *gin.Context) {
	kind, err := adminPanelKindFromRequest(ctx, "")
	if err != nil {
		respondAdminPanelKindError(ctx, err)
		return
	}
	switch kind {
	case "qinglong":
		handleQinglongPanelConnectionTest(ctx)
	case "daidai":
		handleDaidaiPanelConnectionTest(ctx)
	default:
		ApiUnprocessable(ctx, "面板类型必须是 qinglong 或 daidai")
	}
}

func respondAdminPanelKindError(ctx *gin.Context, err error) {
	if strings.Contains(err.Error(), "JSON") || strings.Contains(err.Error(), "1 MiB") {
		ApiFail(ctx, err.Error())
		return
	}
	ApiUnprocessable(ctx, err.Error())
}

func handleDeleteAdminPanel(ctx *gin.Context) {
	id := strings.TrimSpace(ctx.Param("id"))
	if id == "" {
		ApiFail(ctx, "缺少面板 ID")
		return
	}
	deleted := deleteQinglongPanel(id) || deleteDaidaiPanel(id)
	if !deleted {
		ApiNotFound(ctx, "面板不存在")
		return
	}
	ApiOK(ctx, nil)
}

func adminPanelKindFromRequest(ctx *gin.Context, id string) (string, error) {
	data, err := io.ReadAll(http.MaxBytesReader(ctx.Writer, ctx.Request.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("请求体无效或超过 1 MiB")
	}
	ctx.Request.Body = io.NopCloser(bytes.NewReader(data))
	payload := struct {
		Type string `json:"type"`
		Kind string `json:"kind"`
	}{}
	if len(bytes.TrimSpace(data)) != 0 {
		if err := json.Unmarshal(data, &payload); err != nil {
			return "", fmt.Errorf("请求体不是有效 JSON")
		}
	}
	kind := strings.ToLower(strings.TrimSpace(firstNonEmpty(payload.Type, payload.Kind)))
	if kind == "" && id != "" {
		kind = adminPanelKindByID(id)
	}
	if kind == "" {
		return "", fmt.Errorf("缺少面板类型 type")
	}
	ctx.Request.Body = io.NopCloser(bytes.NewReader(data))
	return kind, nil
}

func adminPanelKindByID(id string) string {
	for _, panel := range getQinglongPanels() {
		if panel.ID == id {
			return "qinglong"
		}
	}
	for _, panel := range getDaidaiPanels() {
		if panel.ID == id {
			return "daidai"
		}
	}
	return ""
}

func refreshQinglongPanelsStatus(panels []QinglongPanel) {
	refreshPanelsStatus(len(panels), func(index int) {
		if updated, err := testQinglongPanel(panels[index]); err != nil {
			panels[index].Status = "offline"
			panels[index].Message = err.Error()
			panels[index].LastCheckedAt = int(time.Now().Unix())
		} else if updated != nil {
			panels[index] = *updated
		}
	})
}

func refreshDaidaiPanelsStatus(panels []DaidaiPanel) {
	refreshPanelsStatus(len(panels), func(index int) {
		if updated, err := testDaidaiPanel(panels[index]); err != nil {
			panels[index].Status = "offline"
			panels[index].Message = err.Error()
			panels[index].LastCheckedAt = int(time.Now().Unix())
		} else if updated != nil {
			panels[index] = *updated
		}
	})
}

func refreshPanelsStatus(count int, check func(index int)) {
	var wg sync.WaitGroup
	for index := 0; index < count; index++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			check(i)
		}(index)
	}
	wg.Wait()
}

func getAdminPanels(refresh bool) adminPanelsResponse {
	if refresh {
		qinglongPanels := getQinglongPanels()
		daidaiPanels := getDaidaiPanels()
		refreshQinglongPanelsStatus(qinglongPanels)
		refreshDaidaiPanelsStatus(daidaiPanels)
		return buildAdminPanelsResponse(qinglongPanels, daidaiPanels)
	}
	return buildAdminPanelsResponse(getQinglongPanels(), getDaidaiPanels())
}

func buildAdminPanelsResponse(
	qinglongPanels []QinglongPanel,
	daidaiPanels []DaidaiPanel,
) adminPanelsResponse {
	return adminPanelsResponse{
		Qinglong: adminPanelList[QinglongPanel]{
			List:  qinglongPanels,
			Total: len(qinglongPanels),
		},
		Daidai: adminPanelList[DaidaiPanel]{
			List:  daidaiPanels,
			Total: len(daidaiPanels),
		},
	}
}
