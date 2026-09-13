package core

import (
	"archive/zip"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/qninq/sillyGirlPro/utils"
)

// 定时备份：每日在配置的小时自动打包备份（复用 system_backup 的打包逻辑）到
// 数据目录 backups/ 下，按保留数量自动清理旧备份；支持从备份 ZIP 一键恢复
// （存储桶条目覆盖写入 + 数据文件解压覆盖，完成后自动重启加载）。
const backupAutoFilePrefix = "sillygirl-auto-"

func backupAutoDir() string {
	return filepath.Join(utils.GetDataHome(), "backups")
}

func backupAutoEnabled() bool {
	return sillyGirl.GetBool("backup_auto_enable", false)
}

func backupAutoHour() int {
	hour := sillyGirl.GetInt("backup_auto_hour", 4)
	if hour < 0 || hour > 23 {
		hour = 4
	}
	return hour
}

func backupAutoKeep() int {
	keep := sillyGirl.GetInt("backup_auto_keep", 7)
	if keep < 1 {
		keep = 7
	}
	return keep
}

func init() {
	GinApi(GET, "/api/admin/system-backups/auto", RequireAuth, listAutoBackups)
	GinApi(GET, "/api/admin/system-backups/auto/downloads", RequireAuth, downloadAutoBackup)
	GinApi(POST, "/api/admin/system-backups/auto/deletions", RequireAuth, deleteAutoBackup)
	GinApi(POST, "/api/admin/system-backups/restores", RequireAuth, restoreSystemBackup)
	GinApi(POST, "/api/admin/system-backups/auto/restores", RequireAuth, restoreAutoBackupByName)
	go backupAutoScheduler()
}

func backupAutoScheduler() {
	for {
		now := time.Now()
		if backupAutoEnabled() && now.Hour() == backupAutoHour() {
			if err := runAutoBackupIfMissing(now); err != nil {
				Logs.Error("定时备份失败：%v", err)
			}
		}
		time.Sleep(30 * time.Minute)
	}
}

func runAutoBackupIfMissing(now time.Time) error {
	dir := backupAutoDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	dayPrefix := backupAutoFilePrefix + now.Format("20060102")
	if entries, err := os.ReadDir(dir); err == nil {
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), dayPrefix) {
				return nil // 今日已备份
			}
		}
	}
	name := backupAutoFilePrefix + now.Format("20060102-150405") + ".zip"
	file, err := os.Create(filepath.Join(dir, name))
	if err != nil {
		return err
	}
	if _, err := writeSystemBackup(file, MakeBucket(""), utils.GetDataHome(), now); err != nil {
		_ = file.Close()
		_ = os.Remove(filepath.Join(dir, name))
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	pruneAutoBackups()
	Logs.Info("定时备份完成：" + name)
	return nil
}

func pruneAutoBackups() {
	autos := listAutoBackupFiles()
	keep := backupAutoKeep()
	for i := keep; i < len(autos); i++ {
		_ = os.Remove(filepath.Join(backupAutoDir(), autos[i].name))
	}
}

type backupAutoFile struct {
	name     string
	size     int64
	modified time.Time
}

// listAutoBackupFiles 返回自动备份文件，按名称（即时间戳）倒序。
func listAutoBackupFiles() []backupAutoFile {
	entries, err := os.ReadDir(backupAutoDir())
	if err != nil {
		return nil
	}
	var files []backupAutoFile
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasPrefix(name, backupAutoFilePrefix) || !strings.HasSuffix(name, ".zip") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		files = append(files, backupAutoFile{name: name, size: info.Size(), modified: info.ModTime()})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].name > files[j].name })
	return files
}

// backupAutoFileNameValid 校验备份文件名，拒绝路径穿越与非自动备份文件。
func backupAutoFileNameValid(name string) bool {
	name = strings.TrimSpace(name)
	if !strings.HasPrefix(name, backupAutoFilePrefix) || !strings.HasSuffix(name, ".zip") {
		return false
	}
	return name == filepath.Base(name) && !strings.ContainsAny(name, "/\\")
}

func listAutoBackups(ctx *gin.Context) {
	files := listAutoBackupFiles()
	rows := make([]gin.H, 0, len(files))
	for _, file := range files {
		rows = append(rows, gin.H{
			"name":     file.name,
			"size":     file.size,
			"modified": file.modified.Unix(),
		})
	}
	ApiOK(ctx, gin.H{
		"enabled": backupAutoEnabled(),
		"hour":    backupAutoHour(),
		"keep":    backupAutoKeep(),
		"dir":     backupAutoDir(),
		"files":   rows,
	})
}

