package compressmw

import (
	"compress/gzip"
	"fmt"
	"net/http"
)

type gzipWriter struct {
	gzipw  *gzip.Writer        // should wrap rw
	rw     http.ResponseWriter // the underlying ResponseWriter
	status int                 // the HTTP response code from the first call to WriteHeader
}

func checkgziplevel(lvl int) int {
	switch lvl {
	case gzip.DefaultCompression, 0:
		return 6 // default to 6
	case 1, 2, 3, 4, 5, 6, 7, 8, 9:
		return lvl
	default:
		panic(fmt.Errorf("invalid gzip compression level: expected -1 <= level <= 9, got %d", lvl))
	}
}

// Write writes the compressed data to the underlying ResponseWriter.
func (cw *gzipWriter) WriteHeader(code int) {
	if cw.status != 0 {
		return
	}
	cw.status = code
	cw.rw.WriteHeader(code)
}

func (cw *gzipWriter) Write(b []byte) (int, error) {
	cw.WriteHeader(http.StatusOK)
	return cw.gzipw.Write(b)
}

// Header returns the header map of the underlying ResponseWriter.
func (cw *gzipWriter) Header() http.Header { return cw.rw.Header() }

// ServerAcceptGzip transparently decompresses incoming requests with a Content-Encoding of "gzip" or "x-gzip".
// It does not handle "deflate", "br", "zstd", or any other encoding - use separate middleware for those.
// See ServerGzipBodies for compressing outgoing responses,
// and ClientCompressBodyWithGzip for compressing outgoing requests to be READ by this middleware.
func ServerAcceptGzip(h http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Encoding") == "gzip" {
			r.Header.Del("Content-Encoding")
			defer r.Body.Close()
			zipreader := getzipreader(r.Body)
			defer putzipreader(zipreader)
			r.Body = zipreader
		}
		h.ServeHTTP(w, r)
	}
}

// ServerGzipResponseBody compresses outgoing responses with gzip if the client accepts it.
// That is, if the client sends "Accept-Encoding: gzip" in the request header,
// the response body will be compressed with gzip and sent with "Content-Encoding: gzip" in the response header.
// Level is in the range 1(gzip.BestSpeed) to 9(gzip.BestCompression). 0 or -1 default to 6
//
// See ServerAcceptGzip for decompressing incoming requests, and ClientCompressBodyWithGzip for compressing outgoing requests.
// This does not handle "deflate", "br", "zstd", or any other encoding - use separate middleware for those.
func ServerGzipResponseBody(h http.Handler, lvl int) http.HandlerFunc {
	lvl = checkgziplevel(lvl)
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept-Encoding") == "gzip" {
			w.Header().Set("Content-Encoding", "gzip")
			gzipw := getzipwriter(w, lvl)
			defer putzipwriter(gzipw, lvl)
			h.ServeHTTP(&gzipWriter{rw: w, gzipw: gzipw}, r)
		}
	}
}

// Unwrap returns the underlying ResponseWriter.
func (cw *gzipWriter) Unwrap() http.ResponseWriter { return cw.rw }
