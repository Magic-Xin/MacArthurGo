package soutubot

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	DefaultSimilarityThreshold = 45.0

	mainPageURL       = "https://soutubot.moe"
	searchAPIURL      = mainPageURL + "/api/search"
	defaultTimeout    = 90 * time.Second
	maxImageSize      = 20 << 20
	maxResponseSize   = 16 << 20
	webkitBoundary    = "----WebKitFormBoundary"
	boundarySuffixLen = 16
)

var globalMPattern = regexp.MustCompile(`\bm\s*:\s*([0-9]+)\s*,`)

type Config struct {
	CloudflareBypassURL string
	ProxyURL            string
	Timeout             time.Duration
}

type Client struct {
	bypassURL string
	proxyURL  string
	timeout   time.Duration
	http      *http.Client

	mu        sync.Mutex
	userAgent string
	globalM   int64
}

type Response struct {
	Data          []Item  `json:"data"`
	ID            string  `json:"id"`
	Factor        float64 `json:"factor"`
	ImageURL      string  `json:"imageUrl"`
	SearchOption  string  `json:"searchOption"`
	ExecutionTime float64 `json:"executionTime"`
}

type Item struct {
	Source          Source   `json:"source"`
	Page            int      `json:"page"`
	Title           string   `json:"title"`
	Language        Language `json:"language"`
	PagePath        string   `json:"pagePath"`
	SubjectPath     string   `json:"subjectPath"`
	PreviewImageURL string   `json:"previewImageUrl"`
	Similarity      float64  `json:"similarity"`
}

type Source string

var sourceBaseURLs = map[Source]string{
	"nhentai": "https://nhentai.net",
	"ehentai": "https://e-hentai.org",
	"panda":   "https://panda.chaika.moe",
}

type Language string

const (
	LanguageChinese  Language = "cn"
	LanguageJapanese Language = "jp"
	LanguageEnglish  Language = "gb"
)

func (l Language) Emoji() string {
	switch l {
	case LanguageChinese:
		return "🇨🇳"
	case LanguageJapanese:
		return "🇯🇵"
	case LanguageEnglish:
		return "🇬🇧"
	default:
		return string(l)
	}
}

func (i Item) SourceURL() string {
	baseURL, knownSource := sourceBaseURLs[i.Source]
	if !knownSource {
		parsed, err := url.Parse(strings.TrimSpace(string(i.Source)))
		if err == nil && parsed.Host != "" && (parsed.Scheme == "http" || parsed.Scheme == "https") {
			return parsed.String()
		}
		return ""
	}

	path := strings.TrimSpace(i.SubjectPath)
	if path == "" {
		path = strings.TrimSpace(i.PagePath)
	}
	if path == "" {
		return baseURL
	}
	base, _ := url.Parse(baseURL)
	reference, err := url.Parse(path)
	if err != nil {
		return baseURL
	}
	// API paths are expected to be relative to the known source. Clearing an
	// unexpected host keeps malformed response data on that trusted host.
	reference.Scheme = ""
	reference.Host = ""
	return base.ResolveReference(reference).String()
}

// SelectBestMatches applies SoutuBot's overall confidence threshold and then
// returns the highest-similarity Japanese, Chinese, and English items, in that order.
func SelectBestMatches(items []Item, threshold float64) ([]Item, float64) {
	maxSimilarity := 0.0
	for _, item := range items {
		if item.Similarity > maxSimilarity {
			maxSimilarity = item.Similarity
		}
	}
	if len(items) == 0 || maxSimilarity < threshold {
		return nil, maxSimilarity
	}

	matches := make([]Item, 0, 3)
	for _, language := range []Language{LanguageJapanese, LanguageChinese, LanguageEnglish} {
		var (
			best  Item
			found bool
		)
		for _, item := range items {
			if item.Language == language && (!found || item.Similarity > best.Similarity) {
				best = item
				found = true
			}
		}
		if found {
			matches = append(matches, best)
		}
	}
	return matches, maxSimilarity
}

