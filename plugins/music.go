package plugins

import (
	"MacArthurGo/base"
	"MacArthurGo/plugins/essentials"
	"MacArthurGo/structs"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
)

type Music struct{}

func init() {
	plugin := &essentials.Plugin{
		Name:      "音乐链接解析",
		Enabled:   base.Config.Plugins.Music.Enable,
		Interface: &Music{},
	}
	essentials.PluginArray = append(essentials.PluginArray, plugin)
}

func (*Music) ReceiveAll(chan<- *[]byte) {}

func (m *Music) ReceiveMessage(messageStruct *structs.MessageStruct, send chan<- *[]byte) {
	var (
		urlType string
		res     string
	)
	message := messageStruct.Message
	if message == nil {
		return
	}

	for _, msg := range message {
		if msg.Type == "text" && msg.Data["text"] != nil {
			str := msg.Data["text"].(string)
			if strings.Contains(str, "//music.163.com/") {
				urlType = "163"
				res = str
			} else if strings.Contains(str, "//i.y.qq.com/") {
				urlType = "qq"
				res = str
			} else if match := regexp.MustCompile(`((http|https)://163cn.tv/\w+)`).FindAllStringSubmatch(str, -1); match != nil {
				if url := essentials.GetOriginUrl(match[0][1]); url != nil {
					urlType = "163"
					res = *url
				}
			} else if match = regexp.MustCompile(`((http|https)://c6.y.qq.com/\S+)`).FindAllStringSubmatch(str, -1); match != nil {
				if url := essentials.GetOriginUrl(match[0][1]); url != nil {
					urlType = "qq"
					if id := m.getQQMusicID(url); id != nil {
						res = "id=" + *id + "&"
					}
				}
			} else if match = regexp.MustCompile(`(http|https)://y.music.163.com/m/song/(\d+)`).FindAllStringSubmatch(str, -1); match != nil {
				urlType = "163"
				res = "id=" + match[0][2] + "&"
			}
		}
	}

	if urlType != "" {
		match := regexp.MustCompile(`id=(\d+)`).FindAllStringSubmatch(res, -1)
		if match != nil {
			send <- essentials.SendMusic(messageStruct, urlType, match[0][1])
		}
	}
	return
}

func (*Music) ReceiveEcho(*structs.EchoMessageStruct, chan<- *[]byte) {}

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
