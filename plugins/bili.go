package plugins

import (
	"MacArthurGo/plugins/essentials"
	"MacArthurGo/structs/cqcode"
	"encoding/json"
	"fmt"
	"github.com/gookit/config/v2"
	"io"
	"log"
	"net/http"
	"regexp"
)

type Bili struct {
	essentials.Plugin
}

type VideoData struct {
	Title        string
	Author       string
	ThumbnailUrl string
	Aid          string
	Playtime     string
	Danmaku      string
	Url          string
}

type LiveData struct {
	Title        string
	User         string
	ThumbnailUrl string
	RoomId       string
	AreaName     string
	Status       string
	Url          string
}

func init() {
	bili := Bili{
		essentials.Plugin{
			Name:    "B 站链接解析",
			Enabled: config.Bool("plugins.bili.enable"),
		},
	}
	essentials.PluginArray = append(essentials.PluginArray, &essentials.PluginInterface{Interface: &bili})
}

func (b *Bili) ReceiveAll(_ *map[string]any, _ *chan []byte) {}

func (b *Bili) ReceiveMessage(ctx *map[string]any, send *chan []byte) {
	if !b.Enabled {
		return
	}

	const biliShort = `(https://b23.tv/\w+)`
	const video = `https://www.bilibili.com/video/(\w+)`
	const live = `https://live.bilibili.com/(\d+)`

	rawMsg := (*ctx)["raw_message"].(string)
	var (
		videoData *VideoData
		liveData  *LiveData
	)
	if match := regexp.MustCompile(biliShort).FindAllStringSubmatch(rawMsg, -1); match != nil {
		rawMsg = *essentials.GetOriginUrl(match[0][1])
	}
	if match := regexp.MustCompile(video).FindAllStringSubmatch(rawMsg, -1); match != nil {
		videoData = b.getVideoData(match[0][1])
	} else if match = regexp.MustCompile(live).FindAllStringSubmatch(rawMsg, -1); match != nil {
		liveData = b.getLiveData(match[0][1])
	} else {
		return
	}

	if videoData != nil {
		var messageArray []cqcode.ArrayMessage
		messageArray = append(messageArray, *cqcode.Image(videoData.ThumbnailUrl + "\n"))
		messageArray = append(messageArray, *cqcode.Text(videoData.Aid + "\n"))
		messageArray = append(messageArray, *cqcode.Text(videoData.Title + "\n"))
		messageArray = append(messageArray, *cqcode.Text("UP: " + videoData.Author + "\n"))
		messageArray = append(messageArray, *cqcode.Text("播放: " + videoData.Playtime + "	弹幕: " + videoData.Danmaku + "\n"))
		messageArray = append(messageArray, *cqcode.Text(videoData.Url))
		*send <- *essentials.SendMsg(ctx, "", &messageArray, false, false)
	} else if liveData != nil {
		var messageArray []cqcode.ArrayMessage
		messageArray = append(messageArray, *cqcode.Image(liveData.ThumbnailUrl + "\n"))
		messageArray = append(messageArray, *cqcode.Text(liveData.Title + "\n"))
		messageArray = append(messageArray, *cqcode.Text("主播: " + liveData.User + "\n"))
		messageArray = append(messageArray, *cqcode.Text(liveData.RoomId + "\n"))
		messageArray = append(messageArray, *cqcode.Text("分区: " + liveData.AreaName + "\n"))
		messageArray = append(messageArray, *cqcode.Text(liveData.Status + "\n"))
		messageArray = append(messageArray, *cqcode.Text(liveData.Url))
		*send <- *essentials.SendMsg(ctx, "", &messageArray, false, false)
	}
}

func (b *Bili) ReceiveEcho(_ *map[string]any, _ *chan []byte) {}