func downloadAutoBackup(ctx *gin.Context) {
	name := strings.TrimSpace(ctx.Query("name"))
	if !backupAutoFileNameValid(name) {
		ApiUnprocessable(ctx, "备份文件名不合法")
		return
	}
	path := filepath.Join(backupAutoDir(), name)
	if _, err := os.Stat(path); err != nil {
		ApiNotFound(ctx, "备份文件不存在")
		return
	}
	ctx.Header("Cache-Control", "no-store")
	ctx.FileAttachment(path, name)
}

func deleteAutoBackup(ctx *gin.Context) {
	payload := struct {
		Name string `json:"name"`
	}{}
	if err := json.NewDecoder(ctx.Request.Body).Decode(&payload); err != nil {
		ApiFail(ctx, "请求体不是有效 JSON")
		return
	}
	if !backupAutoFileNameValid(payload.Name) {
		ApiUnprocessable(ctx, "备份文件名不合法")
		return
	}
	if err := os.Remove(filepath.Join(backupAutoDir(), payload.Name)); err != nil {
		ApiNotFound(ctx, "备份文件不存在")
		return
	}
	ApiOK(ctx, nil)
}

// restoreSystemBackup 从上传的备份 ZIP 一键恢复：存储桶条目覆盖写入当前库
// （备份里没有的键保留），数据文件解压覆盖到数据目录，完成后自动重启加载。
func restoreSystemBackup(ctx *gin.Context) {
	file, err := ctx.FormFile("file")
	if err != nil {
		ApiUnprocessable(ctx, "请通过 multipart 字段 file 上传备份 ZIP 文件")
		return
	}
	zipFile, err := file.Open()
	if err != nil {
		ApiError(ctx, http.StatusInternalServerError, "读取上传文件失败："+err.Error())
		return
	}
	defer zipFile.Close()

	reader, err := zip.NewReader(zipFile, file.Size) // multipart.File 实现了 io.ReaderAt
	if err != nil {
		ApiUnprocessable(ctx, "备份文件不是有效的 ZIP："+err.Error())
		return
	}

	summary, err := applyRestore(reader)
	if err != nil {
		ApiUnprocessable(ctx, err.Error())
		return
	}
	Logs.Warn("执行备份恢复（上传文件）：" + summary)
	finishRestore(ctx, summary)
}

// restoreAutoBackupByName 从数据目录 backups/ 下的自动备份按文件名一键恢复。
func restoreAutoBackupByName(ctx *gin.Context) {
	payload := struct {
		Name string `json:"name"`
	}{}
	if err := json.NewDecoder(ctx.Request.Body).Decode(&payload); err != nil {
		ApiFail(ctx, "请求体不是有效 JSON")
		return
	}
	if !backupAutoFileNameValid(payload.Name) {
		ApiUnprocessable(ctx, "备份文件名不合法")
		return
	}
	path := filepath.Join(backupAutoDir(), payload.Name)
	file, err := os.Open(path)
	if err != nil {
		ApiNotFound(ctx, "备份文件不存在")
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		ApiError(ctx, http.StatusInternalServerError, "读取备份文件失败："+err.Error())
		return
	}
	reader, err := zip.NewReader(file, info.Size())
	if err != nil {
		ApiUnprocessable(ctx, "备份文件不是有效的 ZIP："+err.Error())
		return
	}
	summary, err := applyRestore(reader)
	if err != nil {
		ApiUnprocessable(ctx, err.Error())
		return
	}
	Logs.Warn("执行备份恢复（" + payload.Name + "）：" + summary)
	finishRestore(ctx, summary)
}

// applyRestore 校验备份并执行恢复（存储条目覆盖 + 数据文件解压），返回摘要。
func applyRestore(reader *zip.Reader) (string, error) {
	manifest, storageSnapshot, err := parseSystemBackupZip(reader)
	if err != nil {
		return "", err
	}
	buckets, keys, err := restoreSystemBackupStorage(storageSnapshot)
	if err != nil {
		return "", fmt.Errorf("恢复存储数据失败：%v", err)
	}
	fileCount, err := restoreSystemBackupFiles(reader, utils.GetDataHome())
	if err != nil {
		return "", fmt.Errorf("恢复数据文件失败：%v", err)
	}
	return fmt.Sprintf(
		"已恢复 %d 个存储桶 / %d 个键 / %d 个数据文件（备份创建于 %s，版本 %s）。系统将自动重启以加载恢复的数据。",
		buckets, keys, fileCount, manifest.CreatedAt, manifest.AppVersion,
	), nil
}

