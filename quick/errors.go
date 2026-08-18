package quick

import (
	"strings"
	"unicode"
)

type sanitizedError struct {
	err     error
	message string
}

func (e *sanitizedError) Error() string { return e.message }
func (e *sanitizedError) Unwrap() error { return e.err }

func sanitizeError(err error) error {
	if err == nil {
		return nil
	}
	message := sanitizeText(err.Error())
	if message == err.Error() {
		return err
	}
	return &sanitizedError{err: err, message: message}
}

func sanitizeText(value string) string {
	var result strings.Builder
	remaining := value
	for {
		scheme := strings.Index(remaining, "://")
		if scheme < 0 {
			result.WriteString(remaining)
			return result.String()
		}
		tokenEnd := scheme + 3
		for tokenEnd < len(remaining) {
			r := rune(remaining[tokenEnd])
			if unicode.IsSpace(r) || strings.ContainsRune(`"'<>[](){},;`, r) {
				break
			}
			tokenEnd++
		}
		queryOffset := strings.IndexByte(remaining[scheme+3:tokenEnd], '?')
		if queryOffset < 0 {
			result.WriteString(remaining[:tokenEnd])
			remaining = remaining[tokenEnd:]
			continue
		}
		query := scheme + 3 + queryOffset
		result.WriteString(remaining[:query+1])
		result.WriteString("[redacted]")
		remaining = remaining[tokenEnd:]
	}
}
