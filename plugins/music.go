package plugins

import (
	"MacArthurGo/plugins/essentials"
	"github.com/gookit/config/v2"
	"io"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

type Music struct {
	essentials.Plugin
}

func init() {
	music := Music{
		essentials.Plugin{
			Name:    "音乐链接解析",
			Enabled: config.Bool("plugins.music.enable"),
		},
	}
	essentials.PluginArray = append(essentials.PluginArray, &essentials.PluginInterface{Interface: &music})
}

func (m *Music) ReceiveAll(_ *map[string]any, _ *chan []byte) {}

func (m *Music) ReceiveMessage(ctx *map[string]any, send *chan []byte) {
	if !m.Enabled {
		return
	}

	var (
		urlType string
		res     string
	)
	message := essentials.DecodeArrayMessage(ctx)
	if message == nil {
		return
	}

	for _, msg := range *message {
		if msg.Type == "text" && msg.Data["text"] != nil {
			str := msg.Data["text"].(string)
			if strings.Contains(str, "https://music.163.com/") {
				urlType = "163"
				res = str
			} else if strings.Contains(str, "https://i.y.qq.com/") {
				urlType = "qq"
				res = str
			} else if match := regexp.MustCompile(`(http://163cn.tv/\w+)`).FindAllStringSubmatch(str, -1); match != nil {
				if url := essentials.GetOriginUrl(match[0][1]); url != nil {
					urlType = "163"
					res = *url
				}
			} else if match = regexp.MustCompile(`(https://c6.y.qq.com/\S+)`).FindAllStringSubmatch(str, -1); match != nil {
				if url := essentials.GetOriginUrl(match[0][1]); url != nil {
					urlType = "qq"
					res = "id=" + *m.getQQMusicID(url) + "&"
				}
			} else if match = regexp.MustCompile(`https://y.music.163.com/m/song/(\d+)`).FindAllStringSubmatch(str, -1); match != nil {
				urlType = "163"
				res = "id=" + match[0][1] + "&"
			}
		}
	}

	if urlType != "" {
		match := regexp.MustCompile(`id=(\d+)&`).FindAllStringSubmatch(res, -1)
		if match != nil {
			id, err := strconv.ParseInt(match[0][1], 10, 64)
			if err == nil {
				*send <- *essentials.SendMusic(ctx, urlType, id)
			}
		}
	}
}

func (m *Music) ReceiveEcho(_ *map[string]any, _ *chan []byte) {}

func (*Music) getQQMusicID(url *string) *string {
	if mid := regexp.MustCompile(`songmid=(\w+)&`).FindAllStringSubmatch(*url, -1); mid != nil {
		req, err := http.NewRequest("GET", "https://y.qq.com/n/ryqq/songDetail/"+mid[0][1], nil)
		if err != nil {
			log.Printf("Music parser request error: %v", err)
			return nil
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			log.Printf("Music parser response error: %v", err)
			return nil
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Printf("Music parser read body error: %v", err)
			return nil
		}

		if script := regexp.MustCompile(`<script>(.+)</script>`).FindAllStringSubmatch(string(body), -1); script != nil {
			if id := regexp.MustCompile(`"id":(\d+)`).FindAllStringSubmatch(script[0][1], -1); id != nil {
				return &id[0][1]
			}
		}

	}
	return nil
}
