package essentials

import (
	"MacArthurGo/base"
	"MacArthurGo/structs/cqcode"
	"encoding/json"
	"fmt"
	"github.com/minio/selfupdate"
	"io"
	"log"
	"net/http"
	"runtime"
	"time"
)

type Update struct {
	Plugin
	send       *chan []byte
	version    string
	uploadTime time.Time
}

func init() {
	update := Update{
		Plugin: Plugin{
			Name:    "更新",
			Enabled: true,
			Args:    []string{"/update"},
		},
	}

	PluginArray = append(PluginArray, &PluginInterface{Interface: &update})
}

func (u *Update) ReceiveAll(_ *map[string]any, send *chan []byte) {
	if u.send == nil && base.Config.UpdateUrl != "" && base.Config.UpdateInterval != 0 {
		u.send = send
		log.Println("Init update check goroutine...")
		go u.updateCheck()
	}
}

func (u *Update) ReceiveMessage(ctx *map[string]any, send *chan []byte) {
	if base.Config.UpdateUrl == "" {
		return
	}

	words := SplitArgument(ctx)
	if len(words) == 0 {
		return
	}
	if words[0] != u.Args[0] {
		return
	}
	if base.Branch == "Release" {
		*send <- *SendMsg(ctx, "暂时不支持 Release 版本自动更新", nil, false, false)
		return
	}

	if len(words) == 1 {
		err := u.getVersion()
		if err != nil {
			*send <- *SendMsg(ctx, fmt.Sprintf("获取最新版本失败: %v", err), nil, false, false)
			return
		}

		message := []cqcode.ArrayMessage{*cqcode.Text("本地版本:\n分支: " + base.Branch + "\n" + "版本: " + base.Version + "\n" + "编译时间: " + base.BuildTime),
			*cqcode.Text("\n\n最新版本 (dev):\n版本: " + u.version + "\n" + "上传时间: " + u.uploadTime.Format("2006-01-02 15:04:05"))}
		if base.Version != u.version {
			message = append(message, *cqcode.Text(fmt.Sprintf("\n\n有更新！\n请 admin 使用 /update %s 更新到最新版本\n注意：自动更新有风险，请确保可以手动处理未知问题", u.version)))
		} else {
			message = append(message, *cqcode.Text("\n\n版本一致，无需更新"))
		}
		*send <- *SendMsg(ctx, "", &message, false, false)
	}

	if len(words) == 2 {
		if (int64((*ctx)["sender"].(map[string]any)["user_id"].(float64))) != base.Config.Admin {
			*send <- *SendMsg(ctx, "没有更新权限", nil, false, true)
			return
		}

		if base.Version == u.version {
			*send <- *SendMsg(ctx, "版本一致，无需更新", nil, false, true)
			return
		}

		if runtime.GOOS != "windows" && runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
			*send <- *SendMsg(ctx, "不支持当前操作系统，请手动更新", nil, false, true)
			return
		}

		if words[1] != u.version {
			*send <- *SendMsg(ctx, "版本不一致，无法更新", nil, false, true)
		}

		updateUrl := base.Config.UpdateUrl + "MacArthurGo-" + runtime.GOOS + "-" + runtime.GOARCH

		if runtime.GOOS == "windows" {
			updateUrl += ".exe"
		}

		*send <- *SendMsg(ctx, "开始更新...", nil, false, true)
		err := u.doUpdate(updateUrl)
		if err != nil {
			*send <- *SendMsg(ctx, fmt.Sprintf("更新失败: %v", err), nil, false, true)
			return
		}
		*send <- *SendMsg(ctx, "更新成功, 请重启 MacArthurGo", nil, false, true)
	}
}

func (u *Update) ReceiveEcho(*map[string]any, *chan []byte) {}

func (u *Update) updateCheck() {
	for {
		err := u.getVersion()
		if err != nil {
			log.Printf("Get version error: %v", err)
			continue
		}
		if base.Version != u.version {
			ctx := map[string]any{
				"message_type": "private",
				"sender": map[string]any{
					"user_id": float64(base.Config.Admin),
				},
			}
			message := []cqcode.ArrayMessage{*cqcode.Text("检测到版本更新！\n\n本地版本:\n分支: " + base.Branch + "\n" + "版本: " + base.Version + "\n" + "编译时间: " + base.BuildTime),
				*cqcode.Text("\n\n最新版本 (dev):\n版本: " + u.version + "\n" + "上传时间: " + u.uploadTime.Format("2006-01-02 15:04:05"))}
			*u.send <- *SendMsg(&ctx, "", &message, false, false)
		}
		time.Sleep(time.Duration(base.Config.UpdateInterval) * time.Second)
	}
}

func (u *Update) getVersion() error {
	resp, err := http.Get(base.Config.UpdateUrl + "version.json")
	if err != nil {
		log.Printf("Get version.json error: %v", err)
		return err
	}
	defer func(Body io.ReadCloser) {
		err = Body.Close()
		if err != nil {
			log.Printf("Close body error: %v", err)
		}
	}(resp.Body)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Read body error: %v", err)
		return err
	}

	var i any
	err = json.Unmarshal(body, &i)
	if err != nil {
		log.Printf("Unmarshal body error: %v", err)
		return err
	}

	uploadTime, err := time.Parse(time.RFC3339, i.(map[string]any)["upload_time"].(string))
	if err != nil {
		log.Printf("Parse upload time error: %v", err)
		return err
	}
	tz, _ := time.LoadLocation("Asia/Shanghai")
	u.uploadTime = uploadTime.In(tz)
	u.version = i.(map[string]any)["version"].(string)[:7]

	return nil
}

func (u *Update) doUpdate(url string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer func(Body io.ReadCloser) {
		err = Body.Close()
		if err != nil {
			log.Printf("Close body error: %v", err)
		}
	}(resp.Body)

	err = selfupdate.Apply(resp.Body, selfupdate.Options{})
	if err != nil {
		if rerr := selfupdate.RollbackError(err); rerr != nil {
			fmt.Printf("Failed to rollback from bad update: %v\n", rerr)
			return rerr
		}
	}
	return err
}
