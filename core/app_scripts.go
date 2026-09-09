package core

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/qninq/sillyGirlPro/core/common"
	"github.com/qninq/sillyGirlPro/utils"
)

// 应用脚本语言分类。仅 JS/Python 系可执行，其余为占位分类。
const (
	appScriptLangES5           = "es5"
	appScriptLangNode          = "node"
	appScriptLangTypeScript    = "typescript"
	appScriptLangPython        = "python"
	appScriptLangGolang        = "golang"
	appScriptLangAdapterJS     = "adapter-js"
	appScriptLangAdapterPython = "adapter-python"
	appScriptLangAdapterGo     = "adapter-go"

	appScriptFileIDPrefix = "file:"
)

var appScriptLanguages = []string{
	appScriptLangES5,
	appScriptLangNode,
	appScriptLangTypeScript,
	appScriptLangPython,
	appScriptLangGolang,
	appScriptLangAdapterJS,
	appScriptLangAdapterPython,
	appScriptLangAdapterGo,
}

func appScriptLanguageExecutable(language string) bool {
	switch language {
	case appScriptLangES5, appScriptLangNode, appScriptLangPython, appScriptLangAdapterJS, appScriptLangAdapterPython:
		return true
	}
	return false
}

type appScriptItem struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Name        string `json:"name"`
	Language    string `json:"language"`
	Executable  bool   `json:"executable"`
	File        string `json:"file"`
	Desc        string `json:"desc"`
	Version     string `json:"version"`
	Status      bool   `json:"status"`
	OnStart     bool   `json:"on_start"`
	Web         bool   `json:"web"`
	HasCron     bool   `json:"has_cron"`
	HasForm     bool   `json:"has_form"`
	Installed   bool   `json:"installed"`
	CreateAt    string `json:"create_at"`
}

type appScriptRequest struct {
	Name     string `json:"name"`
	Language string `json:"language"`
	Content  string `json:"content"`
}

var appScriptDebugLocks sync.Map // script id -> struct{}{}

func initAppScripts() {
	GinApi(GET, "/api/admin/app-scripts", RequireAuth, listAppScripts)
	GinApi(POST, "/api/admin/app-scripts", RequireAuth, createAppScript)
	GinApi(GET, "/api/admin/app-scripts/:id", RequireAuth, getAppScript)
	GinApi(POST, "/api/admin/app-scripts/:id", RequireAuth, saveAppScript)
	GinApi(POST, "/api/admin/app-scripts/:id/deletions", RequireAuth, deleteAppScript)
	GinApi(GET, "/api/admin/app-scripts/:id/executions", RequireAuth, runAppScriptDebug)
}

// listAppScripts 遍历插件目录，按语言分类返回本地应用脚本
// （无 [rule]/[module]、带 on_start/web/cron 的脚本，以及全部 .ts/.go 文件）。
func listAppScripts(ctx *gin.Context) {
	ApiOK(ctx, gin.H{"items": collectAppScriptItems()})
}

func collectAppScriptItems() []*appScriptItem {
	root := nodePluginsRoot()
	installed := installedPluginSnapshot()
	byPath := map[string]*common.Function{}
	for _, f := range installed {
		if f == nil || f.Path == "" {
			continue
		}
		byPath[filepath.Clean(f.Path)] = f
	}
	items := []*appScriptItem{}
	for _, entry := range discoverAppScriptFiles(root) {
		path := filepath.Clean(filepath.Join(root, entry))
		if item := buildAppScriptItem(path, byPath[path]); item != nil {
			items = append(items, item)
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Language != items[j].Language {
			return items[i].Language < items[j].Language
		}
		return items[i].Name < items[j].Name
	})
	return items
}

