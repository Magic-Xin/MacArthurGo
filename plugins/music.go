package plugins

import (
	"MacArthurGo/base"
	"MacArthurGo/internal/urlmatch"
	"MacArthurGo/plugins/essentials"
	"MacArthurGo/structs"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
)

type Music struct{}

var (
	neteaseShortPattern  = regexp.MustCompile(`((http|https)://163cn\.tv/\w+)`)
	qqShortPattern       = regexp.MustCompile(`((http|https)://c6\.y\.qq\.com/\S+)`)
	qqMusicMIDPattern    = regexp.MustCompile(`songmid=(\w+)&`)
	qqMusicScriptPattern = regexp.MustCompile(`<script>(.+)</script>`)
	qqMusicIDPattern     = regexp.MustCompile(`"id":(\d+)`)
)

func registerMusic() error {
	plugin := &essentials.Plugin{
		Name:    "音乐链接解析",
		Enabled: base.Config.Plugins.Music.Enable,
		Handler: &Music{},
	}
	return essentials.Register(plugin)
}

func (m *Music) ReceiveMessage(messageStruct *structs.MessageStruct, send chan<- []byte) {
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
			} else if match := neteaseShortPattern.FindStringSubmatch(str); match != nil {
				if url := essentials.GetOriginUrl(match[1]); url != nil {
					urlType = "163"
					res = *url
				}
			} else if match := qqShortPattern.FindStringSubmatch(str); match != nil {
				if url := essentials.GetOriginUrl(match[1]); url != nil {
					urlType = "qq"
					if id := m.getQQMusicID(url); id != nil {
						res = "id=" + *id + "&"
					}
				}
			} else if songID, ok := urlmatch.NeteaseMobileSongID(str); ok {
				urlType = "163"
				res = "id=" + songID + "&"
			}
		}
	}

	if urlType != "" {
		if songID, ok := urlmatch.MusicQueryID(res); ok {
			send <- essentials.SendMusic(messageStruct, urlType, songID)
		}
	}
}

func (*Music) ReceiveEcho(*structs.EchoMessageStruct, chan<- []byte) {}

func (*Music) getQQMusicID(url *string) *string {
	if mid := qqMusicMIDPattern.FindStringSubmatch(*url); mid != nil {
		req, err := http.NewRequest("GET", "https://y.qq.com/n/ryqq/songDetail/"+mid[1], nil)
		if err != nil {
			log.Printf("Music parser request error: %v", err)
			return nil
		}

		resp, err := essentials.HTTPClient.Do(req)
		if err != nil {
			log.Printf("Music parser response error: %v", err)
			return nil
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			log.Printf("Music parser returned %s", resp.Status)
			return nil
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Printf("Music parser read body error: %v", err)
			return nil
		}

		if script := qqMusicScriptPattern.FindStringSubmatch(string(body)); script != nil {
			if id := qqMusicIDPattern.FindStringSubmatch(script[1]); id != nil {
				return &id[1]
			}
		}

	}
	return nil
}
