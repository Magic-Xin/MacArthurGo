package ascii2d

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync/atomic"
	"time"

	"github.com/antchfx/htmlquery"
)

var chromiumErrorCodePattern = regexp.MustCompile(`["']errorCode["']\s*:\s*["'](ERR_[A-Z0-9_]+)["']`)

const (
	defaultAPIURL          = "http://127.0.0.1:8191/v1"
	defaultSiteURL         = "https://ascii2d.net"
	colorSearchWaitSeconds = 5
	maxAPIResponse         = 16 << 20
)

type Config struct {
	APIURL   string
	ProxyURL string
	Timeout  time.Duration
}

type Client struct {
	apiURL       string
	proxyURL     string
	siteURL      string
	maxTimeoutMS int
	httpClient   *http.Client
	sessionID    atomic.Uint64
}

type Result struct {
	Mode       string
	Title      string
	Author     string
	URL        string
	AuthorURL  string
	Thumbnail  string
	ResultURL  string
	Info       string
	SourceType string
	cookies    []flareCookie
	userAgent  string
}

type flareRequest struct {
	Command       string      `json:"cmd"`
	URL           string      `json:"url,omitempty"`
	Session       string      `json:"session,omitempty"`
	MaxTimeout    int         `json:"maxTimeout,omitempty"`
	WaitInSeconds int         `json:"waitInSeconds,omitempty"`
	DisableMedia  bool        `json:"disableMedia,omitempty"`
	Proxy         *flareProxy `json:"proxy,omitempty"`
}

type flareProxy struct {
	URL string `json:"url"`
}

type flareResponse struct {
	Status   string        `json:"status"`
	Message  string        `json:"message"`
	Solution flareSolution `json:"solution"`
}

type flareSolution struct {
	URL       string        `json:"url"`
	Status    int           `json:"status"`
	Response  string        `json:"response"`
	Cookies   []flareCookie `json:"cookies"`
	UserAgent string        `json:"userAgent"`
}

type flareCookie struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Domain string `json:"domain"`
	Path   string `json:"path"`
	Secure bool   `json:"secure"`
}

func NewClient(cfg Config) (*Client, error) {
	apiURL := strings.TrimSpace(cfg.APIURL)
	if apiURL == "" {
		apiURL = defaultAPIURL
	}
	parsed, err := url.Parse(apiURL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, fmt.Errorf("invalid FlareSolverr URL %q", apiURL)
	}

	proxyURL := strings.TrimSpace(cfg.ProxyURL)
	if proxyURL != "" {
		parsedProxy, parseErr := url.Parse(proxyURL)
		if parseErr != nil || parsedProxy.Host == "" || (parsedProxy.Scheme != "http" && parsedProxy.Scheme != "https" && parsedProxy.Scheme != "socks4" && parsedProxy.Scheme != "socks5") {
			return nil, fmt.Errorf("invalid FlareSolverr proxy URL %q", proxyURL)
		}
	}

	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 90 * time.Second
	}

	return &Client{
		apiURL:       strings.TrimRight(apiURL, "/"),
		proxyURL:     proxyURL,
		siteURL:      defaultSiteURL,
		maxTimeoutMS: int(timeout.Milliseconds()),
		httpClient:   &http.Client{Timeout: timeout + 15*time.Second},
	}, nil
}

