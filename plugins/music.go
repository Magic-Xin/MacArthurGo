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

	var urlType string
	str := (*ctx)["raw_message"].(string)
	if strings.Contains(str, "music.163.com") {
		urlType = "163"
	} else if strings.Contains(str, "i.y.qq.com") {
		urlType = "qq"
	} else if match := regexp.MustCompile(`(http://163cn.tv/\w+)`).FindAllStringSubmatch(str, -1); match != nil {
		if url := m.getOriginUrl(match[0][1]); url != nil {
			urlType = "163"
			str = *url
		}
	} else if match := regexp.MustCompile(`(https://c6.y.qq.com/\S+)`).FindAllStringSubmatch(str, -1); match != nil {
		if url := m.getOriginUrl(match[0][1]); url != nil {
			urlType = "qq"
			str = "id=" + *m.getQQMusicID(url) + "&"
		}
	}

	if urlType != "" {
		re := regexp.MustCompile(`id=(\d+)&`)
		match := re.FindAllStringSubmatch(str, -1)
		if len(match) > 0 {
			id, err := strconv.ParseInt(match[0][1], 10, 64)
			if err == nil {
				*send <- *essentials.SendMusic(ctx, urlType, id)
			}
		}
	}
}

func (m *Music) ReceiveEcho(_ *map[string]any, _ *chan []byte) {}

func (*Music) getOriginUrl(url string) *string {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Printf("Music parser request error: %v", err)
		return nil
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("Music parser response error: %v", err)
		return nil
	}

	originURL := resp.Request.URL.String()
	return &originURL
}

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
