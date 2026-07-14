package plugins

type wordTokenizer interface {
	CutForSearch(string, bool) []string
}
