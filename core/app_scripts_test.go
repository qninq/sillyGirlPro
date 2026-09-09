package core

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestClassifyAppScriptLanguage(t *testing.T) {
	cases := []struct {
		ext      string
		content  string
		expected string
	}{
		{".js", "console.log(1)", appScriptLangES5},
		{".js", "const { sender } = require(\"sillygirl\");", appScriptLangNode},
		{".js", "import { sender } from \"sillygirl\";", appScriptLangNode},
		{".js", "const a = new Adapter({platform: \"x\"});", appScriptLangAdapterJS},
		{".ts", "const x: number = 1;", appScriptLangTypeScript},
		{".py", "print(\"hi\")", appScriptLangPython},
		{".py", "a = await s.getAdapter()", appScriptLangAdapterPython},
		{".go", "package main", appScriptLangGolang},
		{".go", "type myAdapter struct{}", appScriptLangAdapterGo},
	}
	for _, c := range cases {
		got := classifyAppScriptLanguage(c.ext, c.content)
		if got != c.expected {
			t.Errorf("classify(%q, %q) = %q, want %q", c.ext, c.content, got, c.expected)
		}
	}
}

func TestAppScriptLanguageExecutable(t *testing.T) {
	for _, language := range appScriptLanguages {
		want := language != appScriptLangTypeScript && language != appScriptLangGolang && language != appScriptLangAdapterGo
		if appScriptLanguageExecutable(language) != want {
			t.Errorf("appScriptLanguageExecutable(%q) mismatch", language)
		}
	}
}

const appScriptOnStartPlugin = `// [title: Startup Demo]
// [name: startupDemo]
// [desc: startup task demo]
// [version: v1.0.0]
// [on_start: true]
const { sender } = require("sillygirl");
console.log("startup");
`

const appScriptRulePlugin = `// [title: Rule Demo]
// [name: ruleDemo]
// [desc: rule plugin demo]
// [version: v1.0.0]
// [rule: raw ^hello$]
console.log("rule");
`

const appScriptCronPlugin = `// [title: Cron Demo]
// [name: cronDemo]
// [desc: cron plugin demo]
// [version: v1.0.0]
// [cron: 12 8 * * *]
console.log("cron");
`

func TestBuildAppScriptItemIncludesAllScripts(t *testing.T) {
	t.Setenv("SILLYGIRL_DATA_PATH", t.TempDir())
	root := nodePluginsRoot()
	local := filepath.Join(root, "local")
	if err := writeTestFile(filepath.Join(local, "startupDemo.js"), appScriptOnStartPlugin); err != nil {
		t.Fatal(err)
	}
	if err := writeTestFile(filepath.Join(local, "ruleDemo.js"), appScriptRulePlugin); err != nil {
		t.Fatal(err)
	}
	if err := writeTestFile(filepath.Join(local, "cronDemo.js"), appScriptCronPlugin); err != nil {
		t.Fatal(err)
	}
	if err := writeTestFile(filepath.Join(local, "plain.ts"), "const x: number = 1;\n"); err != nil {
		t.Fatal(err)
	}

	items := collectAppScriptItems()
	byFile := map[string]*appScriptItem{}
	for _, item := range items {
		byFile[item.File] = item
	}
	if _, ok := byFile["local/ruleDemo.js"]; !ok {
		t.Error("rule-based message plugin missing from app scripts (all scripts should be listed)")
	}
	startup, ok := byFile["local/startupDemo.js"]
	if !ok {
		t.Fatal("on_start plugin missing from app scripts")
	}
	if !startup.OnStart || startup.Language != appScriptLangNode || !startup.Executable {
		t.Errorf("startup item = %#v", startup)
	}
	cronItem, ok := byFile["local/cronDemo.js"]
	if !ok {
		t.Fatal("cron plugin missing from app scripts")
	}
	if !cronItem.HasCron {
		t.Errorf("cron item missing cron flag: %#v", cronItem)
	}
	ts, ok := byFile["local/plain.ts"]
	if !ok {
		t.Fatal("typescript file missing from app scripts")
	}
	if ts.Executable || ts.Language != appScriptLangTypeScript {
		t.Errorf("typescript item = %#v", ts)
	}
	if !strings.HasPrefix(ts.ID, appScriptFileIDPrefix) {
		t.Errorf("placeholder id = %q", ts.ID)
	}
}

func TestCreateAppScriptWritesPlaceholderFile(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("SILLYGIRL_DATA_PATH", t.TempDir())
	payload := `{"name":"demoTool","language":"typescript","content":"const answer: number = 42;\n"}`
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/admin/app-scripts", strings.NewReader(payload))
	ctx.Request.Header.Set("Content-Type", "application/json")
	createAppScript(ctx)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("create response HTTP %d: %s", recorder.Code, recorder.Body.String())
	}
	path := filepath.Join(nodePluginsRoot(), "local", "demoTool.ts")
	data, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(data), "const answer") {
		t.Fatalf("placeholder script was not written: %v", err)
	}
	items := collectAppScriptItems()
	found := false
	for _, item := range items {
		if item.File == "local/demoTool.ts" {
			found = true
			if item.Executable {
				t.Error("typescript placeholder must not be executable")
			}
		}
	}
	if !found {
		t.Fatal("created placeholder script missing from list")
	}

	// 重复创建同名脚本应返回冲突。
	recorder = httptest.NewRecorder()
	ctx, _ = gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/admin/app-scripts", strings.NewReader(payload))
	ctx.Request.Header.Set("Content-Type", "application/json")
	createAppScript(ctx)
	if recorder.Code != http.StatusConflict {
		t.Fatalf("duplicate create HTTP %d: %s", recorder.Code, recorder.Body.String())
	}
}

func TestRunAppScriptDebugRejectsPlaceholder(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("SILLYGIRL_DATA_PATH", t.TempDir())
	path := filepath.Join(nodePluginsRoot(), "local", "demoTool.ts")
	if err := writeTestFile(path, "const x: number = 1;\n"); err != nil {
		t.Fatal(err)
	}
	id := appScriptFileIDPrefix + "local/demoTool.ts"
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/admin/app-scripts/"+id+"/executions", nil)
	ctx.Params = gin.Params{{Key: "id", Value: id}}
	runAppScriptDebug(ctx)
	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("debug placeholder response HTTP %d: %s", recorder.Code, recorder.Body.String())
	}
}

func TestListAppScriptsRouteShape(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("SILLYGIRL_DATA_PATH", t.TempDir())
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/admin/app-scripts", nil)
	listAppScripts(ctx)
	if recorder.Code != http.StatusOK {
		t.Fatalf("list response HTTP %d: %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "\"items\"") {
		t.Fatalf("list response missing items: %s", recorder.Body.String())
	}
}

func writeTestFile(path string, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0644)
}
