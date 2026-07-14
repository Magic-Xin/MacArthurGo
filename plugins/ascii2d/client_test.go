package ascii2d

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestClientSearch(t *testing.T) {
	t.Parallel()

	var (
		commands          []string
		searchURL         string
		searchWaitSeconds int
		mediaDisabled     []bool
		mu                sync.Mutex
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request flareRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decode request: %v", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		mu.Lock()
		commands = append(commands, request.Command)
		mu.Unlock()

		response := flareResponse{Status: "ok"}
		switch {
		case request.Command == "request.get" && strings.Contains(request.URL, "/search/url/"):
			mu.Lock()
			searchURL = request.URL
			searchWaitSeconds = request.WaitInSeconds
			mediaDisabled = append(mediaDisabled, request.DisableMedia)
			mu.Unlock()
			response.Solution = flareSolution{
				URL:       "https://ascii2d.net/search/color/token",
				Status:    http.StatusOK,
				Response:  resultHTML("Color title", "/color.jpg"),
				UserAgent: "test-agent",
				Cookies:   []flareCookie{{Name: "cf_clearance", Value: "token"}},
			}
		case request.Command == "request.get" && strings.Contains(request.URL, "/bovw/"):
			mu.Lock()
			mediaDisabled = append(mediaDisabled, request.DisableMedia)
			mu.Unlock()
			response.Solution = flareSolution{
				URL:       "https://ascii2d.net/search/bovw/token",
				Status:    http.StatusOK,
				Response:  resultHTML("Bovw title", "/bovw.jpg"),
				UserAgent: "test-agent",
				Cookies:   []flareCookie{{Name: "cf_clearance", Value: "token"}},
			}
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Errorf("encode response: %v", err)
		}
	}))
	defer server.Close()

	client, err := NewClient(Config{APIURL: server.URL, Timeout: time.Second})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	results, err := client.Search(context.Background(), "https://example.com/image.png?size=large")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}
	if results[0].Mode != "color" || results[0].Title != "Color title" {
		t.Fatalf("unexpected color result: %#v", results[0])
	}
	if results[1].Mode != "bovw" || results[1].Title != "Bovw title" {
		t.Fatalf("unexpected bovw result: %#v", results[1])
	}
	if results[0].Thumbnail != "https://ascii2d.net/color.jpg" {
		t.Fatalf("unexpected thumbnail URL: %s", results[0].Thumbnail)
	}
	wantCommands := []string{"sessions.create", "request.get", "request.get", "sessions.destroy"}
	mu.Lock()
	defer mu.Unlock()
	if !strings.Contains(searchURL, "https%3A%2F%2Fexample.com%2Fimage.png%3Fsize%3Dlarge") {
		t.Fatalf("image URL was not safely encoded: %s", searchURL)
	}
	if !strings.HasSuffix(searchURL, "?type=color") {
		t.Fatalf("color search type is missing: %s", searchURL)
	}
	if searchWaitSeconds != colorSearchWaitSeconds {
		t.Fatalf("color search wait = %d, want %d", searchWaitSeconds, colorSearchWaitSeconds)
	}
	if !reflect.DeepEqual(mediaDisabled, []bool{true, true}) {
		t.Fatalf("request.get disableMedia values = %v, want [true true]", mediaDisabled)
	}
	if !reflect.DeepEqual(commands, wantCommands) {
		t.Fatalf("commands = %v, want %v", commands, wantCommands)
	}
}

func TestClientSearch_ReportsChromiumNetworkErrorWithoutLeakingImageURL(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request flareRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decode request: %v", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		response := flareResponse{Status: "ok"}
		if request.Command == "request.get" {
			response.Solution = flareSolution{
				URL:    request.URL,
				Status: http.StatusOK,
				Response: `<html><script>window.loadTimeDataRaw = {` +
					`"errorCode":"ERR_TIMED_OUT"};</script></html>`,
			}
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Errorf("encode response: %v", err)
		}
	}))
	defer server.Close()

	client, err := NewClient(Config{APIURL: server.URL, Timeout: time.Second})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	_, err = client.Search(context.Background(), "https://example.com/image.png?rkey=secret")
	if err == nil || !strings.Contains(err.Error(), "ERR_TIMED_OUT") {
		t.Fatalf("Search error = %v, want ERR_TIMED_OUT", err)
	}
	if strings.Contains(err.Error(), "rkey") || strings.Contains(err.Error(), "secret") {
		t.Fatalf("Search error leaked the source image URL: %v", err)
	}
}

