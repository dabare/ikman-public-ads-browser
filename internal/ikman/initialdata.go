package ikman

import (
	"bytes"
	"errors"
)

var initialDataMarker = []byte("window.initialData =")

func ExtractInitialData(html []byte) ([]byte, error) {
	idx := bytes.Index(html, initialDataMarker)
	if idx < 0 {
		return nil, errors.New("window.initialData marker not found")
	}

	start := idx + len(initialDataMarker)
	for start < len(html) && isSpace(html[start]) {
		start++
	}
	if start >= len(html) || html[start] != '{' {
		return nil, errors.New("window.initialData object not found")
	}

	depth := 0
	inString := false
	escaped := false
	for i := start; i < len(html); i++ {
		ch := html[i]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			switch ch {
			case '\\':
				escaped = true
			case '"':
				inString = false
			}
			continue
		}
		switch ch {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return html[start : i+1], nil
			}
		}
	}
	return nil, errors.New("window.initialData object is incomplete")
}

func isSpace(ch byte) bool {
	return ch == ' ' || ch == '\n' || ch == '\r' || ch == '\t'
}
