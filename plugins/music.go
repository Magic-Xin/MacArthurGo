package plugins

import (
	"MacArthurGo/base"
	"MacArthurGo/plugins/essentials"
	"MacArthurGo/structs"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"regexp"
)

type Music struct{}

type musicInfo struct {
	Songs []struct {
		Name string `json:"name"`
		Ar   []struct {
			Name string `json:"name"`
		} `json:"ar"`
		Al struct {
			PicURL string `json:"picUrl"`
		} `json:"al"`
	} `json:"songs"`
}

func init() {
	plugin := &essentials.Plugin{
		Name:      "音乐链接解析",
		Enabled:   base.Config.Plugins.Music.Enable,
		Interface: &Music{},
	}
	essentials.PluginArray = append(essentials.PluginArray, plugin)
}

func (m *Music) ReceiveAll() *[]byte {
	return nil
}

func (m *Music) ReceiveMessage(messageStruct *structs.MessageStruct) *[]byte {
	var url string
	message := messageStruct.Message
	if message == nil {
		return nil
	}

	for _, msg := range message {
		if msg.Type == "text" && msg.Data["text"] != nil {
			str := msg.Data["text"].(string)
			if match := regexp.MustCompile(`(https?://music.163.com/song\?id=\d+)`).FindAllStringSubmatch(str, -1); match != nil {
				url = match[0][1]
			} else if match = regexp.MustCompile(`(https?://163cn.tv/\w+)|(https?://y.music.163.com/m/song/\d+)`).FindAllStringSubmatch(str, -1); match != nil {
				if res := essentials.GetOriginUrl(match[0][1]); res != nil {
					url = *res
				}
			}
		}
	}

	if url != "" {
		if match := regexp.MustCompile(`id=(\d+)`).FindAllStringSubmatch(url, -1); match != nil {
			info := m.getNeteaseMusicInfo(match[0][1])
			if info != nil {
				if info.Songs == nil || len(info.Songs) == 0 {
					return nil
				}
				var artists string
				for _, ar := range info.Songs[0].Ar {
					if ar.Name != "" {
						if artists != "" {
							artists += " / "
						}
						artists += ar.Name
					}
				}
				return essentials.SendMusic(messageStruct, "163", url, url, info.Songs[0].Name, artists, info.Songs[0].Al.PicURL)
			}
		}
	}

	return nil
}

func (m *Music) ReceiveEcho(*structs.EchoMessageStruct) *[]byte {
	return nil
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

func (*Music) getNeteaseMusicInfo(id string) *musicInfo {
	const api = "https://docs-neteasecloudmusicapi.vercel.app/song/detail?ids="

	req, err := http.NewRequest("GET", api+id, nil)
	if err != nil {
		log.Printf("Music parser request error: %v", err)
		return nil
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("Music parser response error: %v", err)
		return nil
	}
	defer func(Body io.ReadCloser) {
		err := resp.Body.Close()
		if err != nil {
			log.Printf("Music parser close body error: %v", err)
		}
	}(resp.Body)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Music parser read body error: %v", err)
		return nil
	}

	var info musicInfo
	err = json.Unmarshal(body, &info)
	if err != nil {
		log.Printf("Music parser unmarshal error: %v", err)
		return nil
	}

	return &info
}
