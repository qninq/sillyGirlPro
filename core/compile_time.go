package core

// compiled_at 是构建期版本号：发布流水线通过
// -ldflags "-X github.com/qninq/sillyGirlPro/core.compiled_at=版本号" 注入。
// 留空表示源码直接构建，此时版本号取 version.go 的 appVersion 兜底——
// 发版只需同步 appVersion / VERSION / version.ts，无需再改这里。
var compiled_at = ""