// discoverAppScriptFiles 返回插件根目录下 2 层内的 .js/.py/.ts/.go 相对路径。
func discoverAppScriptFiles(root string) []string {
	files, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	out := []string{}
	appendFile := func(rel string, entry os.DirEntry) {
		if entry.IsDir() || shouldIgnoreNodePluginEntry(entry.Name()) {
			return
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		switch ext {
		case ".js", ".py", ".ts", ".go":
		default:
			return
		}
		if ext == ".js" || ext == ".py" {
			if _, err := checkedNodeScriptPath(filepath.Join(root, rel)); err != nil {
				return
			}
		}
		out = append(out, rel)
	}
	for _, file := range files {
		if shouldIgnoreNodePluginEntry(file.Name()) {
			continue
		}
		if !file.IsDir() {
			appendFile(file.Name(), file)
			continue
		}
		children, err := os.ReadDir(filepath.Join(root, file.Name()))
		if err != nil {
			continue
		}
		for _, child := range children {
			appendFile(filepath.Join(file.Name(), child.Name()), child)
		}
	}
	sort.Strings(out)
	return out
}

// buildAppScriptItem 依据已加载插件或源码元数据判定是否为应用脚本，并做语言分类。
func buildAppScriptItem(path string, loaded *common.Function) *appScriptItem {
	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".js" && ext != ".py" && ext != ".ts" && ext != ".go" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	content := string(data)
	rel, err := filepath.Rel(nodePluginsRoot(), path)
	if err != nil {
		return nil
	}
	item := &appScriptItem{
		File:     filepath.ToSlash(rel),
		Language: classifyAppScriptLanguage(ext, content),
	}
	item.Executable = appScriptLanguageExecutable(item.Language)
	meta := pluginMetaMap(content)
	item.Title = firstNonEmpty(meta["title"], strings.TrimSuffix(filepath.Base(path), ext))
	item.Name = firstNonEmpty(meta["name"], strings.TrimSuffix(filepath.Base(path), ext))
	item.Desc = meta["desc"]
	item.Version = meta["version"]
	if loaded != nil {
		item.ID = loaded.UUID
		item.Installed = true
		item.Status = loaded.Status == nil || *loaded.Status
		item.OnStart = loaded.OnStart
		item.Web = loaded.Web
		item.HasCron = len(loaded.Cron) > 0
		item.HasForm = loaded.HasForm
		item.CreateAt = loaded.CreateAt
		item.Title = firstNonEmpty(loaded.Title, item.Title)
	} else {
		item.Status = parsePluginBoolDefault(meta["status"], true)
		item.OnStart = parsePluginBool(meta["on_start"])
		item.Web = parsePluginBool(meta["web"])
		item.HasCron = strings.TrimSpace(meta["cron"]) != ""
		item.HasForm = strings.TrimSpace(meta["form"]) != "" || strings.Contains(content, "new plugin.Form(") || strings.Contains(content, "plugin.Form(")
		item.ID = appScriptFileIDPrefix + filepath.ToSlash(rel)
	}
	// JS/Python 与 TS/Go 一样全部展示，供「插件开发」页编辑；不再限定
	// 启动/常驻/定时用途（此前带 rule 的响应消息插件被静默过滤，导致
	// 本地插件在页面里看不到）。
	return item
}

// classifyAppScriptLanguage 按扩展名与源码内容启发式分类。
func classifyAppScriptLanguage(ext, content string) string {
	lower := strings.ToLower(content)
	switch ext {
	case ".go":
		if strings.Contains(lower, "adapter") {
			return appScriptLangAdapterGo
		}
		return appScriptLangGolang
	case ".ts":
		return appScriptLangTypeScript
	case ".py":
		if strings.Contains(content, "getAdapter(") || strings.Contains(content, "Adapter(") {
			return appScriptLangAdapterPython
		}
		return appScriptLangPython
	default: // .js
		if strings.Contains(content, "new Adapter(") || strings.Contains(content, "getAdapter(") {
			return appScriptLangAdapterJS
		}
		if strings.Contains(content, "require(") || strings.Contains(content, "import ") {
			return appScriptLangNode
		}
		return appScriptLangES5
	}
}

// createAppScript 新建应用脚本：JS/Python 系走本地插件创建链路，其余仅落盘。
func createAppScript(ctx *gin.Context) {
	req := appScriptRequest{}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ApiFail(ctx, err.Error())
		return
	}
	language := strings.ToLower(strings.TrimSpace(req.Language))
	if !Contains(appScriptLanguages, language) {
		ApiUnprocessable(ctx, "不支持的语言分类："+firstNonEmpty(req.Language, "(空)"))
		return
	}
	if !appScriptLanguageExecutable(language) {
		item, err := writeNewPlaceholderAppScript(req.Name, language, req.Content)
		if err != nil {
			if strings.Contains(err.Error(), "存在") {
				ApiConflict(ctx, err.Error())
			} else {
				ApiUnprocessable(ctx, err.Error())
			}
			return
		}
		ApiCreated(ctx, "/api/admin/app-scripts/"+item.ID, item)
		return
	}
	content, err := validateAndNormalizeLocalPluginContent(req.Content)
	if err != nil {
		ApiUnprocessable(ctx, err.Error())
		return
	}
	class := NODE
	if language == appScriptLangPython || language == appScriptLangAdapterPython {
		class = PYTHON
	}
	if err := validateLocalPluginRequestName(req.Name, content, class); err != nil {
		ApiUnprocessable(ctx, err.Error())
		return
	}
	f, path, err := writeNewLocalMarketPlugin(req.Name, class, content)
	if err != nil {
		if strings.Contains(err.Error(), "存在") {
			ApiConflict(ctx, err.Error())
		} else {
			ApiInternalError(ctx, err.Error())
		}
		return
	}
	item := buildAppScriptItem(path, f)
	if item == nil {
		item = &appScriptItem{ID: f.UUID, Title: f.Title, Language: language, Executable: true, Installed: true, File: path}
	}
	ApiCreated(ctx, "/api/admin/app-scripts/"+item.ID, item)
}