func (b *Bili) getVideoData(bid string) *VideoData {
	const api = "https://api.bilibili.com/x/web-interface/view?bvid="
	url := api + bid
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Printf("Video url parser request error: %v", err)
		return nil
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("Video url parser response error: %v", err)
		return nil
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Printf("Video url parser close error: %v", err)
		}
	}(resp.Body)
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Video url parser read body error: %v", err)
		return nil
	}

	var i any
	err = json.Unmarshal(body, &i)
	if err != nil {
		log.Printf("Video url parser unmarshal error: %v", err)
		return nil
	}

	ctx := i.(map[string]any)
	if ctx["code"].(float64) != 0 {
		return nil
	}
	data := &VideoData{
		Title:        ctx["data"].(map[string]any)["title"].(string),
		Author:       ctx["data"].(map[string]any)["owner"].(map[string]any)["name"].(string),
		ThumbnailUrl: ctx["data"].(map[string]any)["pic"].(string),
		Aid:          fmt.Sprintf("av%d", int64(ctx["data"].(map[string]any)["aid"].(float64))),
		Playtime:     b.iToS(int64(ctx["data"].(map[string]any)["stat"].(map[string]any)["view"].(float64))),
		Danmaku:      b.iToS(int64(ctx["data"].(map[string]any)["stat"].(map[string]any)["danmaku"].(float64))),
		Url:          "https://www.bilibili.com/video/" + bid,
	}
	return data
}

func (*Bili) iToS(i int64) string {
	if i >= 10000 {
		return fmt.Sprintf("%.1f万", float64(i)/10000)
	}
	return fmt.Sprintf("%d", i)
}

func (b *Bili) getLiveData(roomId string) *LiveData {
	const api = "https://api.live.bilibili.com/xlive/web-room/v1/index/getInfoByRoom?room_id="
	url := api + roomId
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Printf("Live url parser request error: %v", err)
		return nil
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("Live url parser response error: %v", err)
		return nil
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Printf("Live url parser close error: %v", err)
		}
	}(resp.Body)
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Live url parser read body error: %v", err)
		return nil
	}

	var i any
	err = json.Unmarshal(body, &i)
	if err != nil {
		log.Printf("Live url parser unmarshal error: %v", err)
		return nil
	}
	ctx := i.(map[string]any)
	if ctx["code"].(float64) != 0 {
		return nil
	}

	data := &LiveData{
		Title:        ctx["data"].(map[string]any)["room_info"].(map[string]any)["title"].(string),
		User:         ctx["data"].(map[string]any)["anchor_info"].(map[string]any)["base_info"].(map[string]any)["uname"].(string),
		ThumbnailUrl: ctx["data"].(map[string]any)["room_info"].(map[string]any)["keyframe"].(string),
	}

	if shortId := ctx["data"].(map[string]any)["room_info"].(map[string]any)["short_id"].(float64); shortId != 0 {
		data.RoomId = fmt.Sprintf("短号: %d", int64(shortId))
		data.Url = "https://live.bilibili.com/" + fmt.Sprintf("%d", int64(shortId))
	} else {
		data.RoomId = fmt.Sprintf("房间号: %d", int64(ctx["data"].(map[string]any)["room_info"].(map[string]any)["room_id"].(float64)))
		data.Url = "https://live.bilibili.com/" + fmt.Sprintf("%d", int64(ctx["data"].(map[string]any)["room_info"].(map[string]any)["room_id"].(float64)))
	}

	areaName := ctx["data"].(map[string]any)["room_info"].(map[string]any)["area_name"].(string)
	parentAreaName := ctx["data"].(map[string]any)["room_info"].(map[string]any)["parent_area_name"].(string)
	if areaName != parentAreaName {
		data.AreaName = parentAreaName + "-" + areaName
	} else {
		data.AreaName = parentAreaName
	}

	if ctx["data"].(map[string]any)["room_info"].(map[string]any)["live_status"].(float64) == 1 {
		data.Status = "直播中	" + b.iToS(int64(ctx["data"].(map[string]any)["room_info"].(map[string]any)["online"].(float64))) + "人气"
	} else {
		data.Status = "未开播"
	}
	return data
}
