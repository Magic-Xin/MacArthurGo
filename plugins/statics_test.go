package plugins

import "testing"

type fixedTokenizer struct{ words []string }

func (f fixedTokenizer) Cut(string, bool) []string { return f.words }
func (fixedTokenizer) Free()                       {}

func TestSegmentWordsFiltersShortWordsAndParticles(t *testing.T) {
	s := &Statics{
		tokenizer: fixedTokenizer{words: []string{"今天", "的", "天气", "真", "不错", "天气", "如果", "a", "Go"}},
		stopWords: buildStopWords([]string{"如果"}),
	}
	got := s.segmentWords("今天的天气真不错，如果用 Go")
	want := map[string]int{"今天": 1, "天气": 2, "不错": 1, "go": 1}
	if len(got) != len(want) {
		t.Fatalf("word frequencies = %#v, want %#v", got, want)
	}
	for word, count := range want {
		if got[word] != count {
			t.Errorf("frequency of %q = %d, want %d", word, got[word], count)
		}
	}
}

func TestStaticsStoreKeepsUpdatesAfterSnapshot(t *testing.T) {
	store, err := newStaticsStore(t.TempDir(), 2)
	if err != nil {
		t.Fatal(err)
	}
	const date = "2026-09-22"
	store.incrementMessage(date, 1, 12)
	versionAtSnapshot := store.dirtyDates[date]
	store.addWordCounts(date, 1, map[string]int{"测试": 1})
	store.clearDirtyDate(date, versionAtSnapshot)
	if _, ok := store.dirtyDates[date]; !ok {
		t.Fatal("new update was cleared by an older snapshot")
	}
	if err := store.flushDate(date); err != nil {
		t.Fatal(err)
	}
	if _, ok := store.dirtyDates[date]; ok {
		t.Fatal("completed flush left date dirty")
	}
}
