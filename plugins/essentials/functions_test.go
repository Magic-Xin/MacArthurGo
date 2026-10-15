package essentials

import "testing"

func TestRemoveMarkdownOrder(t *testing.T) {
	input := "## **标题**\n![图片](https://example.com/a.jpg)\n[链接](https://example.com)"
	want := "标题\n图片\n链接"
	for i := 0; i < 100; i++ {
		if got := RemoveMarkdown(input); got != want {
			t.Fatalf("RemoveMarkdown() = %q, want %q", got, want)
		}
	}
}
