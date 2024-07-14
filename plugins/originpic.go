package plugins

import (
	"MacArthurGo/base"
	"MacArthurGo/plugins/essentials"
	"MacArthurGo/structs"
	"MacArthurGo/structs/cqcode"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type OriginPic struct{}

func init() {
	originPic := OriginPic{}
	plugin := &essentials.Plugin{
		Name:      "原图",
		Enabled:   base.Config.Plugins.OriginPic.Enable,
		Args:      base.Config.Plugins.OriginPic.Args,
		Interface: &originPic,
	}

	essentials.PluginArray = append(essentials.PluginArray, plugin)
	go originPic.deleteCache()
}

func (o *OriginPic) ReceiveAll() *[]byte {
	return nil
}

func (o *OriginPic) ReceiveMessage(messageStruct *structs.MessageStruct) *[]byte {
	if !essentials.CheckArgumentArray(&messageStruct.Message, &base.Config.Plugins.OriginPic.Args) {
		return nil
	}

	message := messageStruct.Message
	if message == nil {
		return nil
	}

	var reply []cqcode.ArrayMessage
	for _, m := range message {
		if m.Type == "image" {
			fileUrl, _ := essentials.GetUniversalImgURL(m.Data["url"].(string))
			reply = append(reply, *cqcode.Image(fileUrl))
		}
		if m.Type == "reply" {
			echo := fmt.Sprintf("originPic|%d", messageStruct.MessageId)
			value := essentials.EchoCache{Value: *messageStruct, Time: time.Now().Unix()}
			essentials.SetCache(strconv.FormatInt(messageStruct.MessageId, 10), value)
			return essentials.SendAction("get_msg", structs.GetMsg{Id: m.Data["id"].(string)}, echo)
		}
	}

	if len(reply) > 0 {
		return essentials.SendMsg(messageStruct, "", &reply, false, false, "")
	}
	return nil
}

func (o *OriginPic) ReceiveEcho(echoMessageStruct *structs.EchoMessageStruct) *[]byte {
	if echoMessageStruct.Status != "ok" {
		return nil
	}

	echo := echoMessageStruct.Echo
	split := strings.Split(echo, "|")

	if split[0] == "originPic" {
		contexts := echoMessageStruct.Data
		message := contexts.Message
		if message == nil {
			return nil
		}

		value, ok := essentials.GetCache(split[1])
		if !ok {
			log.Println("Origin picture cache not found")
			return nil
		}
		messageStruct := value.(essentials.EchoCache).Value

		for _, m := range message {
			if m.Type == "image" {
				imgUrl, _ := essentials.GetUniversalImgURL(m.Data["url"].(string))
				imgType, err := o.getFileType(imgUrl)
				if err != nil {
					log.Printf("Image type error: %v", err)
					continue
				}
				if imgType == "gif" {
					filePath, err := o.downloadGIF(imgUrl)
					if err != nil {
						log.Printf("Download gif error: %v", err)
						continue
					}
					return essentials.SendFile(&messageStruct, filePath, fmt.Sprintf("%d.gif", messageStruct.MessageId))
				} else {
					return essentials.SendMsg(&messageStruct, "", &[]cqcode.ArrayMessage{*cqcode.Image(imgUrl)}, false, false, "")
				}
			}
		}
	}
	return nil
}

func (o *OriginPic) getFileType(url string) (string, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
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

func (o *OriginPic) downloadGIF(url string) (string, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	body := resp.Body
	defer func(Body io.ReadCloser) {
		err = Body.Close()
		if err != nil {
			log.Printf("Image fetch close error: %v", err)
		}
	}(body)
	imgData, err := io.ReadAll(body)
	if err != nil {
		return "", err
	}
	imagePath := filepath.Join(".", "img_cache")
	if _, err := os.Stat(imagePath); os.IsNotExist(err) {
		err = os.Mkdir(imagePath, os.ModeDir|0755)
		if err != nil {
			log.Fatalf("Can not create log folder error: %v", err)
		}
	}
	file, err := os.Create(filepath.Join(imagePath, fmt.Sprintf("%d.gif", time.Now().Unix())))
	if err != nil {
		return "", err
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			log.Printf("Image file close error: %v", err)
		}
	}(file)
	_, err = file.Write(imgData)
	if err != nil {
		return "", err
	}
	filePath, err := filepath.Abs(file.Name())
	if err != nil {
		return "", err
	}

	return filePath, nil
}

func (o *OriginPic) deleteCache() {
	for {
		time.Sleep(1 * time.Hour)
		imagePath := filepath.Join(".", "img_cache")
		if _, err := os.Stat(imagePath); os.IsNotExist(err) {
			continue
		}
		files, err := os.ReadDir(imagePath)
		if err != nil {
			log.Printf("Read directory error: %v", err)
			continue
		}
		for _, f := range files {
			createTime, err := strconv.ParseInt(strings.TrimSuffix(f.Name(), filepath.Ext(f.Name())), 10, 64)
			if err != nil {
				log.Printf("Parse file name error: %v", err)
				continue
			}
			if time.Now().Unix()-createTime > 1800 {
				err := os.Remove(filepath.Join(imagePath, f.Name()))
				if err != nil {
					log.Printf("Remove file error: %v", err)
				}
			}
		}
	}
}
