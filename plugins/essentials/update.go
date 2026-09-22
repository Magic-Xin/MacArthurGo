package essentials

import (
	"MacArthurGo/base"
	"MacArthurGo/structs"
	"MacArthurGo/structs/cqcode"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

type Update struct {
	mu         sync.RWMutex
	version    string
	uploadTime time.Time
}

func registerUpdate() error {
	update := Update{}
	plugin := &Plugin{
		Name:    "update",
		Enabled: true,
		Args:    []string{"/update"},
		Handler: &update,
	}
	return Register(plugin)
}

func (u *Update) Start(ctx context.Context, send chan<- []byte) {
	if base.Config.UpdateUrl != "" && base.Config.UpdateInterval > 0 {
		log.Println("Starting update watcher")
		go u.UpdateWatcher(ctx, send)
	}
}

func (u *Update) ReceiveMessage(messageStruct *structs.MessageStruct, send chan<- []byte) {
	if messageStruct.Command != "/update" {
		return
	}
	if base.Config.UpdateUrl == "" {
		return
	}
	if base.Branch == "Release" {
		send <- SendMsg(messageStruct, "暂时不支持 Release 版本自动更新", nil, false, false, "")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	err := u.getVersion(ctx)
	if err != nil {
		send <- SendMsg(messageStruct, fmt.Sprintf("获取最新版本失败: %v", err), nil, false, false, "")
		return
	}

	u.mu.RLock()
	version, uploadTime := u.version, u.uploadTime
	u.mu.RUnlock()

	message := []cqcode.ArrayMessage{*cqcode.Text("本地版本:\n分支: " + base.Branch + "\n" + "版本: " + base.Version + "\n" + "编译时间: " + base.BuildTime),
		*cqcode.Text("\n\n最新版本 (dev):\n版本: " + version + "\n" + "上传时间: " + uploadTime.Format("2006-01-02 15:04:05"))}
	if base.Version != version {
		message = append(message, *cqcode.Text(fmt.Sprintf("\n\n有更新！\n请 admin 使用 /update %s 更新到最新版本\n注意：自动更新有风险，请确保可以手动处理未知问题", version)))
	} else {
		message = append(message, *cqcode.Text("\n\n版本一致，无需更新"))
	}
	send <- SendMsg(messageStruct, "", message, false, false, "")
}

func (*Update) ReceiveEcho(*structs.EchoMessageStruct, chan<- []byte) {}

func (u *Update) UpdateWatcher(ctx context.Context, send chan<- []byte) {
	interval := time.Duration(base.Config.UpdateInterval) * time.Second
	lastNotified := ""
	for {
		err := u.getVersion(ctx)
		if err != nil {
			log.Printf("Get version error: %v", err)
			if !waitFor(ctx, interval) {
				return
			}
			continue
		}
		version, uploadTime := u.snapshot()
		if base.Version == version {
			lastNotified = ""
		} else if version != lastNotified {
			sendStruct := structs.MessageStruct{
				MessageType: "private",
				UserId:      base.Config.Admin,
			}

			message := []cqcode.ArrayMessage{*cqcode.Text("检测到版本更新！\n\n本地版本:\n分支: " + base.Branch + "\n" + "版本: " + base.Version + "\n" + "编译时间: " + base.BuildTime),
				*cqcode.Text("\n\n最新版本 (dev):\n版本: " + version + "\n" + "上传时间: " + uploadTime.Format("2006-01-02 15:04:05"))}
			select {
			case send <- SendMsg(&sendStruct, "", message, false, false, ""):
				lastNotified = version
			case <-ctx.Done():
				return
			}
		}
		if !waitFor(ctx, interval) {
			return
		}
	}
}

func waitFor(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func (u *Update) snapshot() (string, time.Time) {
	u.mu.RLock()
	defer u.mu.RUnlock()
	return u.version, u.uploadTime
}

func (u *Update) getVersion(ctx context.Context) error {
	versionURL := strings.TrimRight(base.Config.UpdateUrl, "/") + "/version.json"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, versionURL, nil)
	if err != nil {
		return fmt.Errorf("create version request: %w", err)
	}
	resp, err := HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("request version: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("version request returned %s", resp.Status)
	}

	var payload struct {
		UploadTime string `json:"upload_time"`
		Version    string `json:"version"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return fmt.Errorf("decode version response: %w", err)
	}
	if strings.TrimSpace(payload.Version) == "" {
		return fmt.Errorf("version response is missing version")
	}
	uploadTime, err := time.Parse(time.RFC3339, payload.UploadTime)
	if err != nil {
		return fmt.Errorf("parse upload time: %w", err)
	}
	tz, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		tz = time.FixedZone("Asia/Shanghai", 8*60*60)
	}
	version := payload.Version
	if len(version) > 7 {
		version = version[:7]
	}
	u.mu.Lock()
	u.uploadTime = uploadTime.In(tz)
	u.version = version
	u.mu.Unlock()

	return nil
}
