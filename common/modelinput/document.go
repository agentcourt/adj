package modelinput

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/jsmorph/adj/common/documents"
)

func DocumentContentItem(document documents.File, raw []byte) (map[string]any, error) {
	mediaType := strings.ToLower(strings.TrimSpace(strings.Split(document.MediaType, ";")[0]))
	switch {
	case strings.HasPrefix(mediaType, "image/"):
		return map[string]any{
			"type":      "input_image",
			"image_url": dataURL(mediaType, raw),
			"detail":    "auto",
		}, nil
	case mediaType == "application/pdf":
		return map[string]any{
			"type":      "input_file",
			"file_data": dataURL(mediaType, raw),
			"filename":  filepath.Base(filepath.FromSlash(document.Path)),
		}, nil
	case utf8.Valid(raw) && !bytes.ContainsRune(raw, '\x00'):
		return map[string]any{"type": "input_text", "text": string(raw)}, nil
	default:
		return nil, fmt.Errorf("document %q has unsupported media type %q", document.Path, document.MediaType)
	}
}

func dataURL(mediaType string, raw []byte) string {
	return "data:" + mediaType + ";base64," + base64.StdEncoding.EncodeToString(raw)
}
