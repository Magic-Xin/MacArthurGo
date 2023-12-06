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
	expirationTime    int64
	intervalTime      int64
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
		expirationTime:    config.Int64("plugins.picSearch.expirationTime"),
		intervalTime:      config.Int64("plugins.picSearch.intervalTime"),
		searchFeedback:    config.String("plugins.picSearch.searchFeedback"),
		sauceNAOToken:     config.String("plugins.picSearch.sauceNAOToken"),
	}
	essentials.PluginArray = append(essentials.PluginArray, &essentials.PluginInterface{Interface: &pSearch})

	sqlTable := `CREATE TABLE IF NOT EXISTS picsearch(uid TEXT PRIMARY KEY NOT NULL, res TEXT NOT NULL, created NUMERIC NOT NULL);`
	_, err := essentials.DB.Exec(sqlTable)
	if err != nil {
		log.Printf("SQL exec error: %v", err)
	}

	go pSearch.deleteExpiration()
}

func (p *PicSearch) ReceiveAll(_ *map[string]any, _ *chan []byte) {}

func (p *PicSearch) ReceiveMessage(ctx *map[string]any, send *chan []byte) {
	if !p.Enabled || !p.checkArgs(ctx) {
		return
	}

	if (*ctx)["message_type"].(string) == "group" {
		p.picSearch(ctx, send, (*ctx)["raw_message"].(string), false, true)
	} else {
		p.picSearch(ctx, send, (*ctx)["raw_message"].(string), false, false)
	}
}

