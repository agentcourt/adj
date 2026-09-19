package pievent

import (
	"bytes"
	"io"
)

type Writer struct {
	dst     io.Writer
	observe func([]byte)
	buffer  []byte
}

func NewWriter(dst io.Writer, observe func([]byte)) *Writer {
	return &Writer{dst: dst, observe: observe}
}

func (w *Writer) Write(p []byte) (int, error) {
	n, err := w.dst.Write(p)
	w.buffer = append(w.buffer, p[:n]...)
	for {
		end := bytes.IndexByte(w.buffer, '\n')
		if end < 0 {
			break
		}
		w.observe(w.buffer[:end])
		w.buffer = w.buffer[end+1:]
	}
	return n, err
}

func (w *Writer) Flush() {
	if len(w.buffer) > 0 {
		w.observe(w.buffer)
		w.buffer = nil
	}
}