func (c *Client) Search(ctx context.Context, imageURL string) ([]Result, error) {
	image, err := url.Parse(strings.TrimSpace(imageURL))
	if err != nil || image.Host == "" || (image.Scheme != "http" && image.Scheme != "https") {
		return nil, fmt.Errorf("invalid image URL")
	}

	session := fmt.Sprintf("macarthurgo-%d-%d", time.Now().UnixNano(), c.sessionID.Add(1))
	create := flareRequest{Command: "sessions.create", Session: session}
	if c.proxyURL != "" {
		create.Proxy = &flareProxy{URL: c.proxyURL}
	}
	if _, err = c.call(ctx, create); err != nil {
		return nil, fmt.Errorf("create FlareSolverr session: %w", err)
	}
	defer c.destroySession(session)

	// ascii2d requires the search mode on URL searches. Without it the
	// browser stays on /search/url/... instead of redirecting to the color
	// result page, even though FlareSolverr itself reports a successful 200.
	searchURL := c.siteURL + "/search/url/" + url.QueryEscape(image.String()) + "?type=color"
	// ascii2d redirects to the result page asynchronously. FlareSolverr can
	// otherwise return the initial /search/url/... page before that navigation.
	color, err := c.get(ctx, session, searchURL, colorSearchWaitSeconds)
	if err != nil {
		return nil, fmt.Errorf("open ascii2d color search: %w", err)
	}

	// FlareSolverr snapshots driver.current_url before waitInSeconds, but takes
	// page_source after that wait. The URL can therefore still point at
	// /search/url/... while Response already contains the redirected result
	// page. Recover the actual result URLs from the post-wait HTML instead of
	// treating that stale URL snapshot as a failed navigation.
	colorURL := findResultPageURL(color, "color", c.siteURL)
	bovwURL := findResultPageURL(color, "bovw", c.siteURL)
	if colorURL == "" && bovwURL != "" {
		colorURL = switchResultMode(bovwURL, "bovw", "color")
	}
	if bovwURL == "" && colorURL != "" {
		bovwURL = switchResultMode(colorURL, "color", "bovw")
	}

	results := make([]Result, 0, 2)
	colorResultURL := colorURL
	if colorResultURL == "" {
		colorResultURL = color.URL
	}
	colorResult, colorErr := parseResult(color.Response, "color", colorResultURL, c.siteURL)
	if colorErr == nil {
		colorResult.cookies = color.Cookies
		colorResult.userAgent = color.UserAgent
		results = append(results, colorResult)
	}

	var (
		bovw    flareSolution
		bovwErr error
	)
	if bovwURL == "" {
		bovwErr = errors.New("could not recover the ascii2d result URL from the post-wait HTML")
	} else {
		bovw, bovwErr = c.get(ctx, session, bovwURL, 0)
	}
	if bovwErr == nil {
		bovwResult, parseErr := parseResult(bovw.Response, "bovw", bovw.URL, c.siteURL)
		if parseErr == nil {
			bovwResult.cookies = bovw.Cookies
			bovwResult.userAgent = bovw.UserAgent
			results = append(results, bovwResult)
		} else {
			bovwErr = parseErr
		}
	}

	if len(results) == 0 {
		return nil, errors.Join(colorErr, bovwErr)
	}
	return results, errors.Join(colorErr, bovwErr)
}

func findResultPageURL(solution flareSolution, mode string, siteURL string) string {
	candidates := []string{solution.URL}
	doc, err := htmlquery.Parse(strings.NewReader(solution.Response))
	if err == nil {
		for _, node := range htmlquery.Find(doc, `//*[@href]`) {
			candidates = append(candidates, htmlquery.SelectAttr(node, "href"))
		}
		for _, node := range htmlquery.Find(doc, `//*[@action]`) {
			candidates = append(candidates, htmlquery.SelectAttr(node, "action"))
		}
		for _, node := range htmlquery.Find(doc, `//meta[@content]`) {
			candidates = append(candidates, htmlquery.SelectAttr(node, "content"))
		}
	}

	for _, candidate := range candidates {
		if resultURL, ok := normalizeResultPageURL(candidate, mode, siteURL); ok {
			return resultURL
		}
	}
	return ""
}

func normalizeResultPageURL(candidate string, mode string, siteURL string) (string, bool) {
	resultURL := absoluteURL(siteURL, candidate)
	parsed, err := url.Parse(resultURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", false
	}
	site, err := url.Parse(siteURL)
	if err != nil || !strings.EqualFold(parsed.Host, site.Host) {
		return "", false
	}

	prefix := "/search/" + mode + "/"
	if !strings.HasPrefix(parsed.Path, prefix) {
		return "", false
	}
	identifier := strings.Trim(strings.TrimPrefix(parsed.Path, prefix), "/")
	if identifier == "" || strings.Contains(identifier, "/") {
		return "", false
	}
	parsed.Fragment = ""
	return parsed.String(), true
}

func switchResultMode(resultURL string, from string, to string) string {
	return strings.Replace(resultURL, "/search/"+from+"/", "/search/"+to+"/", 1)
}

func (c *Client) DownloadThumbnail(ctx context.Context, result Result) ([]byte, error) {
	if result.Thumbnail == "" {
		return nil, errors.New("thumbnail URL is empty")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, result.Thumbnail, nil)
	if err != nil {
		return nil, err
	}
	if result.userAgent != "" {
		req.Header.Set("User-Agent", result.userAgent)
	}
	req.Header.Set("Referer", c.siteURL+"/")
	for _, cookie := range result.cookies {
		req.AddCookie(&http.Cookie{Name: cookie.Name, Value: cookie.Value, Path: cookie.Path, Domain: cookie.Domain, Secure: cookie.Secure})
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("thumbnail request returned %s", resp.Status)
	}
	if contentType := resp.Header.Get("Content-Type"); contentType != "" && !strings.HasPrefix(contentType, "image/") && contentType != "application/octet-stream" {
		return nil, fmt.Errorf("thumbnail returned unexpected content type %q", contentType)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20+1))
	if err != nil {
		return nil, err
	}
	if len(data) > 10<<20 {
		return nil, errors.New("thumbnail exceeds 10 MiB")
	}
	return data, nil
}

