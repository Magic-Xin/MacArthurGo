package plugins

import (
	"MacArthurGo/plugins/essentials"
	_struct "MacArthurGo/structs"
	"MacArthurGo/structs/cqcode"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/FloatTech/floatbox/web"
	xpath "github.com/antchfx/htmlquery"
	"github.com/gookit/config/v2"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

type PicSearch struct {
	essentials.Plugin
	groupForward      bool
	allowPrivate      bool
	handleBannedHosts bool
	searchFeedback    string
	sauceNAOToken     string
}

func init() {
	pSearch := PicSearch{
		Plugin: essentials.Plugin{
			Name:    "搜图",
			Enabled: config.Bool("plugins.picSearch.enable"),
			Args:    config.Strings("plugins.picSearch.args"),
		},
		groupForward:      config.Bool("plugins.picSearch.groupForward"),
		allowPrivate:      config.Bool("plugins.picSearch.allowPrivate"),
		handleBannedHosts: config.Bool("plugins.picSearch.handleBannedHosts"),
		searchFeedback:    config.String("plugins.picSearch.searchFeedback"),
		sauceNAOToken:     config.String("plugins.picSearch.sauceNAOToken"),
	}

	sqlTable := `CREATE TABLE IF NOT EXISTS picsearch(uid TEXT PRIMARY KEY NOT NULL, res TEXT NOT NULL, created NUMERIC NOT NULL);`
	_, err := essentials.DB.Exec(sqlTable)
	if err != nil {
		log.Printf("SQL exec error: %v", err)
		return
	}

	essentials.PluginArray = append(essentials.PluginArray, &essentials.PluginInterface{Interface: &pSearch})
	go essentials.DeleteExpired("DELETE FROM picsearch WHERE (? - created) > ?", config.Int64("plugins.picSearch.expirationTime"), config.Int64("plugins.picSearch.intervalTime"))
}

func (p *PicSearch) ReceiveAll(_ *map[string]any, _ *chan []byte) {}

func (p *PicSearch) ReceiveMessage(ctx *map[string]any, send *chan []byte) {
	if !p.Enabled {
		return
	}

	if (*ctx)["message_type"].(string) == "group" {
		if p.checkArgs(ctx) {
			p.picSearch(ctx, send, false, true)
		}
	} else if p.allowPrivate {
		if p.checkArgs(ctx) {
			p.picSearch(ctx, send, false, false)
		} else if !strings.HasPrefix(essentials.SplitArgument(ctx)[0], "/") {
			p.picSearch(ctx, send, false, false)
		}

	}
}

func (p *PicSearch) ReceiveEcho(ctx *map[string]any, send *chan []byte) {
	if !p.Enabled {
		return
	}

	echo := (*ctx)["echo"].(string)
	split := strings.Split(echo, "|")

	if split[0] == "picSearch" && (*ctx)["data"] != nil {
		contexts := (*ctx)["data"].(map[string]any)
		p.picSearch(&contexts, send, true, contexts["message_type"].(string) == "group")
	} else if (*ctx)["status"].(string) == "failed" {
		if split[0] == "picForward" {
			p.SecondTimesGroupForward(send, split[1:])
		} else if split[0] == "picFailed" {
			p.groupFailed(send, split[1:])
		}
	}
}

func (p *PicSearch) SecondTimesGroupForward(send *chan []byte, echo []string) {
	id, _ := strconv.ParseFloat(echo[2], 64)
	ctx := &map[string]any{
		"message_type": echo[1],
		"sender": map[string]any{
			"user_id": id,
		},
		"group_id": id,
	}

	selectRes := essentials.SelectDB("SELECT res FROM picsearch WHERE uid=?", echo[0])
	if selectRes == nil {
		*send <- *essentials.SendMsg(ctx, "数据库查询失败，搜图结果丢失", nil, false, false)
		return
	} else if len(*selectRes) == 0 {
		*send <- *essentials.SendMsg(ctx, "数据库查询失败，搜图结果丢失", nil, false, false)
		return
	}

	res := (*selectRes)[0]["res"].(string)
	result := append([]string{"sauceNAO 搜索结果被 QQ 拦截，已舍弃"}, strings.Split(res, "|")...)

	var data []_struct.ForwardNode
	for _, r := range result {
		if !strings.Contains(r, "SauceNAO") {
			data = append(data, *essentials.ConstructForwardNode(&[]cqcode.ArrayMessage{*cqcode.Text(r)}))
		}
	}
	if echo[1] == "group" {
		*send <- *essentials.SendGroupForward(ctx, &data, *p.genEcho(ctx, echo[0], true))
	} else {
		*send <- *essentials.SendPrivateForward(ctx, &data, *p.genEcho(ctx, echo[0], true))
	}
}

func (p *PicSearch) groupFailed(send *chan []byte, echo []string) {
	id, _ := strconv.ParseFloat(echo[2], 64)
	ctx := &map[string]any{
		"message_type": echo[1],
		"sender": map[string]any{
			"user_id": id,
		},
		"group_id": id,
	}

	*send <- *essentials.SendMsg(ctx, "合并转发失败，将独立发送搜索结果", nil, false, false)

	selectRes := essentials.SelectDB("SELECT res FROM picsearch WHERE uid=?", echo[0])
	if selectRes == nil {
		*send <- *essentials.SendMsg(ctx, "数据库查询失败，搜图结果丢失", nil, false, false)
		return
	} else if len(*selectRes) == 0 {
		*send <- *essentials.SendMsg(ctx, "数据库查询失败，搜图结果丢失", nil, false, false)
		return
	}

	res := (*selectRes)[0]["res"].(string)
	result := strings.Split(res, "|")
	for _, r := range result {
		*send <- *essentials.SendMsg(ctx, r, nil, false, false)
	}
}

func (p *PicSearch) picSearch(ctx *map[string]any, send *chan []byte, isEcho bool, isGroup bool) {
	if !isGroup && !p.allowPrivate {
		return
	}

	var (
		key     string
		result  [][]cqcode.ArrayMessage
		isStart bool
		cached  bool
	)

	msg := essentials.DecodeArrayMessage(ctx)
	if msg == nil {
		return
	}

	start := time.Now()
	for _, c := range *msg {
		if c.Type == "image" {
			if !isStart {
				*send <- *essentials.SendMsg(ctx, p.searchFeedback, nil, false, true)
				isStart = true
			}
			fileUrl := c.Data["url"].(string)
			fileUrl, key = essentials.GetUniversalImgURL(fileUrl)
			selectRes := essentials.SelectDB("SELECT res FROM picsearch WHERE uid=?", key)
			if selectRes != nil {
				if len(*selectRes) > 0 {
					res := (*selectRes)[0]["res"].(string)
					cached = true
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

			for i := 0; i < 2; i++ {
				wg.Add(1)
				limiter <- true
				switch i {
				case 0:
					go p.sauceNAO(fileUrl, response, limiter, wg)
				case 1:
					go p.ascii2d(fileUrl, response, limiter, wg)
				}
			}
			wg.Wait()
			close(response)
			wgResponse.Wait()
		}
		if c.Type == "reply" && !isEcho {
			mid := int64(c.Data["id"].(float64))
			*send <- *essentials.SendAction("get_msg", _struct.GetMsg{Id: mid}, "picSearch")
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
				essentials.InsertDB("picsearch", &[]string{"uid", "res", "created"}, key, string(jsonMsg), time.Now().Unix())
			}
		}

		if p.groupForward {
			var data []_struct.ForwardNode
			for _, r := range result {
				data = append(data, *essentials.ConstructForwardNode(&r))
			}
			if isGroup {
				*send <- *essentials.SendGroupForward(ctx, &data, *p.genEcho(ctx, key, false))
			} else {
				*send <- *essentials.SendPrivateForward(ctx, &data, *p.genEcho(ctx, key, false))
			}
		} else {
			for _, r := range result {
				*send <- *essentials.SendMsg(ctx, "", &r, false, false)
			}
		}
	}
}

func (p *PicSearch) sauceNAO(img string, response chan []cqcode.ArrayMessage, limiter chan bool, wg *sync.WaitGroup) {
	defer wg.Done()
	const api = "https://saucenao.com/search.php?db=999&output_type=2&testmode=1&numres=1"

	reqUrl := api + "&api_key=" + p.sauceNAOToken + "&url=" + img
	client := web.NewTLS12Client()
	req, _ := http.NewRequest("GET", reqUrl, nil)
	resp, _ := client.Do(req)
	body, _ := io.ReadAll(resp.Body)
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Printf("SauceNAO response close error: %v", err)
		}
	}(resp.Body)

	var i any
	err := json.Unmarshal(body, &i)
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
		similarity, _ = strconv.ParseFloat(header["similarity"].(string), 64)
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
			essentials.HandleBannedHostsArray(&sourceUrl)
		}
		msg += sourceUrl + "\n"
	}
	if extUrl != "" {
		if p.handleBannedHosts {
			essentials.HandleBannedHostsArray(&extUrl)
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

	reqC, _ := http.NewRequest("POST", api, fromData)
	reqC.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqC.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 6.1; WOW64; rv:6.0) Gecko/20100101 Firefox/6.0")
	respC, err := client.Do(reqC)
	if err != nil {
		response <- []cqcode.ArrayMessage{*cqcode.Text(fmt.Sprintf("%v", err))}
		return
	}

	urlB := strings.ReplaceAll(respC.Request.URL.String(), "color", "bovw")
	reqB, _ := http.NewRequest("GET", urlB, nil)
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
					essentials.HandleBannedHostsArray(&Link)
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

func (p *PicSearch) checkArgs(ctx *map[string]any) bool {
	for _, arg := range p.Args {
		if strings.Contains((*ctx)["raw_message"].(string), arg) {
			return true
		}
	}
	return false
}

func (p *PicSearch) genEcho(ctx *map[string]any, key string, retry bool) *string {
	var res string

	if retry {
		res = "picFailed|" + key
	} else {
		res = "picForward|" + key
	}

	if (*ctx)["message_type"].(string) == "private" {
		res += "|private|" + strconv.FormatInt(int64((*ctx)["sender"].(map[string]any)["user_id"].(float64)), 10)
	} else {
		res += "|group|" + strconv.FormatInt(int64((*ctx)["group_id"].(float64)), 10)
	}

	return &res
}
