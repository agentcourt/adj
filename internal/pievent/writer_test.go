package pievent

import (
	"bytes"
	"reflect"
	"testing"
)

func TestWriterPreservesOutputAndObservesSplitLines(t *testing.T) {
	var output bytes.Buffer
	var lines []string
	writer := NewWriter(&output, func(line []byte) { lines = append(lines, string(line)) })
	for _, part := range []string{"first", "\nsec", "ond\nlast"} {
		if _, err := writer.Write([]byte(part)); err != nil {
			t.Fatal(err)
		}
	}
	writer.Flush()
	if output.String() != "first\nsecond\nlast" || !reflect.DeepEqual(lines, []string{"first", "second", "last"}) {
		t.Fatalf("output=%q, lines=%q", output.String(), lines)
	}
}
