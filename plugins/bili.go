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

type Data struct {
	Title        string
	Author       string
	ThumbnailUrl string
	Aid          string
	Playtime     string
	Danmaku      string
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

	biliShort := regexp.MustCompile(`"(https://b23.tv/\w+)`)
	bv := regexp.MustCompile(`https://www.bilibili.com/video/(\w+)`)

	rawMsg := (*ctx)["raw_message"].(string)
	var bvid string
	if match := biliShort.FindAllStringSubmatch(rawMsg, -1); match != nil {
		biliLong := bv.FindAllStringSubmatch(*essentials.GetOriginUrl(match[0][1]), -1)
		if biliLong != nil {
			bvid = biliLong[0][1]
		} else {
			return
		}
	} else if match = bv.FindAllStringSubmatch(rawMsg, -1); match != nil {
		bvid = match[0][1]
	} else {
		return
	}

	data := b.getBiliDate(bvid)
	if data != nil {
		var messageArray []cqcode.ArrayMessage
		messageArray = append(messageArray, *cqcode.Image(data.ThumbnailUrl + "\n"))
		messageArray = append(messageArray, *cqcode.Text(data.Aid + "\n"))
		messageArray = append(messageArray, *cqcode.Text(data.Title + "\n"))
		messageArray = append(messageArray, *cqcode.Text("UP: " + data.Author + "\n"))
		messageArray = append(messageArray, *cqcode.Text("播放: " + data.Playtime + "	弹幕: " + data.Danmaku + "\n"))
		messageArray = append(messageArray, *cqcode.Text("https://www.bilibili.com/video/" + bvid))
		*send <- *essentials.SendMsg(ctx, "", &messageArray, false, false)
	}
}

func (b *Bili) ReceiveEcho(_ *map[string]any, _ *chan []byte) {}

func (b *Bili) getBiliDate(bvid string) *Data {
	const api = "https://api.bilibili.com/x/web-interface/view?bvid="
	url := api + bvid
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Printf("Url parser request error: %v", err)
		return nil
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("Url parser response error: %v", err)
		return nil
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Printf("Url parser close error: %v", err)
		}
	}(resp.Body)
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Url parser read body error: %v", err)
		return nil
	}

	var i any
	err = json.Unmarshal(body, &i)
	if err != nil {
		log.Printf("Url parser unmarshal error: %v", err)
		return nil
	}

	ctx := i.(map[string]any)
	if ctx["code"].(float64) != 0 {
		return nil
	}
	data := &Data{
		Title:        ctx["data"].(map[string]any)["title"].(string),
		Author:       ctx["data"].(map[string]any)["owner"].(map[string]any)["name"].(string),
		ThumbnailUrl: ctx["data"].(map[string]any)["pic"].(string),
		Aid:          fmt.Sprintf("av%d", int64(ctx["data"].(map[string]any)["aid"].(float64))),
		Playtime:     b.iToS(int64(ctx["data"].(map[string]any)["stat"].(map[string]any)["view"].(float64))),
		Danmaku:      b.iToS(int64(ctx["data"].(map[string]any)["stat"].(map[string]any)["danmaku"].(float64))),
	}
	return data
}

func (*Bili) iToS(i int64) string {
	if i >= 10000 {
		return fmt.Sprintf("%.1f万", float64(i)/10000)
	}
	return fmt.Sprintf("%d", i)
}
