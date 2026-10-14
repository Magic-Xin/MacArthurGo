package chatai

import (
	"MacArthurGo/structs/cqcode"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"google.golang.org/genai"
)

func TestGeminiToolsForTextModels(t *testing.T) {
	for _, model := range []string{GeminiFlashModel, GeminiProModel} {
		tools := geminiTools(model)
		if len(tools) != 3 || tools[0].GoogleSearch == nil || tools[1].GoogleMaps == nil || tools[2].URLContext == nil {
			t.Errorf("%s tools = %#v", model, tools)
		}
	}
	if tools := geminiTools(GeminiImageModel); len(tools) != 0 {
		t.Errorf("image model tools = %#v", tools)
	}
}

func TestParseGeminiThinking(t *testing.T) {
	for _, test := range []struct {
		text, want string
		level      genai.ThinkingLevel
	}{
		{"low explain", "explain", genai.ThinkingLevelLow},
		{"high 为什么", "为什么", genai.ThinkingLevelHigh},
		{"high为什么", "high为什么", ""},
		{"mid explain", "explain", genai.ThinkingLevelMedium},
		{"medium explain", "explain", genai.ThinkingLevelMedium},
		{"HIGH\nexplain", "explain", genai.ThinkingLevelHigh},
		{"  high\texplain", "explain", genai.ThinkingLevelHigh},
		{"low", "", genai.ThinkingLevelLow},
		{"highway", "highway", ""},
		{"medium-sized", "medium-sized", ""},
		{"explain high", "explain high", ""},
	} {
		t.Run(test.text, func(t *testing.T) {
			message := []cqcode.ArrayMessage{*cqcode.Reply("42"), *cqcode.Text(test.text), *cqcode.Image("keep.png"), *cqcode.Text("high later")}
			got, level := parseGeminiThinking(message, GeminiProModel)
			if level != test.level || got[1].Data["text"] != test.want || got[0].Type != "reply" || got[2].Type != "image" || got[3].Data["text"] != "high later" {
				t.Fatalf("parsed = %#v, %q", got, level)
			}
			if message[1].Data["text"] != test.text {
				t.Fatal("parser mutated the cached original message")
			}
		})
	}
	message := []cqcode.ArrayMessage{*cqcode.Text("high draw")}
	if got, level := parseGeminiThinking(message, GeminiImageModel); level != "" || got[0].Data["text"] != "high draw" {
		t.Fatalf("image command consumed thinking argument: %#v, %q", got, level)
	}
}

