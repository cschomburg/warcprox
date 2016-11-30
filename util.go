package main

import (
	"errors"
	"fmt"
	"io"
	"os"
)

func fprintf(nr *int64, err *error, w io.Writer, pat string, a ...interface{}) {
	if *err != nil {
		return
	}
	var n int
	n, *err = fmt.Fprintf(w, pat, a...)
	*nr += int64(n)
}

func write(nr *int64, err *error, w io.Writer, b []byte) {
	if *err != nil {
		return
	}
	var n int
	n, *err = w.Write(b)
	*nr += int64(n)
}

type FileStream struct {
	path string
	f    *os.File
}

func NewFileStream(path string) *FileStream {
	return &FileStream{path, nil}
}

func (fs *FileStream) Write(b []byte) (nr int, err error) {
	if fs.f == nil {
		fs.f, err = os.Create(fs.path)
		if err != nil {
			return 0, err
		}
	}
	return fs.f.Write(b)
}

func (fs *FileStream) Close() error {
	fmt.Println("Close", fs.path)
	if fs.f == nil {
		return errors.New("FileStream was never written into")
	}
	return fs.f.Close()
}

// TeeReadCloser extends io.TeeReader by allowing reader and writer to be
// closed.
type TeeReadCloser struct {
	r io.ReadCloser
	w io.WriteCloser
}

func NewTeeReadCloser(r io.ReadCloser, w io.WriteCloser) io.ReadCloser {
	return &TeeReadCloser{r, w}
}

func (t *TeeReadCloser) Read(p []byte) (int, error) {
	n, err := t.r.Read(p)
	if n > 0 {
		t.w.Write(p[:n])
		return n, err
	}
	return n, err
}

// Close attempts to close the reader and write. It returns an error if both
// failed to Close.
func (t *TeeReadCloser) Close() error {
	err1 := t.r.Close()
	t.w.Close()
	return err1
}
