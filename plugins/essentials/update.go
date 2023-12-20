package essentials

import (
	"MacArthurGo/base"
	"MacArthurGo/structs/cqcode"
	"encoding/json"
	"fmt"
	"github.com/gookit/config/v2"
	"github.com/minio/selfupdate"
	"io"
	"log"
	"net/http"
	"runtime"
	"time"
)

type Update struct {
	Plugin
	Url   string
	Admin int64
}

func init() {
	update := Update{
		Plugin: Plugin{
			Name:    "更新",
			Enabled: true,
			Args:    []string{"/update"},
		},
		Url:   config.String("plugins.updateUrl"),
		Admin: config.Int64("admin"),
	}

	PluginArray = append(PluginArray, &PluginInterface{Interface: &update})
}

func (u *Update) ReceiveAll(*map[string]any, *chan []byte) {}

func (u *Update) ReceiveMessage(ctx *map[string]any, send *chan []byte) {
	if u.Url == "" {
		return
	}

	words := SplitArgument(ctx)
	if len(words) < 0 {
		return
	}
	if words[0] != u.Args[0] {
		return
	}
	if base.Branch == "Release" {
		*send <- *SendMsg(ctx, "暂时不支持 Release 版本自动更新", nil, false, false)
		return
	}

	version, uploadTime, err := u.getVersion()
	if err != nil {
		*send <- *SendMsg(ctx, fmt.Sprintf("获取最新版本失败: %v", err), nil, false, false)
		return
	}

	if len(words) == 1 {
		message := []cqcode.ArrayMessage{*cqcode.Text("本地版本:\n分支: " + base.Branch + "\n" + "版本: " + base.Version + "\n" + "编译时间: " + base.BuildTime + "\n"),
			*cqcode.Text("最新版本 (dev):\n版本: " + version + "\n" + "上传时间: " + uploadTime + "\n")}
		if base.Version != version {
			message = append(message, *cqcode.Text(fmt.Sprintf("有更新！\n请 admin 使用 /update %s 更新到最新版本\n注意：自动更新有风险，请确保可以手动处理未知问题", version)))
		} else {
			message = append(message, *cqcode.Text("版本一致，无需更新"))
		}
		*send <- *SendMsg(ctx, "", &message, false, false)
	}
	if len(words) == 2 {
		if int64((*ctx)["sender"].(map[string]any)["user_id"].(float64)) != u.Admin {
			*send <- *SendMsg(ctx, "没有更新权限", nil, false, true)
		}

		if words[1] != version {
			*send <- *SendMsg(ctx, "版本不一致，无法更新", nil, false, true)
		}

		updateUrl := u.Url + "MacArthurGo-" + runtime.GOOS + "-" + runtime.GOARCH + "-" + version
		if runtime.GOOS == "windows" {
			updateUrl += ".exe"
		}
		*send <- *SendMsg(ctx, "开始更新...", nil, false, true)
		err = u.doUpdate(updateUrl)
		if err != nil {
			*send <- *SendMsg(ctx, fmt.Sprintf("更新失败: %v", err), nil, false, true)
			return
		}
		*send <- *SendMsg(ctx, "更新成功", nil, false, true)
	}
}

func (u *Update) ReceiveEcho(*map[string]any, *chan []byte) {}

func (u *Update) getVersion() (string, string, error) {
	resp, err := http.Get(u.Url + "version.json")
	if err != nil {
		log.Printf("Get version.json error: %v", err)
		return "", "", err
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
		return "", "", err
	}

	var i any
	err = json.Unmarshal(body, &i)
	if err != nil {
		log.Printf("Unmarshal body error: %v", err)
		return "", "", err
	}

	uploadTime, err := time.Parse(time.RFC3339, i.(map[string]any)["upload_time"].(string))
	if err != nil {
		log.Printf("Parse upload time error: %v", err)
		return "", "", err
	}
	tz, _ := time.LoadLocation("Asia/Shanghai")
	timeParsed := uploadTime.In(tz).Format("2006-01-02 15:04:05")
	version := i.(map[string]any)["version"].(string)[:7]

	return version, timeParsed, nil
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
		log.Printf("Update error: %v", err)
	}
	return err
}