func TestGeminiRequestsAndResponseBlocks(t *testing.T) {
	for _, test := range []struct {
		name, model, prompt, wantPrompt string
		level                           genai.ThinkingLevel
		quoted                          bool
	}{
		{"pro default", GeminiProModel, "explain", "explain", genai.ThinkingLevelLow, false},
		{"pro high", GeminiProModel, "high explain", "explain", genai.ThinkingLevelHigh, false},
		{"flash low", GeminiFlashModel, "low explain", "explain", genai.ThinkingLevelLow, false},
		{"quoted mid", GeminiProModel, "mid explain", "explain", genai.ThinkingLevelMedium, true},
		{"quoted default", GeminiFlashModel, "explain", "explain", genai.ThinkingLevelLow, true},
		{"flash high separate", GeminiFlashModel, "high 为什么", "为什么", genai.ThinkingLevelHigh, false},
		{"flash high joined", GeminiFlashModel, "high为什么", "high为什么", genai.ThinkingLevelLow, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if !strings.HasSuffix(r.URL.Path, "/models/"+test.model+":generateContent") {
					t.Errorf("request path = %q", r.URL.Path)
				}
				var request struct {
					Contents []*genai.Content `json:"contents"`
					Tools    []*genai.Tool    `json:"tools"`
					Config   struct {
						Thinking *genai.ThinkingConfig `json:"thinkingConfig"`
					} `json:"generationConfig"`
				}
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Errorf("decode request: %v", err)
					http.Error(w, "bad JSON", http.StatusBadRequest)
					return
				}
				thinking := request.Config.Thinking
				if thinking == nil || thinking.ThinkingLevel != test.level || !thinking.IncludeThoughts || thinking.ThinkingBudget != nil {
					t.Errorf("thinking config = %#v", thinking)
				}
				var texts []string
				for _, content := range request.Contents {
					for _, part := range content.Parts {
						texts = append(texts, part.Text)
					}
				}
				wantTexts := []string{test.wantPrompt}
				if test.quoted {
					wantTexts = append(wantTexts, "quoted source")
				}
				if !reflect.DeepEqual(texts, wantTexts) {
					t.Errorf("sent texts = %#v, want %#v", texts, wantTexts)
				}
				if len(request.Tools) != 3 || request.Tools[0].GoogleSearch == nil || request.Tools[1].GoogleMaps == nil || request.Tools[2].URLContext == nil {
					t.Errorf("tools = %#v", request.Tools)
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"candidates":[{"content":{"role":"model","parts":[{"text":"first ","thought":true},{"text":"thought","thought":true},{"text":"First "},{"text":"**answer**","thoughtSignature":"c2lnbmF0dXJl"}]},"groundingMetadata":{"groundingChunks":[{"maps":{"title":"Place","uri":"https://maps.google.com/example"}}]}}]}`))
			}))
			defer server.Close()
			gemini := &Gemini{ApiKey: "test-key", httpOptions: genai.HTTPOptions{BaseURL: server.URL}}
			message := []cqcode.ArrayMessage{*cqcode.Text(test.prompt)}
			var responses []string
			if test.quoted {
				message = append([]cqcode.ArrayMessage{*cqcode.Reply("42")}, message...)
				_, action := gemini.RequireAnswer(message, 123, test.model)
				var request struct {
					Action, Echo string
				}
				if err := json.Unmarshal(action, &request); err != nil || request.Action != "get_msg" || request.Echo != "gemini|123|"+test.model || calls.Load() != 0 {
					t.Fatalf("quote request = %#v, err = %v, API calls = %d", request, err, calls.Load())
				}
				responses = gemini.RequireEchoAnswer(message, []cqcode.ArrayMessage{*cqcode.Text("quoted source")}, test.model)
			} else {
				var action []byte
				responses, action = gemini.RequireAnswer(message, 123, test.model)
				if action != nil {
					t.Fatalf("unexpected quote request: %s", action)
				}
			}
			want := []string{
				"[" + test.model + "] thinking:\nfirst thought",
				"[" + test.model + "] answer:\nFirst answer\nGoogle Maps: Place https://maps.google.com/example",
			}
			if !reflect.DeepEqual(responses, want) || calls.Load() != 1 {
				t.Fatalf("responses = %#v, want %#v; API calls = %d", responses, want, calls.Load())
			}
		})
	}
}

func TestGeminiMissingThinkingSummary(t *testing.T) {
	resp := &genai.GenerateContentResponse{Candidates: []*genai.Candidate{nil, {Content: &genai.Content{Parts: []*genai.Part{nil, {Text: "answer"}}}}}}
	got := formatGeminiResponse(resp, GeminiProModel)
	want := []string{"[gemini-4-argon] thinking:\n模型未返回 thinking 摘要", "[gemini-4-argon] answer:\nanswer"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("response = %#v, want %#v", got, want)
	}
}

func TestGeminiImageResponsePreservesImages(t *testing.T) {
	resp := &genai.GenerateContentResponse{Candidates: []*genai.Candidate{{Content: &genai.Content{Parts: []*genai.Part{
		{Thought: true, Text: "hidden thought"},
		{Text: "generated image"},
		{InlineData: &genai.Blob{Data: []byte("image"), MIMEType: "image/png"}},
	}}}}}
	got := formatGeminiResponse(resp, GeminiImageModel)
	want := []string{GeminiImageModel + " response: ", "generated image", "base64://aW1hZ2U="}
	if !reflect.DeepEqual(got, want) || geminiThinkingConfig(GeminiImageModel, "") != nil {
		t.Fatalf("image response = %#v, want %#v", got, want)
	}
}