func (c *Client) get(ctx context.Context, session string, targetURL string, waitInSeconds int) (flareSolution, error) {
	res, err := c.call(ctx, flareRequest{
		Command:       "request.get",
		URL:           targetURL,
		Session:       session,
		MaxTimeout:    c.maxTimeoutMS,
		WaitInSeconds: waitInSeconds,
	})
	if err != nil {
		return flareSolution{}, err
	}
	if res.Solution.Status < http.StatusOK || res.Solution.Status >= http.StatusMultipleChoices {
		return flareSolution{}, fmt.Errorf("browser request returned HTTP %d", res.Solution.Status)
	}
	if strings.TrimSpace(res.Solution.Response) == "" {
		return flareSolution{}, errors.New("browser request returned an empty page")
	}
	if code := chromiumNetworkErrorCode(res.Solution.Response); code != "" {
		return flareSolution{}, fmt.Errorf("FlareSolverr browser navigation failed with %s; check network or proxy connectivity from the FlareSolverr host", code)
	}
	return res.Solution, nil
}

func chromiumNetworkErrorCode(body string) string {
	if !strings.Contains(body, "window.loadTimeDataRaw") {
		return ""
	}
	match := chromiumErrorCodePattern.FindStringSubmatch(body)
	if len(match) != 2 {
		return ""
	}
	return match[1]
}

func (c *Client) call(ctx context.Context, payload flareRequest) (flareResponse, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return flareResponse{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.apiURL, bytes.NewReader(body))
	if err != nil {
		return flareResponse{}, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return flareResponse{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4<<10))
		return flareResponse{}, fmt.Errorf("FlareSolverr returned %s", resp.Status)
	}

	var result flareResponse
	decoder := json.NewDecoder(io.LimitReader(resp.Body, maxAPIResponse))
	if err = decoder.Decode(&result); err != nil {
		return flareResponse{}, fmt.Errorf("decode FlareSolverr response: %w", err)
	}
	if result.Status != "ok" {
		message := strings.TrimSpace(result.Message)
		if message == "" {
			message = "unknown error"
		}
		return flareResponse{}, errors.New(message)
	}
	return result, nil
}

func (c *Client) destroySession(session string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, _ = c.call(ctx, flareRequest{Command: "sessions.destroy", Session: session})
}

func parseResult(body string, mode string, resultURL string, siteURL string) (Result, error) {
	doc, err := htmlquery.Parse(strings.NewReader(body))
	if err != nil {
		return Result{}, fmt.Errorf("parse ascii2d HTML: %w", err)
	}

	boxes := htmlquery.Find(doc, `//div[contains(concat(' ', normalize-space(@class), ' '), ' item-box ')]`)
	for _, box := range boxes {
		detail := htmlquery.FindOne(box, `.//*[contains(concat(' ', normalize-space(@class), ' '), ' detail-box ')]`)
		if detail == nil {
			continue
		}
		links := htmlquery.Find(detail, `.//a[@href]`)
		if len(links) == 0 {
			continue
		}

		image := htmlquery.FindOne(box, `.//*[contains(concat(' ', normalize-space(@class), ' '), ' image-box ')]//img[@src]`)
		result := Result{
			Mode:      mode,
			Title:     cleanText(htmlquery.InnerText(links[0])),
			URL:       strings.TrimSpace(htmlquery.SelectAttr(links[0], "href")),
			ResultURL: resultURL,
		}
		if len(links) > 1 {
			result.Author = cleanText(htmlquery.InnerText(links[1]))
			result.AuthorURL = strings.TrimSpace(htmlquery.SelectAttr(links[1], "href"))
		}
		if image != nil {
			result.Thumbnail = absoluteURL(siteURL, htmlquery.SelectAttr(image, "src"))
		}
		if info := htmlquery.FindOne(detail, `.//small[1]`); info != nil {
			result.Info = cleanText(htmlquery.InnerText(info))
		}
		if source := htmlquery.FindOne(detail, `.//h6/small`); source != nil {
			result.SourceType = cleanText(htmlquery.InnerText(source))
		}
		if result.Title != "" && result.URL != "" {
			return result, nil
		}
	}
	return Result{}, fmt.Errorf("no usable ascii2d %s result found", mode)
}

func absoluteURL(baseURL string, value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return ""
	}
	if parsed.IsAbs() {
		return parsed.String()
	}
	base, err := url.Parse(baseURL)
	if err != nil {
		return ""
	}
	return base.ResolveReference(parsed).String()
}

func cleanText(value string) string {
	return strings.Join(strings.Fields(value), " ")
}
