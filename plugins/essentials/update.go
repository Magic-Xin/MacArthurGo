package essentials

//import (
//	"MacArthurGo/base"
//	"MacArthurGo/structs"
//	"encoding/json"
//	"fmt"
//	"io"
//	"log"
//	"net/http"
//	"time"
//)
//
//type Update struct {
//	sendCache  *[]byte
//	version    string
//	uploadTime time.Time
//}
//
//func init() {
//	update := Update{}
//	plugin := &Plugin{
//		Name:      "update",
//		Enabled:   true,
//		Args:      []string{"/update"},
//		Interface: &update,
//	}
//	PluginArray = append(PluginArray, plugin)
//
//	if base.Config.UpdateUrl != "" && base.Config.UpdateInterval != 0 {
//		log.Println("Init update check goroutine...")
//		go update.UpdateWatcher()
//	}
//}
//
//func (u *Update) ReceiveAll(Send chan<- *[]byte) {
//	if u.sendCache != nil {
//		Send <- u.sendCache
//		u.sendCache = nil
//	}
//	return
//}
//
//func (u *Update) ReceiveMessage(messageStruct *structs.MessageStruct, Send chan<- *[]byte) {
//	if base.Config.UpdateUrl == "" {
//		return
//	}
//	if base.Branch == "Release" {
//		Send <- SendMsg(messageStruct, "暂时不支持 Release 版本自动更新", nil, false, false, "")
//		return
//	}
//
//	if messageStruct.Command != "/update" {
//		return
//	}
//
//	err := u.getVersion()
//	if err != nil {
//		Send <- SendMsg(messageStruct, fmt.Sprintf("获取最新版本失败: %v", err), nil, false, false, "")
//		return
//	}
//
//	message := []structs.ArrayMessage{*structs.Text("本地版本:\n分支: " + base.Branch + "\n" + "版本: " + base.Version + "\n" + "编译时间: " + base.BuildTime),
//		*structs.Text("\n\n最新版本 (dev):\n版本: " + u.version + "\n" + "上传时间: " + u.uploadTime.Format("2006-01-02 15:04:05"))}
//	if base.Version != u.version {
//		message = append(message, *structs.Text(fmt.Sprintf("\n\n有更新！\n请 admin 使用 /update %s 更新到最新版本\n注意：自动更新有风险，请确保可以手动处理未知问题", u.version)))
//	} else {
//		message = append(message, *structs.Text("\n\n版本一致，无需更新"))
//	}
//	Send <- SendMsg(messageStruct, "", &message, false, false, "")
//	return
//}
//
//func (*Update) ReceiveEcho(*structs.EchoMessageStruct, chan<- *[]byte) {}
//
//func (u *Update) UpdateWatcher() {
//	for {
//		err := u.getVersion()
//		if err != nil {
//			log.Printf("Get version error: %v", err)
//			continue
//		}
//		if base.Version != u.version {
//			sendStruct := structs.MessageStruct{
//				MessageType: "private",
//				UserId:      base.Config.Admin,
//			}
//
//			message := []structs.ArrayMessage{*structs.Text("检测到版本更新！\n\n本地版本:\n分支: " + base.Branch + "\n" + "版本: " + base.Version + "\n" + "编译时间: " + base.BuildTime),
//				*structs.Text("\n\n最新版本 (dev):\n版本: " + u.version + "\n" + "上传时间: " + u.uploadTime.Format("2006-01-02 15:04:05"))}
//			u.sendCache = SendMsg(&sendStruct, "", &message, false, false, "")
//		}
//		time.Sleep(time.Duration(base.Config.UpdateInterval) * time.Second)
//	}
//}
//
//func (u *Update) getVersion() error {
//	url := base.Config.UpdateUrl + "version.json"
//	req, err := http.NewRequest("GET", url, nil)
//	if err != nil {
//		log.Printf("Request version.json error: %v", err)
//		return err
//	}
//	resp, err := http.DefaultClient.Do(req)
//	if err != nil {
//		log.Printf("Get version.json error: %v", err)
//		return err
//	}
//	defer func(Body io.ReadCloser) {
//		err = Body.Close()
//		if err != nil {
//			log.Printf("Close body error: %v", err)
//		}
//	}(resp.Body)
//
//	body, err := io.ReadAll(resp.Body)
//	if err != nil {
//		log.Printf("Read body error: %v", err)
//		return err
//	}
//
//	var i any
//	err = json.Unmarshal(body, &i)
//	if err != nil {
//		log.Printf("Unmarshal body error: %v", err)
//		return err
//	}
//
//	uploadTime, err := time.Parse(time.RFC3339, i.(map[string]any)["upload_time"].(string))
//	if err != nil {
//		log.Printf("Parse upload time error: %v", err)
//		return err
//	}
//	tz, err := time.LoadLocation("Asia/Shanghai")
//	if err != nil {
//		tz = time.FixedZone("Asia/Shanghai", 8*60*60)
//	}
//	u.uploadTime = uploadTime.In(tz)
//	u.version = i.(map[string]any)["version"].(string)[:7]
//
//	return nil
//}
