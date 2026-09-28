package source

import (
	"bytes"
	"compress/zlib"
	"io"
	"regexp"
	"strconv"
)

var (
	pagesNode  = regexp.MustCompile(`/Type\s*/Pages\b`)
	pageLeaf   = regexp.MustCompile(`/Type\s*/Page(?:[^s]|$)`)
	countEntry = regexp.MustCompile(`/Count\s+(\d+)`)
	streamBody = regexp.MustCompile(`(?s)stream\r?\n(.*?)\r?\nendstream`)
)

// PDFPages counts a PDF's pages from its page tree: the largest /Count of a /Pages node,
// or, when no node says, the number of page objects. Page objects inside compressed
// object streams are read by inflating each stream. It returns 0 when neither is found.
func PDFPages(data []byte) int {
	if !bytes.HasPrefix(data, []byte("%PDF")) {
		return 0
	}
	parts := [][]byte{data}
	for _, m := range streamBody.FindAllSubmatch(data, -1) {
		r, err := zlib.NewReader(bytes.NewReader(m[1]))
		if err != nil {
			continue
		}
		inflated, err := io.ReadAll(io.LimitReader(r, 64<<20))
		r.Close()
		if err == nil && (pagesNode.Match(inflated) || pageLeaf.Match(inflated)) {
			parts = append(parts, inflated)
		}
	}
	best, leaves := 0, 0
	for _, part := range parts {
		for _, loc := range pagesNode.FindAllIndex(part, -1) {
			start := bytes.LastIndex(part[:loc[0]], []byte("<<"))
			if start < 0 {
				start = loc[0]
			}
			end := min(len(part), loc[1]+4096)
			window := part[start:end]
			if close := dictEnd(window); close > 0 {
				window = window[:close]
			}
			if m := countEntry.FindSubmatch(window); m != nil {
				if n, err := strconv.Atoi(string(m[1])); err == nil && n > best {
					best = n
				}
			}
		}
		leaves += len(pageLeaf.FindAllIndex(part, -1))
	}
	if best > 0 {
		return best
	}
	return leaves
}

// dictEnd is the offset just past the >> that closes the dictionary window opens with,
// or 0 when the window does not hold it.
func dictEnd(window []byte) int {
	depth := 0
	for i := 0; i+1 < len(window); i++ {
		switch {
		case window[i] == '<' && window[i+1] == '<':
			depth++
			i++
		case window[i] == '>' && window[i+1] == '>':
			depth--
			i++
			if depth == 0 {
				return i + 1
			}
		}
	}
	return 0
}
