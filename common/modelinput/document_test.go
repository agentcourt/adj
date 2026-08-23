package modelinput

import (
	"reflect"
	"strings"
	"testing"

	"github.com/agentcourt/adj/common/documents"
)

func TestDocumentContentItem(t *testing.T) {
	tests := []struct {
		name     string
		document documents.File
		raw      []byte
		want     map[string]any
	}{
		{
			name:     "text",
			document: documents.File{Path: "facts.txt", MediaType: "text/plain; charset=utf-8"},
			raw:      []byte("evidence"),
			want:     map[string]any{"type": "input_text", "text": "evidence"},
		},
		{
			name:     "image",
			document: documents.File{Path: "figure.png", MediaType: "IMAGE/PNG"},
			raw:      []byte{0x89, 'P', 'N', 'G'},
			want: map[string]any{
				"type":      "input_image",
				"image_url": "data:image/png;base64,iVBORw==",
				"detail":    "auto",
			},
		},
		{
			name:     "pdf",
			document: documents.File{Path: "sub/report.pdf", MediaType: "application/pdf"},
			raw:      []byte("%PDF"),
			want: map[string]any{
				"type":      "input_file",
				"file_data": "data:application/pdf;base64,JVBERg==",
				"filename":  "report.pdf",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := DocumentContentItem(test.document, test.raw)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("content item = %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestDocumentContentItemRejectsUnsupportedBinary(t *testing.T) {
	_, err := DocumentContentItem(documents.File{Path: "archive.bin", MediaType: "application/octet-stream"}, []byte{0, 1, 2})
	if err == nil || !strings.Contains(err.Error(), `document "archive.bin" has unsupported media type "application/octet-stream"`) {
		t.Fatalf("error = %v", err)
	}
}