// normalizeAppScriptFileName 校验 .ts/.go 占位脚本文件名。
func normalizeAppScriptFileName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", errors.New("脚本名称不能为空")
	}
	if strings.ContainsAny(name, `/\:<>"|?*`) || strings.Contains(name, "..") {
		return "", errors.New("脚本名称不合法")
	}
	ext := strings.ToLower(filepath.Ext(name))
	if ext != ".ts" && ext != ".go" {
		return "", errors.New("占位脚本文件名必须是 .ts 或 .go 文件")
	}
	title := strings.TrimSuffix(name, filepath.Ext(name))
	if strings.TrimSpace(title) == "" || title == "." {
		return "", errors.New("脚本名称不能为空")
	}
	if windowsReservedPathBase(title) {
		return "", errors.New("脚本名称不能使用 Windows 保留设备名")
	}
	return title + ext, nil
}

func writeNewPlaceholderAppScript(name, language, content string) (*appScriptItem, error) {	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("脚本名称不能为空")
	}
	if strings.TrimSpace(content) == "" {
		return nil, errors.New("脚本内容不能为空")
	}
	ext := ".ts"
	if language == appScriptLangGolang || language == appScriptLangAdapterGo {
		ext = ".go"
	}
	fileName, err := normalizeAppScriptFileName(name + ext)
	if err != nil {
		return nil, fmt.Errorf("脚本名称 %s 不是有效文件名：%v", name, err)
	}
	root := nodePluginsRoot()
	target := filepath.Join(root, "local")
	if err := os.MkdirAll(target, 0755); err != nil {
		return nil, err
	}
	index := filepath.Join(target, fileName)
	if !isAppScriptPathInsideRoot(index, root) {
		return nil, errors.New("脚本文件路径不合法")
	}
	file, err := os.OpenFile(index, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		if os.IsExist(err) {
			return nil, fmt.Errorf("脚本文件已存在：%s", fileName)
		}
		return nil, err
	}
	if _, err := file.WriteString(content); err != nil {
		_ = file.Close()
		_ = os.Remove(index)
		return nil, err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(index)
		return nil, err
	}
	rel, err := filepath.Rel(root, index)
	if err != nil {
		return nil, err
	}
	return &appScriptItem{
		ID:         appScriptFileIDPrefix + filepath.ToSlash(rel),
		Title:      strings.TrimSuffix(fileName, ext),
		Name:       strings.TrimSuffix(fileName, ext),
		Language:   language,
		Executable: false,
		File:       filepath.ToSlash(rel),
	}, nil
}