func FormatMatches(matches []Item) string {
	lines := make([]string, 0, 3)
	for _, language := range []Language{LanguageJapanese, LanguageChinese, LanguageEnglish} {
		var (
			match Item
			found bool
		)
		for _, candidate := range matches {
			if candidate.Language == language && (!found || candidate.Similarity > match.Similarity) {
				match = candidate
				found = true
			}
		}
		if !found {
			switch language {
			case LanguageJapanese:
				lines = append(lines, "未找到日文结果")
			case LanguageChinese:
				lines = append(lines, "未找到中文结果")
			case LanguageEnglish:
				lines = append(lines, "未找到英文结果")
			}
			continue
		}

		title := strings.Join(strings.Fields(match.Title), " ")
		if title == "" {
			title = "-"
		}
		source := match.SourceURL()
		if source == "" {
			source = "-"
		}
		lines = append(lines, fmt.Sprintf(
			"标题: %s | 相似度: %.2f%% | 语言: %s | 来源: %s",
			title,
			match.Similarity,
			match.Language.Emoji(),
			source,
		))
	}
	return "SoutuBot\n" + strings.Join(lines, "\n")
}

func IsFormattedMatches(value string) bool {
	lines := strings.Split(value, "\n")
	if len(lines) != 4 || lines[0] != "SoutuBot" {
		return false
	}
	return isFormattedMatchOrMissing(lines[1], "未找到日文结果") &&
		isFormattedMatchOrMissing(lines[2], "未找到中文结果") &&
		isFormattedMatchOrMissing(lines[3], "未找到英文结果")
}

func isFormattedMatchOrMissing(value string, missingMessage string) bool {
	return value == missingMessage ||
		(strings.HasPrefix(value, "标题: ") &&
			strings.Contains(value, " | 相似度: ") &&
			strings.Contains(value, " | 语言: ") &&
			strings.Contains(value, " | 来源: "))
}

