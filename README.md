# warcprox - WARC writing MITM HTTP/S proxy

MITM Proxy that records all requests and responses in WARC files.
Inspired by [internetarchive/warcprox](https://github.com/internetarchive/warcprox) and based on [goproxy](https://github.com/elazarl/goproxy).

This is a proof-of-concept that focuses on optimizing browsing speed. It does
not implement WARC deduplication or revisits, and currently has no configuration
options. The CA certificate is embedded in `certs.go`.

## Install

```
go get github.com/xconstruct/warcprox
warcprox -v
```