// isAppScriptPathInsideRoot 校验 .ts/.go 占位脚本路径未跳出插件根目录。
func isAppScriptPathInsideRoot(path, root string) bool {
	clean := filepath.Clean(path)
	rel, err := filepath.Rel(root, clean)
	if err != nil || rel == "." || filepath.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false
	}
	return true
}

// resolveAppScriptTarget 将 :id 解析为脚本路径与运行时类别。
// 插件 UUID 返回已加载插件信息；file: 前缀返回占位脚本路径。
func resolveAppScriptTarget(id string) (*common.Function, string, string, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, "", "", errors.New("缺少脚本 ID")
	}
	if strings.HasPrefix(id, appScriptFileIDPrefix) {
		rel := strings.TrimPrefix(id, appScriptFileIDPrefix)
		if rel == "" {
			return nil, "", "", errors.New("缺少脚本文件路径")
		}
		path := filepath.Join(nodePluginsRoot(), filepath.FromSlash(rel))
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".ts" && ext != ".go" {
			return nil, "", "", errors.New("仅 TypeScript/Golang 占位脚本使用 file 标识")
		}
		if _, err := os.Stat(path); err != nil {
			return nil, "", "", errors.New("脚本文件不存在")
		}
		return nil, path, "", nil
	}
	f, err := nodeFunctionByID(id)
	if err != nil {
		return nil, "", "", err
	}
	path, err := checkedNodeScriptPath(f.Path)
	if err != nil {
		return nil, "", "", err
	}
	return f, path, f.Type, nil
}

func getAppScript(ctx *gin.Context) {
	f, path, _, err := resolveAppScriptTarget(ctx.Param("id"))
	if err != nil {
		ApiNotFound(ctx, err.Error())
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		ApiInternalError(ctx, err.Error())
		return
	}
	item := buildAppScriptItem(path, f)
	if item == nil {
		ApiNotFound(ctx, "脚本不存在或不是应用脚本")
		return
	}
	ApiOK(ctx, gin.H{
		"id":         item.ID,
		"title":      item.Title,
		"name":       item.Name,
		"language":   item.Language,
		"executable": item.Executable,
		"file":       item.File,
		"path":       path,
		"installed":  item.Installed,
		"editable":   true,
		"content":    string(data),
	})
}

func saveAppScript(ctx *gin.Context) {
	req := appScriptRequest{}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ApiFail(ctx, err.Error())
		return
	}
	req.Name = strings.TrimSpace(ctx.Param("id"))
	f, path, _, err := resolveAppScriptTarget(req.Name)
	if err != nil {
		ApiNotFound(ctx, err.Error())
		return
	}
	content := strings.TrimSpace(req.Content)
	if content == "" {
		ApiUnprocessable(ctx, "脚本源码不能为空")
		return
	}
	if f != nil {
		if _, err := validateExistingPluginContent(content); err != nil {
			ApiUnprocessable(ctx, err.Error())
			return
		}
		if err := validateLocalPluginRequestName(nodePluginNameFromPath(path), content, f.Type); err != nil {
			ApiUnprocessable(ctx, err.Error())
			return
		}
		refreshed, err := replaceLoadedPluginSource(path, []byte(content), nodePluginIdentityFromPath(path), f.Type)
		if err != nil {
			ApiInternalError(ctx, err.Error())
			return
		}
		ApiOK(ctx, gin.H{"id": refreshed.UUID, "title": refreshed.Title, "path": path})
		return
	}
	if err := writeAppScriptFileAtomic(path, []byte(content)); err != nil {
		ApiInternalError(ctx, err.Error())
		return
	}
	ApiOK(ctx, gin.H{"id": req.Name, "path": path})
}

func writeAppScriptFileAtomic(path string, content []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".app-script-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(content); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	if err := os.Chmod(tmpName, 0644); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, path)
}

