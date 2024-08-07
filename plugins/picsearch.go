package plugins

import (
	"MacArthurGo/base"
	"MacArthurGo/plugins/essentials"
	"MacArthurGo/structs"
	"MacArthurGo/structs/cqcode"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/FloatTech/floatbox/web"
	xpath "github.com/antchfx/htmlquery"
	"github.com/google/go-cmp/cmp"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

type PicSearch struct {
	groupForward      bool
	allowPrivate      bool
	handleBannedHosts bool
	sauceNAOToken     string
}

func init() {
	picSearch := PicSearch{
		groupForward:      base.Config.Plugins.PicSearch.GroupForward,
		allowPrivate:      base.Config.Plugins.PicSearch.AllowPrivate,
		handleBannedHosts: base.Config.Plugins.PicSearch.HandleBannedHosts,
		sauceNAOToken:     base.Config.Plugins.PicSearch.SauceNAOToken,
	}
	plugin := &essentials.Plugin{
		Name:      "搜图",
		Enabled:   base.Config.Plugins.PicSearch.Enable,
		Args:      base.Config.Plugins.PicSearch.Args,
		Interface: &picSearch,
	}
	essentials.PluginArray = append(essentials.PluginArray, plugin)

	key := &[]string{"uid", "res", "created"}
	value := &[]string{"TEXT PRIMARY KEY NOT NULL", "TEXT NOT NULL", "NUMERIC NOT NULL"}
	err := essentials.CreateDB("picSearch", key, value)

	if err != nil {
		log.Printf("Database picSearch create error: %v", err)
		return
	}

	go essentials.DeleteExpired("picSearch", "created", base.Config.Plugins.PicSearch.ExpirationTime, base.Config.Plugins.PicSearch.IntervalTime)
}

func (p *PicSearch) ReceiveAll() *[]byte {
	return nil
}

func (p *PicSearch) ReceiveMessage(messageStruct *structs.MessageStruct) *[]byte {
	rawMsg := messageStruct.RawMessage

	if messageStruct.MessageType == "group" {
		if p.checkArgs(rawMsg, &base.Config.Plugins.PicSearch.Args) {
			return p.picSearch(messageStruct, &messageStruct.Message, false, true, p.checkArgs(rawMsg, &[]string{"purge"}))
		}
	} else if p.allowPrivate {
		if p.checkArgs(rawMsg, &base.Config.Plugins.PicSearch.Args) {
			return p.picSearch(messageStruct, &messageStruct.Message, false, false, p.checkArgs(rawMsg, &[]string{"purge"}))
		} else {
			words := essentials.SplitArgument(&messageStruct.Message)
			if len(words) == 0 {
				return p.picSearch(messageStruct, &messageStruct.Message, false, false, p.checkArgs(rawMsg, &[]string{"purge"}))
			} else if !strings.HasPrefix(words[0], "/") {
				return p.picSearch(messageStruct, &messageStruct.Message, false, false, p.checkArgs(rawMsg, &[]string{"purge"}))
			}
		}

	}
	return nil
}

func (p *PicSearch) ReceiveEcho(echoMessageStruct *structs.EchoMessageStruct) *[]byte {
	echo := echoMessageStruct.Echo
	split := strings.Split(echo, "|")

	if split[0] == "picSearch" && !cmp.Equal(echoMessageStruct.Data, struct{}{}) {
		data := echoMessageStruct.Data
		msg := data.Message
		value, ok := essentials.GetCache(split[1])
		if !ok {
			log.Println("Pic search get cache error")
			return nil
		}
		originCtx := value.(essentials.EchoCache).Value

		if echoMessageStruct.Status == "failed" {
			return essentials.SendMsg(&originCtx, "搜图失败", nil, false, false, "")
		}

		if len(split) == 3 {
			return p.picSearch(&originCtx, &msg, true, originCtx.MessageType == "group", split[2] == "purge")
		} else {
			return p.picSearch(&originCtx, &msg, true, originCtx.MessageType == "group", false)
		}
	}
	return nil
}

func (p *PicSearch) picSearch(messageStruct *structs.MessageStruct, msg *[]cqcode.ArrayMessage, isEcho bool, isGroup bool, isPurge bool) *[]byte {
	if !isGroup && !p.allowPrivate {
		return nil
	}

	var (
		key    string
		result [][]cqcode.ArrayMessage
		cached bool
	)

	if msg == nil {
		return nil
	}

	start := time.Now()
	for _, c := range *msg {
		if c.Type == "image" {
			fileUrl := c.Data["url"].(string)
			fileUrl, key = essentials.GetUniversalImgURL(fileUrl)
			selectRes := essentials.SelectDB("picSearch", "res", fmt.Sprintf("uid='%s'", key))
			if selectRes != nil {
				if len(*selectRes) > 0 {
					cached = true
					if !isPurge {
						res := (*selectRes)[0]["res"].(string)
						result = append(result, []cqcode.ArrayMessage{*cqcode.Text("本次搜图结果来自数据库缓存")})
						var cachedMsg [][]cqcode.ArrayMessage
						err := json.Unmarshal([]byte(res), &cachedMsg)
						if err != nil {
							log.Printf("Unmarshal cached message error: %v", err)
							continue
						}
						result = append(result, cachedMsg[:len(cachedMsg)-1]...)
						continue
					}
				}
			}

			wg := &sync.WaitGroup{}
			wgResponse := &sync.WaitGroup{}
			limiter := make(chan bool, 10)
			response := make(chan []cqcode.ArrayMessage, 200)

			go func() {
				wgResponse.Add(1)
				for rc := range response {
					result = append(result, rc)
				}
				wgResponse.Done()
			}()

			wg.Add(2)
			limiter <- true
			go p.sauceNAO(fileUrl, response, limiter, wg)
			limiter <- true
			go p.ascii2d(fileUrl, response, limiter, wg)

			wg.Wait()
			close(response)
			wgResponse.Wait()
		}
		if c.Type == "reply" && !isEcho {
			value := essentials.EchoCache{Value: *messageStruct, Time: time.Now().Unix()}
			essentials.SetCache(strconv.FormatInt(messageStruct.MessageId, 10), value)
			echo := fmt.Sprintf("picSearch|%d", messageStruct.MessageId)
			if isPurge {
				echo += "|purge"
			}
			return essentials.SendAction("get_msg", structs.GetMsg{Id: c.Data["id"].(string)}, echo)
		}
	}
	end := time.Since(start)

	if result != nil {
		if !cached {
			result = append(result, []cqcode.ArrayMessage{*cqcode.Text(fmt.Sprintf("本次搜图总用时: %0.3fs", end.Seconds()))})
			jsonMsg, err := json.Marshal(result)
			if err != nil {
				log.Printf("Search result mashal error: %v", err)
			} else {
				err = essentials.InsertDB("picSearch", &[]string{"uid", "res", "created"},
					&[]string{key, string(jsonMsg), strconv.FormatInt(time.Now().Unix(), 10)})
				if err != nil {
					log.Printf("Insert picSearch error: %v", err)
				}
			}
		} else if isPurge {
			result = append(result, []cqcode.ArrayMessage{*cqcode.Text(fmt.Sprintf("本次搜图总用时: %0.3fs", end.Seconds()))})
			jsonMsg, err := json.Marshal(result)
			if err != nil {
				log.Printf("Search result mashal error: %v", err)
			} else {
				err = essentials.UpdateDB("picSearch", "uid", key, &[]string{"res", "created"}, &[]string{string(jsonMsg), strconv.FormatInt(time.Now().Unix(), 10)})
				if err != nil {
					log.Printf("Update picSearch error: %v", err)
				}
			}
		}

		if p.groupForward {
			var data []structs.ForwardNode
			for _, r := range result {
				data = append(data, *essentials.ConstructForwardNode(essentials.Info.UserId, essentials.Info.NickName, &r))
			}
			if isGroup {
				return essentials.SendGroupForward(messageStruct, &data, *p.genEcho(messageStruct, key, false))
			} else {
				return essentials.SendPrivateForward(messageStruct, &data, *p.genEcho(messageStruct, key, false))
			}
		} else {
			for _, r := range result {
				return essentials.SendMsg(messageStruct, "", &r, false, false, "")
			}
		}
	}
	return nil
}

func (p *PicSearch) sauceNAO(img string, response chan []cqcode.ArrayMessage, limiter chan bool, wg *sync.WaitGroup) {
	defer wg.Done()
	const api = "https://saucenao.com/search.php?db=999&output_type=2&testmode=1&numres=1"

	reqUrl := api + "&api_key=" + p.sauceNAOToken + "&url=" + img
	client := web.NewTLS12Client()
	req, err := http.NewRequest("GET", reqUrl, nil)
	if err != nil {
		response <- []cqcode.ArrayMessage{*cqcode.Text(fmt.Sprintf("%v", err))}
		return
	}
	resp, err := client.Do(req)
	if err != nil {
		response <- []cqcode.ArrayMessage{*cqcode.Text(fmt.Sprintf("%v", err))}
		return
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		response <- []cqcode.ArrayMessage{*cqcode.Text(fmt.Sprintf("%v", err))}
		return
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Printf("SauceNAO response close error: %v", err)
		}
	}(resp.Body)

	var i any
	err = json.Unmarshal(body, &i)
	if err != nil {
		response <- []cqcode.ArrayMessage{*cqcode.Text(fmt.Sprintf("%v", err))}
		return
	}

	ctx := i.(map[string]any)
	var (
		similarity float64
		thumbNail  string
		author     string
		title      string
		sourceUrl  string
		extUrl     string
	)
	if ctx["results"] != nil {
		results := ctx["results"].([]any)[0]
		header := results.(map[string]any)["header"].(map[string]any)
		data := results.(map[string]any)["data"].(map[string]any)
		similarity, err = strconv.ParseFloat(header["similarity"].(string), 64)
		if err != nil {
			response <- []cqcode.ArrayMessage{*cqcode.Text(fmt.Sprintf("%v", err))}
			return
		}
		thumbNail = header["thumbnail"].(string)
		if data["member_name"] != nil {
			author = data["member_name"].(string)
		} else if data["author_name"] != nil {
			author = data["author_name"].(string)
		}
		if data["title"] != nil {
			title = data["title"].(string)
		}
		if data["source"] != nil {
			sourceUrl = data["source"].(string)
		}
		if data["ext_urls"] != nil {
			extUrl = data["ext_urls"].([]any)[0].(string)
		}
	}
	r := []cqcode.ArrayMessage{*cqcode.Text("SauceNAO\n")}
	r = append(r, *cqcode.Image(thumbNail))
	msg := fmt.Sprintf("\n相似度: %.2f%%\n", similarity)
	if title != "" {
		msg += "「" + title + "」"
		if author != "" {
			msg += "/「" + author + "」"
		}
		msg += "\n"
	}
	if sourceUrl != "" {
		if p.handleBannedHosts {
			p.HandleBannedHostsArray(&sourceUrl)
		}
		msg += sourceUrl + "\n"
	}
	if extUrl != "" {
		if p.handleBannedHosts {
			p.HandleBannedHostsArray(&extUrl)
		}
		msg += extUrl
	}
	r = append(r, *cqcode.Text(msg))
	response <- r
	<-limiter
}

