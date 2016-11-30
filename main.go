package main

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"

	"github.com/davecgh/go-spew/spew"
	"github.com/elazarl/goproxy"
)

func orPanic(err error) {
	if err != nil {
		panic(err)
	}
}

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func init() {
	ca, err := tls.X509KeyPair(CA_CERT, CA_KEY)
	must(err)
	ca.Leaf, err = x509.ParseCertificate(ca.Certificate[0])
	must(err)
	goproxy.GoproxyCa = ca

	goproxy.OkConnect.TLSConfig = goproxy.TLSConfigFromCA(&ca)
	goproxy.MitmConnect.TLSConfig = goproxy.TLSConfigFromCA(&ca)
	goproxy.HTTPMitmConnect.TLSConfig = goproxy.TLSConfigFromCA(&ca)
	goproxy.RejectConnect.TLSConfig = goproxy.TLSConfigFromCA(&ca)
}

func main() {
	verbose := flag.Bool("v", false, "should every proxy request be logged to stdout")
	addr := flag.String("l", ":8080", "on which address should the proxy listen")
	flag.Parse()

	proxy := goproxy.NewProxyHttpServer()
	proxy.Tr.TLSClientConfig = nil
	proxy.Verbose = *verbose

	if err := os.MkdirAll("warcs", 0755); err != nil {
		log.Fatal("Can't create dir", err)
	}
	logger, err := NewLogger("warcs")
	if err != nil {
		log.Fatal("can't open log file", err)
	}

	proxy.OnRequest().
		HandleConnect(goproxy.AlwaysMitm)
	proxy.OnRequest().
		DoFunc(func(req *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
		logger.LogReq(req, ctx)
		return req, nil
	})
	proxy.OnResponse().
		DoFunc(func(resp *http.Response, ctx *goproxy.ProxyCtx) *http.Response {
		spew.Dump(ctx.RoundTripper)
		logger.LogResp(resp, ctx)
		return resp
	})

	l, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatal("listen:", err)
	}
	ch := make(chan os.Signal)
	signal.Notify(ch, os.Interrupt)
	go func() {
		<-ch
		log.Println("Got SIGINT exiting")
		l.Close()
	}()
	log.Println("Starting Proxy")
	http.Serve(l, proxy)
	logger.Close()
	log.Println("All connections closed - exit")
}
