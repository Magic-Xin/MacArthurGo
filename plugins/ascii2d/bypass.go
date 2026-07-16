package ascii2d

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

var errFirstByteTimeout = errors.New("first byte timeout")

type bypassClient struct {
	baseURL    string
	proxyURL   string
	httpClient *http.Client
	mu         sync.Mutex
}

func newBypassClient(baseURL string, proxyURL string, httpClient *http.Client) (*bypassClient, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, fmt.Errorf("invalid CloudflareBypassForScraping URL %q", baseURL)
	}
	return &bypassClient{baseURL: baseURL, proxyURL: proxyURL, httpClient: httpClient}, nil
}

func (c *bypassClient) getHTML(ctx context.Context, targetURL string) (flareSolution, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	endpoint, err := url.Parse(c.baseURL + "/html")
	if err != nil {
		return flareSolution{}, err
	}
	query := endpoint.Query()
	query.Set("url", targetURL)
	if c.proxyURL != "" {
		query.Set("proxy", c.proxyURL)
	}
	endpoint.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return flareSolution{}, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return flareSolution{}, fmt.Errorf("CloudflareBypassForScraping HTML request: %w", unwrapURLError(err))
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxAPIResponse+1))
	if err != nil {
		return flareSolution{}, fmt.Errorf("read CloudflareBypassForScraping HTML: %w", err)
	}
	if len(body) > maxAPIResponse {
		return flareSolution{}, errors.New("CloudflareBypassForScraping HTML exceeds 16 MiB")
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		if strings.Contains(strings.ToLower(string(body)), errFirstByteTimeout.Error()) {
			return flareSolution{}, errFirstByteTimeout
		}
		return flareSolution{}, fmt.Errorf("CloudflareBypassForScraping returned %s", resp.Status)
	}

	finalURL := strings.TrimSpace(resp.Header.Get("x-cf-bypasser-final-url"))
	if finalURL == "" {
		finalURL = targetURL
	}
	return flareSolution{
		URL:       finalURL,
		Status:    http.StatusOK,
		Response:  string(body),
		UserAgent: strings.TrimSpace(resp.Header.Get("x-cf-bypasser-user-agent")),
	}, nil
}

func (c *bypassClient) getImage(ctx context.Context, targetURL string) ([]byte, error) {
	parsed, err := url.Parse(strings.TrimSpace(targetURL))
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, errors.New("invalid thumbnail URL")
	}

	path := parsed.EscapedPath()
	if path == "" {
		path = "/"
	}
	endpoint := c.baseURL + path
	if parsed.RawQuery != "" {
		endpoint += "?" + parsed.RawQuery
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-hostname", parsed.Host)
	if c.proxyURL != "" {
		req.Header.Set("x-proxy", c.proxyURL)
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("CloudflareBypassForScraping image request: %w", unwrapURLError(err))
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("CloudflareBypassForScraping image returned %s", resp.Status)
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

func unwrapURLError(err error) error {
	var urlError *url.Error
	if errors.As(err, &urlError) {
		return urlError.Err
	}
	return err
}