func NewClient(cfg Config) (*Client, error) {
	bypassURL := strings.TrimRight(strings.TrimSpace(cfg.CloudflareBypassURL), "/")
	parsed, err := url.Parse(bypassURL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, fmt.Errorf("invalid CloudflareBypassForScraping URL %q", bypassURL)
	}

	proxyURL := strings.TrimSpace(cfg.ProxyURL)
	if proxyURL != "" {
		proxy, parseErr := url.Parse(proxyURL)
		if parseErr != nil || proxy.Host == "" || (proxy.Scheme != "http" && proxy.Scheme != "https" && proxy.Scheme != "socks4" && proxy.Scheme != "socks5") {
			return nil, fmt.Errorf("invalid SoutuBot proxy URL %q", proxyURL)
		}
	}

	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	return &Client{
		bypassURL: bypassURL,
		proxyURL:  proxyURL,
		timeout:   timeout,
		http: &http.Client{
			Timeout: timeout,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}, nil
}

func (c *Client) Search(ctx context.Context, imageData []byte) (*Response, error) {
	if len(imageData) == 0 {
		return nil, errors.New("SoutuBot image data is empty")
	}
	if len(imageData) > maxImageSize {
		return nil, fmt.Errorf("SoutuBot image exceeds %d MiB", maxImageSize>>20)
	}

	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	// CloudflareBypassForScraping caches clearance state by host. Keep the
	// homepage-derived m value and user agent paired with the upload request.
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.userAgent == "" || c.globalM <= 0 {
		if err := c.refreshCredentials(ctx); err != nil {
			return nil, err
		}
	}

	result, err := c.search(ctx, imageData, false)
	var statusErr *httpStatusError
	if err == nil || !errors.As(err, &statusErr) || (statusErr.StatusCode != http.StatusUnauthorized && statusErr.StatusCode != http.StatusForbidden) {
		return result, err
	}

	if refreshErr := c.refreshCredentials(ctx); refreshErr != nil {
		return nil, errors.Join(err, refreshErr)
	}
	return c.search(ctx, imageData, true)
}

func (c *Client) refreshCredentials(ctx context.Context) error {
	endpoint, err := url.Parse(c.bypassURL + "/html")
	if err != nil {
		return err
	}
	query := endpoint.Query()
	query.Set("url", mainPageURL)
	if c.proxyURL != "" {
		query.Set("proxy", c.proxyURL)
	}
	endpoint.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("CloudflareBypassForScraping SoutuBot homepage request: %w", unwrapURLError(err))
	}
	defer resp.Body.Close()

	body, err := readLimited(resp.Body, maxResponseSize)
	if err != nil {
		return fmt.Errorf("read CloudflareBypassForScraping SoutuBot homepage: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return &httpStatusError{StatusCode: resp.StatusCode, Operation: "SoutuBot homepage"}
	}

	userAgent := strings.TrimSpace(resp.Header.Get("x-cf-bypasser-user-agent"))
	if userAgent == "" {
		return errors.New("CloudflareBypassForScraping SoutuBot homepage did not return a user agent")
	}
	globalM, err := parseGlobalM(body)
	if err != nil {
		return err
	}
	c.userAgent = userAgent
	c.globalM = globalM
	return nil
}

func (c *Client) search(ctx context.Context, imageData []byte, forceBypass bool) (*Response, error) {
	body, contentType, err := buildMultipartBody(imageData)
	if err != nil {
		return nil, fmt.Errorf("build SoutuBot request: %w", err)
	}

	target, _ := url.Parse(searchAPIURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.bypassURL+target.EscapedPath(), body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Origin", mainPageURL)
	req.Header.Set("Referer", mainPageURL+"/")
	req.Header.Set("DNT", "1")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("X-Api-Key", calculateAPIKey(len(c.userAgent), c.globalM, time.Now().Unix()))
	req.Header.Set("x-hostname", target.Host)
	if c.proxyURL != "" {
		req.Header.Set("x-proxy", c.proxyURL)
	}
	if forceBypass {
		req.Header.Set("x-bypass-cache", "true")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("CloudflareBypassForScraping SoutuBot search request: %w", unwrapURLError(err))
	}
	defer resp.Body.Close()

	responseBody, err := readLimited(resp.Body, maxResponseSize)
	if err != nil {
		return nil, fmt.Errorf("read SoutuBot response: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, &httpStatusError{StatusCode: resp.StatusCode, Operation: "SoutuBot search"}
	}

	var result Response
	if err = json.Unmarshal(responseBody, &result); err != nil {
		return nil, fmt.Errorf("decode SoutuBot response: %w", err)
	}
	return &result, nil
}

func parseGlobalM(body []byte) (int64, error) {
	match := globalMPattern.FindSubmatch(body)
	if len(match) != 2 {
		return 0, errors.New("could not find SoutuBot global m value")
	}
	value, err := strconv.ParseInt(string(match[1]), 10, 64)
	if err != nil || value <= 0 {
		return 0, errors.New("invalid SoutuBot global m value")
	}
	return value, nil
}

func buildMultipartBody(imageData []byte) (*bytes.Buffer, string, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	suffix := make([]byte, 12)
	if _, err := io.ReadFull(rand.Reader, suffix); err != nil {
		return nil, "", err
	}
	boundary := webkitBoundary + base64.RawURLEncoding.EncodeToString(suffix)[:boundarySuffixLen]
	if err := writer.SetBoundary(boundary); err != nil {
		return nil, "", err
	}

	header := textproto.MIMEHeader{}
	header.Set("Content-Disposition", `form-data; name="file"; filename="image"`)
	header.Set("Content-Type", "application/octet-stream")
	part, err := writer.CreatePart(header)
	if err != nil {
		return nil, "", err
	}
	if _, err = part.Write(imageData); err != nil {
		return nil, "", err
	}
	if err = writer.WriteField("factor", "1.2"); err != nil {
		return nil, "", err
	}
	if err = writer.Close(); err != nil {
		return nil, "", err
	}
	return body, writer.FormDataContentType(), nil
}

func calculateAPIKey(userAgentLength int, globalM int64, timestamp int64) string {
	timeValue := float64(timestamp)
	value := math.Pow(timeValue, 2) + math.Pow(float64(userAgentLength), 2) + float64(globalM)
	encoded := []byte(base64.StdEncoding.EncodeToString([]byte(strconv.FormatFloat(value, 'g', -1, 64))))
	for left, right := 0, len(encoded)-1; left < right; left, right = left+1, right-1 {
		encoded[left], encoded[right] = encoded[right], encoded[left]
	}
	return strings.ReplaceAll(string(encoded), "=", "")
}

func readLimited(reader io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("response exceeds %d MiB", limit>>20)
	}
	return data, nil
}

func unwrapURLError(err error) error {
	var urlError *url.Error
	if errors.As(err, &urlError) {
		return urlError.Err
	}
	return err
}

type httpStatusError struct {
	StatusCode int
	Operation  string
}

func (e *httpStatusError) Error() string {
	return fmt.Sprintf("CloudflareBypassForScraping %s returned HTTP %d", e.Operation, e.StatusCode)
}