func (p *PicSearch) ReceiveEcho(ctx *map[string]any, send *chan []byte) {
	if !p.Enabled {
		return
	}

	if (*ctx)["data"] != nil {
		if (*ctx)["echo"].(string) == "picSearch" {
			*ctx = (*ctx)["data"].(map[string]any)
			p.picSearch(ctx, send, (*ctx)["message"].(string), true, (*ctx)["group"].(bool))
		}
	} else if (*ctx)["msg"] != nil && strings.Contains((*ctx)["echo"].(string), "picForward") {
		if (*ctx)["msg"].(string) == "SEND_MSG_API_ERROR" {
			p.groupFailed(send, strings.Split((*ctx)["echo"].(string), "|")[1:])
		}
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

	*send <- *essentials.SendMsg(ctx, "合并转发失败，将独立发送搜索结果", false)

	res := p.selectDB(echo[0])
	if res == nil {
		*send <- *essentials.SendMsg(ctx, "数据库查询失败，搜图结果丢失", false)
		return
	}

	result := strings.Split(*res, "|")
	for _, r := range result {
		*send <- *essentials.SendMsg(ctx, r, false)
	}
}

func (p *PicSearch) picSearch(ctx *map[string]any, send *chan []byte, msg string, isEcho bool, isGroup bool) {
	if !isGroup && !p.allowPrivate {
		return
	}

	var (
		key     string
		result  []string
		isStart bool
		cached  bool
	)
	cc := cqcode.FromStr(msg)
	start := time.Now()
	for _, c := range *cc {
		if c.Type == "image" {
			if !isStart {
				*send <- *essentials.SendMsg(ctx, p.searchFeedback, false)
				isStart = true
			}
			fileUrl := c.Data["url"].(string)
			fileUrl, key = essentials.GetUniversalImgURL(fileUrl)
			res := p.selectDB(key)
			if res != nil {
				cached = true
				result = append(result, "已查询到缓存")
				split := strings.Split(*res, "|")
				result = append(result, split[:len(split)-1]...)
				break
			}

			wg := &sync.WaitGroup{}
			wgResponse := &sync.WaitGroup{}
			limiter := make(chan bool, 10)
			response := make(chan string, 200)

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
			cqMid := c.Data["id"].(string)
			mid, err := strconv.Atoi(cqMid)
			if err != nil {
				continue
			}
			*send <- *essentials.SendAction("get_msg", _struct.GetMsg{Id: mid}, "picSearch")
			return
		}
	}
	end := time.Since(start)
	if result != nil {
		if p.handleBannedHosts {
			result = *essentials.HandleBannedHostsArray(&result)
		}
		if !cached {
			result = append(result, fmt.Sprintf("本次搜图总用时: %0.3fs", end.Seconds()))
			p.insertDB(key, strings.Join(result, "|"))
		}

		if p.groupForward {
			var data []_struct.ForwardNode
			for _, r := range result {
				data = append(data, *essentials.ConstructForwardNode(&r, essentials.Info.NickName, essentials.Info.UserId))
			}
			if isGroup {
				*send <- *essentials.SendGroupForward(ctx, &data, *p.genEcho(ctx, key, isGroup))
			} else {
				*send <- *essentials.SendPrivateForward(ctx, &data, *p.genEcho(ctx, key, isGroup))
			}
		} else {
			for _, r := range result {
				*send <- *essentials.SendMsg(ctx, r, false)
			}
		}
	}
}

func (p *PicSearch) sauceNAO(img string, response chan string, limiter chan bool, wg *sync.WaitGroup) {
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
		response <- fmt.Sprintf("%v", err)
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

	r := fmt.Sprintf("SauceNAO\n%s\n相似度: %.2f%%\n", cqcode.Image(thumbNail), similarity)
	if title != "" {
		r += "「" + author + "」"
		if author != "" {
			r += "/「" + author + "」"
		}
		r += "\n"
	}
	if sourceUrl != "" {
		r += sourceUrl + "\n"
	}
	if extUrl != "" {
		r += extUrl
	}
	response <- r
	<-limiter
}

func (p *PicSearch) ascii2d(img string, response chan string, limiter chan bool, wg *sync.WaitGroup) {
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
		response <- fmt.Sprintf("%v", err)
		return
	}

	urlB := strings.ReplaceAll(respC.Request.URL.String(), "color", "bovw")
	reqB, _ := http.NewRequest("GET", urlB, nil)
	reqB.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 6.1; WOW64; rv:6.0) Gecko/20100101 Firefox/6.0")
	respB, err := client.Do(reqB)
	if err != nil {
		response <- fmt.Sprintf("%v", err)
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
			response <- fmt.Sprintf("%v", err)
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

				response <- fmt.Sprintf("ascii2d %s\n%s\n%s %s\n「%s」/「%s」\n%s\nArthor:%s",
					checkType[i], cqcode.Image(Thumb), Info, Type, Name, AuthNm, Link, Author)
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

func (p *PicSearch) genEcho(ctx *map[string]any, key string, isGroup bool) *string {
	res := "picForward|" + key
	if !isGroup {
		res += "|private|" + strconv.FormatInt(int64((*ctx)["sender"].(map[string]any)["user_id"].(float64)), 10)
	} else {
		res += "|group|" + strconv.FormatInt(int64((*ctx)["group_id"].(float64)), 10)
	}

	return &res
}

func (p *PicSearch) insertDB(uid string, res string) {
	stmt, err := essentials.DB.Prepare("INSERT INTO picsearch(uid, res, created) values (?, ?, ?)")
	if err != nil {
		log.Printf("Database insert prepare error: %v", err)
		return
	}
	_, err = stmt.Exec(uid, res, time.Now().Unix())
	if err != nil {
		log.Printf("Database insert execution error: %v", err)
	}
}

func (p *PicSearch) selectDB(uid string) *string {
	query, err := essentials.DB.Query("SELECT res FROM picsearch WHERE uid=?", uid)
	if err != nil {
		log.Printf("Database query error: %v", err)
		return nil
	}

	var res string
	for query.Next() {
		err = query.Scan(&res)
		if err != nil {
			return nil
		}
	}

	if res == "" {
		return nil
	}

	return &res
}

func (p *PicSearch) deleteExpiration() {
	for {
		stmt, err := essentials.DB.Prepare("DELETE FROM picsearch WHERE uid=(SELECT uid FROM picsearch WHERE (?-created) > ?)")
		if err != nil {
			log.Printf("Database delete prepare error: %v", err)
			return
		}

		_, err = stmt.Exec(time.Now().Unix(), p.expirationTime)
		if err != nil {
			log.Printf("Database delete execution error: %v", err)
		}
		time.Sleep(time.Duration(p.intervalTime) * time.Second)
	}
}