func deleteAppScript(ctx *gin.Context) {
	id := strings.TrimSpace(ctx.Param("id"))
	f, path, _, err := resolveAppScriptTarget(id)
	if err != nil {
		ApiNotFound(ctx, err.Error())
		return
	}
	if f != nil {
		if err := ensurePluginModuleUnused(f, installedPluginSnapshot()); err != nil {
			ApiConflict(ctx, err.Error())
			return
		}
		if err := removeNodePluginScript(path); err != nil {
			ApiInternalError(ctx, err.Error())
			return
		}
		AddNodePlugin(strings.ReplaceAll(path, "\\", "/"), nodePluginIdentityFromPath(path), UNKNOWN)
		ApiOK(ctx, nil)
		return
	}
	if err := os.Remove(path); err != nil {
		ApiInternalError(ctx, err.Error())
		return
	}
	ApiOK(ctx, nil)
}

// runAppScriptDebug 以 SSE 运行一次应用脚本，流式回传 stdout/stderr 与退出码。
func runAppScriptDebug(ctx *gin.Context) {
	id := strings.TrimSpace(ctx.Param("id"))
	f, path, class, err := resolveAppScriptTarget(id)
	if err != nil {
		ApiNotFound(ctx, err.Error())
		return
	}
	if f != nil {
		class = f.Type
	} else {
		ApiUnprocessable(ctx, "该语言暂不支持在线调试")
		return
	}
	if !pluginExecutionEnabled(f) {
		ApiUnprocessable(ctx, "脚本未启用，无法调试")
		return
	}
	if _, loaded := appScriptDebugLocks.LoadOrStore(id, struct{}{}); loaded {
		ApiConflict(ctx, "该脚本正在调试中")
		return
	}
	defer appScriptDebugLocks.Delete(id)

	cmd, err := buildAppScriptDebugCommand(path, class, f)
	if err != nil {
		ApiUnprocessable(ctx, err.Error())
		return
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		ApiUnprocessable(ctx, "获取脚本标准输出管道失败："+err.Error())
		return
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		_ = stdout.Close()
		ApiUnprocessable(ctx, "获取脚本标准错误管道失败："+err.Error())
		return
	}
	if err := cmd.Start(); err != nil {
		_ = stdout.Close()
		_ = stderr.Close()
		ApiUnprocessable(ctx, "脚本进程启动失败："+err.Error())
		return
	}

	flusher, ok := ctx.Writer.(http.Flusher)
	if !ok {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		ApiError(ctx, http.StatusInternalServerError, "Streaming not supported")
		return
	}
	ctx.Header("Content-Type", "text/event-stream")
	ctx.Header("Cache-Control", "no-cache")
	ctx.Header("Connection", "keep-alive")
	ctx.Header("X-Accel-Buffering", "no")

	timeout := 120
	if v := ctx.Query("timeout"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			timeout = n
		}
	}
	if timeout > 600 {
		timeout = 600
	}
	timer := time.NewTimer(time.Duration(timeout) * time.Second)
	defer timer.Stop()

	writeEvent := func(kind, message string) bool {
		if _, err := fmt.Fprintf(ctx.Writer, "data: %s %s\n\n", kind, message); err != nil {
			return false
		}
		flusher.Flush()
		return true
	}
	done := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(2)
	stream := func(pipe io.Reader, kind string) {
		defer wg.Done()
		scanner := bufio.NewScanner(pipe)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			select {
			case <-done:
				return
			default:
			}
			if !writeEvent(kind, scanner.Text()) {
				return
			}
		}
	}
	go stream(stdout, "out")
	go stream(stderr, "err")

	clientGone := ctx.Request.Context().Done()
	waitCh := make(chan error, 1)
	go func() {
		wg.Wait()
		waitCh <- nil
	}()
	processDone := make(chan error, 1)
	go func() {
		processDone <- cmd.Wait()
	}()

	killed := false
	start := time.Now()
	var waitErr error
