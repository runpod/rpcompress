package compressmw

import (
	"io"
	"net/http"
)

var _ http.RoundTripper = (*roundtripfunc)(nil)

type roundtripfunc func(*http.Request) (*http.Response, error)

func (rt roundtripfunc) RoundTrip(r *http.Request) (*http.Response, error) { return rt(r) }

// ClientGzipBody is a RoundTripper that compresses non-nil request bodies with gzip. Level is in the range 1(gzip.BestSpeed) to 9(gzip.BestCompression). 0 or -1 default to 6.
func ClientGzipBody(rt http.RoundTripper, level int) http.RoundTripper {
	level = checkgziplevel(level)

	return roundtripfunc(func(r *http.Request) (*http.Response, error) {
		if r.Body == nil {
			return rt.RoundTrip(r)
		}
		if b, ok := r.Body.(interface{ Len() int }); ok && b.Len() == 0 {
			return rt.RoundTrip(r)
		}
		// naive solution: read the entire body into memory, compress it, and send it.
		// I don't want to deal with pipes. If we get into streaming http bodies (usually a bad idea, but it happens)
		// we can revisit this.
		buf := getbuf()
		defer putbuf(buf)
		gw := getzipwriter(buf, level)
		io.Copy(gw, r.Body)
		gw.Close()
		r.Body = io.NopCloser(buf)
		r.ContentLength = int64(buf.Len())
		r.Header.Set("Content-Encoding", "gzip")
		return rt.RoundTrip(r)
	})
}
