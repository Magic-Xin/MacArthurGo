package ascii2d

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/antchfx/htmlquery"
)

var chromiumErrorCodePattern = regexp.MustCompile(`["']errorCode["']\s*:\s*["'](ERR_[A-Z0-9_]+)["']`)

const (
	defaultSiteURL  = "https://ascii2d.net"
	maxHTMLResponse = 16 << 20
)

type Config struct {
	CloudflareBypassURL string
	ProxyURL            string
	Timeout             time.Duration
}

type Client struct {
	siteURL string
	bypass  *bypassClient
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
}

type pageResponse struct {
	URL      string
	Status   int
	Response string
}

func NewClient(cfg Config) (*Client, error) {
	proxyURL := strings.TrimSpace(cfg.ProxyURL)
	if proxyURL != "" {
		parsedProxy, parseErr := url.Parse(proxyURL)
		if parseErr != nil || parsedProxy.Host == "" || (parsedProxy.Scheme != "http" && parsedProxy.Scheme != "https" && parsedProxy.Scheme != "socks4" && parsedProxy.Scheme != "socks5") {
			return nil, fmt.Errorf("invalid ascii2d proxy URL %q", proxyURL)
		}
	}

	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 90 * time.Second
	}
	httpClient := &http.Client{Timeout: timeout + 15*time.Second}
	bypass, err := newBypassClient(cfg.CloudflareBypassURL, proxyURL, httpClient)
	if err != nil {
		return nil, err
	}
	return &Client{siteURL: defaultSiteURL, bypass: bypass}, nil
}

func (c *Client) Search(ctx context.Context, imageURL string) ([]Result, error) {
	image, err := url.Parse(strings.TrimSpace(imageURL))
	if err != nil || image.Host == "" || (image.Scheme != "http" && image.Scheme != "https") {
		return nil, fmt.Errorf("invalid image URL")
	}

	// ascii2d requires the search mode on URL searches. Without it the
	// browser stays on /search/url/... instead of redirecting to the color result page.
	searchURL := c.siteURL + "/search/url/" + url.QueryEscape(image.String()) + "?type=color"
	color, err := c.getWithRetry(ctx, searchURL, false)
	if err != nil {
		return nil, fmt.Errorf("open ascii2d color search: %w", err)
	}

	// Recover result URLs from the rendered HTML when the bypass service's final
	// URL still points at the initial /search/url/... page.
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
		results = append(results, colorResult)
	}

	var (
		bovw    pageResponse
		bovwErr error
	)
	if bovwURL == "" {
		bovwErr = errors.New("could not recover the ascii2d result URL from the post-wait HTML")
	} else {
		bovw, bovwErr = c.getWithRetry(ctx, bovwURL, true)
	}
	if bovwErr == nil {
		bovwResult, parseErr := parseResult(bovw.Response, "bovw", bovw.URL, c.siteURL)
		if parseErr == nil {
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

func findResultPageURL(solution pageResponse, mode string, siteURL string) string {
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
	return c.bypass.getImage(ctx, result.Thumbnail)
}

func (c *Client) getWithRetry(ctx context.Context, targetURL string, retryAll bool) (pageResponse, error) {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		response, err := c.bypass.getHTML(ctx, targetURL)
		if err == nil {
			response, err = validatePageResponse(response)
			if err == nil {
				return response, nil
			}
		}
		lastErr = err
		if ctx.Err() != nil {
			return pageResponse{}, ctx.Err()
		}
		if !retryAll && !errors.Is(err, errFirstByteTimeout) && !strings.Contains(strings.ToLower(err.Error()), errFirstByteTimeout.Error()) {
			break
		}
	}
	return pageResponse{}, lastErr
}

func validatePageResponse(response pageResponse) (pageResponse, error) {
	if response.Status < http.StatusOK || response.Status >= http.StatusMultipleChoices {
		return pageResponse{}, fmt.Errorf("browser request returned HTTP %d", response.Status)
	}
	if strings.TrimSpace(response.Response) == "" {
		return pageResponse{}, errors.New("browser request returned an empty page")
	}
	if code := chromiumNetworkErrorCode(response.Response); code != "" {
		return pageResponse{}, fmt.Errorf("CloudflareBypassForScraping browser navigation failed with %s; check network or proxy connectivity from the bypass service host", code)
	}
	return response, nil
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