loop:
	for {
		select {
		case <-clientGone:
			_ = cmd.Process.Kill()
			killed = true
			waitErr = <-processDone
			break loop
		case <-timer.C:
			_ = cmd.Process.Kill()
			killed = true
			waitErr = <-processDone
			break loop
		case waitErr = <-processDone:
			break loop
		}
	}
	close(done)
	<-waitCh

	// 进程结束后残余输出做一次性兜底冲刷。
	drain := func(pipe io.Reader, kind string) {
		scanner := bufio.NewScanner(pipe)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			if !writeEvent(kind, scanner.Text()) {
				return
			}
		}
	}
	drain(stdout, "out")
	drain(stderr, "err")

	elapsed := time.Since(start)
	exitCode := 0
	if waitErr != nil {
		var exitErr *exec.ExitError
		if errors.As(waitErr, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}
	if killed {
		writeEvent("kill", fmt.Sprintf("脚本已被终止（退出码 %d）", exitCode))
	} else if waitErr != nil {
		writeEvent("exit", fmt.Sprintf("exit_code=%d error=%v", exitCode, waitErr))
	} else {
		writeEvent("exit", fmt.Sprintf("exit_code=0 duration=%s", elapsed))
	}
}

// buildAppScriptDebugCommand 复用插件进程的运行时装配逻辑，构造一次性调试进程。
func buildAppScriptDebugCommand(path, class string, f *common.Function) (*exec.Cmd, error) {
	if class != NODE && class != PYTHON {
		return nil, errors.New("该语言暂不支持在线调试")
	}
	var cmd *exec.Cmd
	switch class {
	case NODE:
		workDir := nodePluginWorkDir(path)
		if err := ensureNodeSillygirlModule(workDir); err != nil {
			return nil, fmt.Errorf("NodeJS sillygirl 模块初始化失败：%v", err)
		}
		if err := ensureNodeRuntimeDependencies(workDir); err != nil {
			return nil, fmt.Errorf("NodeJS 运行时依赖安装失败：%v", err)
		}
		bin, err := resolveNodeCommand()
		if err != nil {
			return nil, fmt.Errorf("NodeJS 运行时未找到：%v", err)
		}
		if preload, err := ensureNodeRuntimePreload(); err == nil {
			cmd = exec.Command(bin, "--require", preload, path)
		} else {
			cmd = exec.Command(bin, path)
		}
		cmd.Dir = workDir
	case PYTHON:
		bin, args, err := resolvePythonCommand()
		if err != nil {
			return nil, fmt.Errorf("Python 运行时未找到：%v", err)
		}
		args = append(args, "-u", path)
		cmd = exec.Command(bin, args...)
		cmd.Dir = filepath.Dir(path)
		pythonPath, err := ensurePythonSillygirlModule()
		if err != nil {
			return nil, fmt.Errorf("Python sillygirl 模块初始化失败：%v", err)
		}
		if err := ensurePipxRuntimeEnv(); err != nil {
			return nil, fmt.Errorf("Python 运行时依赖安装失败：%v", err)
		}
		cmd.Env = append(cmd.Env, pythonRuntimeEnvVars(pythonPath)...)
	}
	cmd.Env = append(os.Environ(), cmd.Env...)
	if class == NODE {
		if nodePath := nodeRuntimeNodePath(); nodePath != "" {
			cmd.Env = append(cmd.Env, "NODE_PATH="+nodePath)
		}
	}
	grpcAddress, grpcErr := grpcClientAddress()
	if grpcErr != nil {
		return nil, fmt.Errorf("gRPC 插件运行时未就绪：%v", grpcErr)
	}
	runtimeID := utils.GenUUID()
	uuid := ""
	if f != nil {
		uuid = f.UUID
	}
	cmd.Env = append(cmd.Env,
		"RUNTIME_ID="+runtimeID,
		"PLUGIN_ID="+uuid,
		"SILLYGIRL_GRPC_ADDR="+grpcAddress,
		"SILLYGIRL_GRPC_TOKEN="+grpcRuntimeMetadataToken(),
	)
	cmd.Env = append(cmd.Env, sillyGirlRuntimeEnv()...)
	if class == NODE || class == PYTHON {
		cmd.Env = append(cmd.Env, "PLUGIN_CONFIG_JSON="+string(utils.JsonMarshal(getPluginUserConfig(uuid))))
	}
	if class == NODE && f != nil && f.Web {
		cmd.Env = append(cmd.Env, "SILLYGIRL_WEB=true")
	}
	return cmd, nil
}