func TestClientSearch_UsesPostWaitHTMLWhenFlareURLIsStale(t *testing.T) {
	t.Parallel()

	const (
		staleURL = "https://ascii2d.net/search/url/encoded-image?type=color"
		colorURL = "https://ascii2d.net/search/color/result-token"
		bovwURL  = "https://ascii2d.net/search/bovw/result-token"
	)
	var requestedBovwURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request flareRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decode request: %v", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		response := flareResponse{Status: "ok"}
		switch {
		case request.Command == "request.get" && strings.Contains(request.URL, "/search/url/"):
			response.Solution = flareSolution{
				URL:      staleURL,
				Status:   http.StatusOK,
				Response: `<html><head><link rel="canonical" href="` + colorURL + `"></head><body>` + resultHTML("Color title", "/color.jpg") + `</body></html>`,
			}
		case request.Command == "request.get" && request.URL == bovwURL:
			requestedBovwURL = request.URL
			response.Solution = flareSolution{
				URL:      bovwURL,
				Status:   http.StatusOK,
				Response: resultHTML("Bovw title", "/bovw.jpg"),
			}
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Errorf("encode response: %v", err)
		}
	}))
	defer server.Close()

	client, err := NewClient(Config{APIURL: server.URL, Timeout: time.Second})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	results, err := client.Search(context.Background(), "https://example.com/image.png")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}
	if results[0].ResultURL != colorURL {
		t.Fatalf("color result URL = %q, want %q", results[0].ResultURL, colorURL)
	}
	if requestedBovwURL != bovwURL {
		t.Fatalf("requested bovw URL = %q, want %q", requestedBovwURL, bovwURL)
	}
}

func TestFindResultPageURL_RejectsAnotherHost(t *testing.T) {
	t.Parallel()

	solution := flareSolution{
		URL:      "https://ascii2d.net/search/url/image?type=color",
		Response: `<link rel="canonical" href="https://example.com/search/color/not-ascii2d">`,
	}
	if got := findResultPageURL(solution, "color", "https://ascii2d.net"); got != "" {
		t.Fatalf("findResultPageURL accepted another host: %q", got)
	}
}

func TestClientSearchRejectsInvalidURL(t *testing.T) {
	t.Parallel()

	client, err := NewClient(Config{APIURL: "http://127.0.0.1:8191/v1"})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if _, err = client.Search(context.Background(), "file:///tmp/image.png"); err == nil {
		t.Fatal("Search accepted a non-HTTP image URL")
	}
}

func TestParseResultIgnoresIncompleteBoxes(t *testing.T) {
	t.Parallel()

	body := `<div class="row item-box"><div class="detail-box">no links</div></div>` + resultHTML("Title", "//cdn.ascii2d.net/thumb.jpg")
	result, err := parseResult(body, "color", "https://ascii2d.net/search/color/id", "https://ascii2d.net")
	if err != nil {
		t.Fatalf("parseResult: %v", err)
	}
	if result.Title != "Title" || result.Author != "Author" || result.SourceType != "pixiv" {
		t.Fatalf("unexpected result: %#v", result)
	}
	if result.Thumbnail != "https://cdn.ascii2d.net/thumb.jpg" {
		t.Fatalf("unexpected thumbnail: %s", result.Thumbnail)
	}
}

func TestDownloadThumbnailUsesClearanceCredentials(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("cf_clearance")
		if err != nil || cookie.Value != "token" {
			http.Error(w, "missing cookie", http.StatusForbidden)
			return
		}
		if r.UserAgent() != "test-agent" {
			http.Error(w, "wrong user agent", http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write([]byte("image-data"))
	}))
	defer server.Close()

	client, err := NewClient(Config{APIURL: server.URL, Timeout: time.Second})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	data, err := client.DownloadThumbnail(context.Background(), Result{
		Thumbnail: server.URL + "/thumb.jpg",
		cookies:   []flareCookie{{Name: "cf_clearance", Value: "token"}},
		userAgent: "test-agent",
	})
	if err != nil {
		t.Fatalf("DownloadThumbnail: %v", err)
	}
	if string(data) != "image-data" {
		t.Fatalf("data = %q", data)
	}
}

func resultHTML(title string, thumbnail string) string {
	return fmt.Sprintf(`
<div class="row item-box">
  <div class="image-box"><img src="%s"></div>
  <div class="detail-box">
    <small>1200x800 JPEG</small>
    <h6><a href="https://example.com/work"> %s </a><a href="https://example.com/author">Author</a><small>pixiv</small></h6>
  </div>
</div>`, thumbnail, title)
}
