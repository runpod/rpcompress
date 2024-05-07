package compressmw

import (
	"compress/gzip"
	"fmt"
	"net/http"
	"strings"
)

// hasGzipAt returns the index of "gzip" or "x-gzip" in headers, or -1 if it's not present.
// it splits on commas, so it can handle "br, gzip" or "gzip, br".
// usually called as follows:
func hasGzipAt(headers []string) int {
	for i := range headers {
		if !strings.Contains(headers[i], "gzip") {
			continue
		}
		for _, v := range strings.Split(headers[i], ",") {
			switch strings.TrimSpace(v) {
			case "gzip", "x-gzip":
				return i
			}
		}
		// this should not be reachable, but particularly bad input could cause it.
		// move on to the next header.
	}
	return -1 //
}

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
		i := hasGzipAt(r.Header.Values("Content-Encoding"))
		if i == -1 { // not gzip-encoded. pass it through.
			h.ServeHTTP(w, r)
			return
		}
		// remove 'content-encoding: gzip' from the header: we don't want something later down the line to do it again.
		r.Header["Content-Encoding"] = append(r.Header["Content-Encoding"][:i], r.Header["Content-Encoding"][i+1:]...)

		// replace the request body with a streaming, decompressing reader.
		defer r.Body.Close()
		zipreader := getzipreader(r.Body)
		defer putzipreader(zipreader)
		r.Body = zipreader

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
		acceptEncoding := r.Header.Values("Accept-Encoding")
		i := hasGzipAt(acceptEncoding)
		if i == -1 {
			// we didn't find a gzip encoding, so we can skip the rest of this middleware.
			h.ServeHTTP(w, r)
			return
		}
		// remove 'accept-encoding: gzip' from the header: we don't want something later down the line to do it again.
		r.Header["Accept-Encoding"] = append(acceptEncoding[:i], acceptEncoding[i+1:]...)
		w.Header().Add("Content-Encoding", "gzip")
		// replace the response writer with a streaming, compressing writer.
		gzipw := getzipwriter(w, lvl)
		defer putzipwriter(gzipw, lvl)
		h.ServeHTTP(&gzipWriter{rw: w, gzipw: gzipw}, r)
	}
}

// Unwrap returns the underlying ResponseWriter.
func (cw *gzipWriter) Unwrap() http.ResponseWriter { return cw.rw }
