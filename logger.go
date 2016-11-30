package main

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"os"
	"sync"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/elazarl/goproxy"
	"github.com/elazarl/goproxy/transport"
	"github.com/satori/go.uuid"
	"github.com/slyrz/warc"
)

type Meta struct {
	req      *http.Request
	resp     *http.Response
	err      error
	t        time.Time
	sess     int64
	bodyPath string
	from     string
	body     io.ReadCloser
}

func (m *Meta) WriteTo(w io.Writer) (n int64, err error) {
	record := warc.NewRecord()
	record.Header.Set("WARC-Record-ID", "<url:uuid:"+uuid.NewV4().String()+">")
	if m.resp != nil {
		if m.resp.StatusCode == 304 {
			return 0, nil
		}
		record.Header.Set("WARC-Type", "response")
		record.Header.Set("WARC-Target-URI", m.resp.Request.URL.String())
		record.Header.Set("Content-Type", "application/http; msgtype=response")
	} else {
		record.Header.Set("WARC-Type", "request")
		record.Header.Set("WARC-Target-URI", m.req.URL.String())
		record.Header.Set("Content-Type", "application/http; msgtype=request")
	}
	record.Header.Set("WARC-Date", time.Now().UTC().Format(time.RFC3339))
	// record.Header.Set("WARC-IP-Address", "")
	// record.Header.Set("WARC-Warcinfo-ID", "")

	var buf []byte
	if m.resp != nil {
		buf, err = httputil.DumpResponse(m.resp, false)
	} else {
		buf, err = httputil.DumpRequest(m.req, false)
	}
	if err != nil {
		return 0, err
	}
	record.Content = bytes.NewReader(buf)
	if m.body != nil {
		record.Content = io.MultiReader(record.Content, m.body)
		go func() {
			time.Sleep(1 * time.Minute)
			m.body.Close()
		}()
	}

	writer := warc.NewWriter(w)
	nr, err := writer.WriteRecord(record)
	return int64(nr), err
}

// HttpLogger is an asynchronous HTTP request/response logger. It traces
// requests and responses headers in a "log" file in logger directory and dumps
// their bodies in files prefixed with the session identifiers.
// Close it to ensure pending items are correctly logged.
type HttpLogger struct {
	path  string
	c     chan *bytes.Buffer
	errch chan error

	mutex   sync.Mutex
	blocked int
}

func NewLogger(basepath string) (*HttpLogger, error) {
	logger := &HttpLogger{basepath, make(chan *bytes.Buffer, 10), make(chan error, 10), sync.Mutex{}, 0}
	for i := 0; i < 10; i++ {
		go logger.RunWriter(basepath, i)
	}
	return logger, nil
}

func (logger *HttpLogger) RunWriter(basepath string, i int) {
	fname := fmt.Sprintf("%s/prox-%05d.warc.gz", basepath, i)
	f, err := os.OpenFile(fname, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		spew.Dump(err)
		logger.errch <- f.Close()
		return
	}
	w := gzip.NewWriter(f)
	for buf := range logger.c {
		logger.mutex.Lock()
		logger.blocked++
		log.Printf("blocked: %d", logger.blocked)
		logger.mutex.Unlock()
		if _, err := buf.WriteTo(w); err != nil {
			log.Println("Can't write meta", err)
		}
		w.Close()
		w.Reset(f)
		logger.mutex.Lock()
		logger.blocked--
		log.Printf("blocked: %d", logger.blocked)
		logger.mutex.Unlock()
	}
	logger.errch <- f.Close()
}

func (logger *HttpLogger) LogResp(resp *http.Response, ctx *goproxy.ProxyCtx) {
	from := ""
	if ctx.UserData != nil {
		from = ctx.UserData.(*transport.RoundTripDetails).TCPAddr.String()
	}
	r, w := io.Pipe()
	resp.Body = NewTeeReadCloser(resp.Body, w)
	go logger.LogMeta(&Meta{
		resp: resp,
		err:  ctx.Error,
		t:    time.Now(),
		sess: ctx.Session,
		body: r,
		from: from})
}

var emptyResp = &http.Response{}
var emptyReq = &http.Request{}

func (logger *HttpLogger) LogReq(req *http.Request, ctx *goproxy.ProxyCtx) {
	if req == nil {
		req = emptyReq
	}
	go logger.LogMeta(&Meta{
		req:  req,
		err:  ctx.Error,
		t:    time.Now(),
		sess: ctx.Session,
		from: req.RemoteAddr})
}

func (logger *HttpLogger) LogMeta(m *Meta) {
	buf := &bytes.Buffer{}
	if _, err := m.WriteTo(buf); err != nil {
		log.Println("Can't write meta", err)
	}
	logger.c <- buf
}

func (logger *HttpLogger) Close() error {
	close(logger.c)
	return <-logger.errch
}
