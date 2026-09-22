package chatai

import "testing"

func TestGeminiToolsForTextModels(t *testing.T) {
	for _, model := range []string{"gemini-3.8-flash", "gemini-3.1-pro-preview"} {
		tools := geminiTools(model)
		if len(tools) != 3 || tools[0].GoogleSearch == nil || tools[1].GoogleMaps == nil || tools[2].URLContext == nil {
			t.Errorf("%s tools = %#v", model, tools)
		}
	}
	if tools := geminiTools("gemini-3-pro-image-preview"); len(tools) != 0 {
		t.Errorf("image model tools = %#v", tools)
	}
}
