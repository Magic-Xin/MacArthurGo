package plugins

import (
	"MacArthurGo/base"
	"MacArthurGo/plugins/essentials"
	"MacArthurGo/structs"
	"MacArthurGo/structs/cqcode"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
)

type OriginPic struct {
	essentials.Plugin
}

func init() {
	originPic := OriginPic{
		essentials.Plugin{
			Name:    "原图",
			Enabled: base.Config.Plugins.OriginPic.Enable,
			Args:    base.Config.Plugins.OriginPic.Args,
		},
	}
	essentials.PluginArray = append(essentials.PluginArray, &essentials.PluginInterface{Interface: &originPic})
}

func (o OriginPic) ReceiveAll(*map[string]any, *chan []byte) {}

func (o OriginPic) ReceiveMessage(ctx *map[string]any, send *chan []byte) {
	if !o.Enabled {
		return
	}

	if !essentials.CheckArgumentArray(ctx, &o.Args) {
		return
	}

	message := essentials.DecodeArrayMessage(ctx)
	if message == nil {
		return
	}

	var reply []cqcode.ArrayMessage
	for _, m := range *message {
		if m.Type == "image" {
			fileUrl, _ := essentials.GetUniversalImgURL(m.Data["url"].(string))
			reply = append(reply, *cqcode.Image(fileUrl))
		}
		if m.Type == "reply" {
			*send <- *essentials.SendAction("get_msg", structs.GetMsg{Id: m.Data["id"].(string)}, "originPic")
			return
		}
	}

	if len(reply) > 0 {
		*send <- *essentials.SendMsg(ctx, "", &reply, false, false)
	}
}

func (o OriginPic) ReceiveEcho(ctx *map[string]any, send *chan []byte) {
	if !o.Enabled {
		return
	}

	if (*ctx)["status"].(string) != "ok" {
		return
	}

	echo := (*ctx)["echo"].(string)
	split := strings.Split(echo, "|")

	if split[0] == "originPic" {
		contexts := (*ctx)["data"].(map[string]any)
		message := essentials.DecodeArrayMessage(&contexts)
		if message == nil {
			return
		}

		for _, m := range *message {
			if m.Type == "image" {
				originUrl := *essentials.GetOriginUrl(m.Data["url"].(string))
				imgType, err := o.getFileType(originUrl)
				if err != nil {
					log.Printf("Image type error: %v", err)
					continue
				}
				if imgType == "gif" {
					params := struct {
						Url string `json:"url"`
					}{Url: originUrl}
					echo := "originPicFile|" + contexts["message_type"].(string)
					if contexts["message_type"].(string) == "group" {
						echo += "|" + strconv.FormatInt(int64(contexts["group_id"].(float64)), 10)
					} else {
						echo += "|" + strconv.FormatInt(int64(contexts["user_id"].(float64)), 10)
					}
					echo += "|" + m.Data["file"].(string)

					*send <- *essentials.SendAction("download_file", params, echo)
				} else {
					*send <- *essentials.SendMsg(&contexts, "", &[]cqcode.ArrayMessage{*cqcode.Image(originUrl)}, false, false)
				}
			}
		}
	} else if split[0] == "originPicFile" {
		id, _ := strconv.ParseFloat(split[2], 64)
		contexts := &map[string]any{
			"message_type": split[1],
			"sender": map[string]any{
				"user_id": id,
			},
			"group_id": id,
		}
		file := (*ctx)["data"].(map[string]any)["file"].(string)
		*send <- *essentials.SendFile(contexts, file, split[3]+".gif")
	}
}

func (o OriginPic) getFileType(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer func(Body io.ReadCloser) {
		err = Body.Close()
		if err != nil {
			log.Printf("Image fetch close error: %v", err)
		}
	}(resp.Body)

	imgData, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	switch imgType := http.DetectContentType(imgData); imgType {
	case "image/jpeg":
		return "jpeg", nil
	case "image/png":
		return "png", nil
	case "image/gif":
		return "gif", nil
	default:
		return "", errors.New("unsupported image type")
	}
}