func (p *PicSearch) ascii2d(img string, response chan []cqcode.ArrayMessage, limiter chan bool, wg *sync.WaitGroup) {
	defer wg.Done()
	const api = "https://ascii2d.net/search/uri"
	client := web.NewTLS12Client()
	data := url.Values{}
	data.Set("uri", img) // 图片链接
	fromData := strings.NewReader(data.Encode())

	reqC, err := http.NewRequest("POST", api, fromData)
	if err != nil {
		response <- []cqcode.ArrayMessage{*cqcode.Text(fmt.Sprintf("%v", err))}
		return
	}
	reqC.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqC.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 6.1; WOW64; rv:6.0) Gecko/20100101 Firefox/6.0")
	respC, err := client.Do(reqC)
	if err != nil {
		response <- []cqcode.ArrayMessage{*cqcode.Text(fmt.Sprintf("%v", err))}
		return
	}

	urlB := strings.ReplaceAll(respC.Request.URL.String(), "color", "bovw")
	reqB, err := http.NewRequest("GET", urlB, nil)
	if err != nil {
		response <- []cqcode.ArrayMessage{*cqcode.Text(fmt.Sprintf("%v", err))}
		return
	}
	reqB.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 6.1; WOW64; rv:6.0) Gecko/20100101 Firefox/6.0")
	respB, err := client.Do(reqB)
	if err != nil {
		response <- []cqcode.ArrayMessage{*cqcode.Text(fmt.Sprintf("%v", err))}
		return
	}

	defer func() {
		err := respB.Body.Close()
		if err != nil {
			log.Printf("Picsearch response close error: %v", err)
			return
		}
		err = respC.Body.Close()
		if err != nil {
			log.Printf("Picsearch response close error: %v", err)
			return
		}
	}()

	checkType := []string{"色合検索", "特徴検索"}

	for i, resp := range []*http.Response{respC, respB} {
		doc, err := xpath.Parse(resp.Body)
		if err != nil {
			return
		}
		// 取出每个返回的结果
		list := xpath.Find(doc, `//div[@class="row item-box"]`)
		if len(list) == 0 {
			err := errors.New("ascii2d not found")
			response <- []cqcode.ArrayMessage{*cqcode.Text(fmt.Sprintf("%v", err))}
			return
		}
		for _, n := range list {
			linkPath := xpath.FindOne(n, `//div[2]/div[3]/h6/a[1]`)
			authPath := xpath.FindOne(n, `//div[2]/div[3]/h6/a[2]`)
			picPath := xpath.FindOne(n, `//div[1]/img`)
			typePath := xpath.FindOne(n, `//div[2]/div[3]/h6/small`)

			if linkPath != nil && authPath != nil && picPath != nil && typePath != nil {
				Info := xpath.InnerText(xpath.FindOne(list[0], `//div[2]/small`))
				Link := xpath.SelectAttr(linkPath, "href")
				Name := xpath.InnerText(linkPath)
				Author := xpath.SelectAttr(authPath, "href")
				AuthNm := xpath.InnerText(authPath)
				Thumb := "https://ascii2d.net" + xpath.SelectAttr(picPath, "src")
				Type := strings.Trim(xpath.InnerText(typePath), "\n")

				if p.handleBannedHosts {
					p.HandleBannedHostsArray(&Link)
				}

				r := []cqcode.ArrayMessage{*cqcode.Text(fmt.Sprintf("ascii2d %s\n", checkType[i]))}
				r = append(r, *cqcode.Image(Thumb))
				msg := fmt.Sprintf("\n%s %s\n「%s」/「%s」\n%s\nArthor:%s", Info, Type, Name, AuthNm, Link, Author)
				r = append(r, *cqcode.Text(msg))

				response <- r
				break
			}
		}
	}
	<-limiter
}

func (p *PicSearch) checkArgs(rawMsg string, args *[]string) bool {
	for _, arg := range *args {
		if match := regexp.MustCompile(`(` + arg + `$|` + arg + `\W)`).FindStringIndex(rawMsg); match != nil {
			return true
		}
	}
	return false
}

func (p *PicSearch) genEcho(messageStruct *structs.MessageStruct, key string, retry bool) *string {
	var res string

	if retry {
		res = "picFailed|" + key
	} else {
		res = "picForward|" + key
	}

	if messageStruct.MessageType == "private" {
		res += "|private|" + strconv.FormatInt(messageStruct.UserId, 10)
	} else {
		res += "|group|" + strconv.FormatInt(messageStruct.GroupId, 10)
	}

	return &res
}

func (p *PicSearch) HandleBannedHostsArray(str *string) {
	bannedHosts := []string{"danbooru.donmai.us", "konachan.com"}
	*str = strings.Replace(*str, "//", "//\u200B", -1)
	for _, host := range bannedHosts {
		*str = strings.Replace(*str, host, strings.Replace(host, ".", ".\u200B", -1), -1)
	}
	return
}