func finishRestore(ctx *gin.Context, summary string) {
	go func() {
		time.Sleep(time.Second)
		sillyGirl.Set("started_at", time.Now().Format("2006-01-02 15:04:05"))
	}()
	ApiOK(ctx, gin.H{"summary": summary})
}

func parseSystemBackupZip(reader *zip.Reader) (systemBackupManifest, systemBackupStorage, error) {
	var manifest systemBackupManifest
	var snapshot systemBackupStorage

	manifestFile := findZipFile(reader, "manifest.json")
	storageFile := findZipFile(reader, "storage.json")
	if manifestFile == nil || storageFile == nil {
		return manifest, snapshot, errors.New("备份缺少 manifest.json 或 storage.json，不是有效的系统备份")
	}
	manifestData, err := readZipFile(manifestFile)
	if err != nil {
		return manifest, snapshot, err
	}
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		return manifest, snapshot, errors.New("备份 manifest.json 解析失败")
	}
	if manifest.Format != systemBackupFormat {
		return manifest, snapshot, fmt.Errorf("备份格式不支持：%s（需要 %s）", manifest.Format, systemBackupFormat)
	}
	storageData, err := readZipFile(storageFile)
	if err != nil {
		return manifest, snapshot, err
	}
	if err := json.Unmarshal(storageData, &snapshot); err != nil {
		return manifest, snapshot, errors.New("备份 storage.json 解析失败")
	}
	if snapshot.Format != systemBackupFormat {
		return manifest, snapshot, fmt.Errorf("备份存储格式不支持：%s", snapshot.Format)
	}
	return manifest, snapshot, nil
}

func findZipFile(reader *zip.Reader, name string) *zip.File {
	for _, file := range reader.File {
		if file.Name == name {
			return file
		}
	}
	return nil
}

func readZipFile(file *zip.File) ([]byte, error) {
	rc, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(io.LimitReader(rc, 512<<20))
}

// restoreSystemBackupStorage 将备份中的键值覆盖写入当前存储（备份没有的键保留）。
func restoreSystemBackupStorage(snapshot systemBackupStorage) (buckets, keys int, err error) {
	for _, bucket := range snapshot.Buckets {
		target := MakeBucket(bucket.Name)
		for _, entry := range bucket.Entries {
			rawKey, err := base64.StdEncoding.DecodeString(entry.KeyBase64)
			if err != nil {
				return buckets, keys, fmt.Errorf("键解码失败（%s）：%v", bucket.Name, err)
			}
			rawValue, err := base64.StdEncoding.DecodeString(entry.ValueBase64)
			if err != nil {
				return buckets, keys, fmt.Errorf("值解码失败（%s）：%v", bucket.Name, err)
			}
			if _, _, err := target.Set(string(rawKey), string(rawValue)); err != nil {
				return buckets, keys, fmt.Errorf("写入失败（%s）：%v", bucket.Name, err)
			}
			keys++
		}
		buckets++
	}
	return buckets, keys, nil
}

// restoreSystemBackupFiles 将备份里的 files/ 数据文件解压覆盖到数据目录。
func restoreSystemBackupFiles(reader *zip.Reader, dataHome string) (int, error) {
	fileCount := 0
	for _, file := range reader.File {
		if !strings.HasPrefix(file.Name, "files/") || file.FileInfo().IsDir() {
			continue
		}
		rel := strings.TrimPrefix(file.Name, "files/")
		rel = filepath.Clean(filepath.FromSlash(rel))
		if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return fileCount, errors.New("备份文件路径越界：" + file.Name)
		}
		target := filepath.Join(dataHome, rel)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return fileCount, err
		}
		rc, err := file.Open()
		if err != nil {
			return fileCount, err
		}
		out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
		if err != nil {
			rc.Close()
			return fileCount, err
		}
		_, copyErr := io.Copy(out, rc)
		closeErr := out.Close()
		rc.Close()
		if copyErr != nil {
			return fileCount, copyErr
		}
		if closeErr != nil {
			return fileCount, closeErr
		}
		fileCount++
	}
	return fileCount, nil
}
