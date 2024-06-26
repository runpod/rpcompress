// gincompat.go creates gin-compatible versions of the ServerAcceptCompression and ServerCompressBodies functions from compressmw.go.
// I'd prefer we don't use gin, but since we already do, we need to make sure our middleware is compatible with it.
package compressmw

import (
	"bufio"
	"io"
	"net"
	"net/http"

	"github.com/andybalholm/brotli"
	"github.com/gin-gonic/gin"
)

func GinAcceptGzip(c *gin.Context) {
	i := hasGzipAt(c.Request.Header.Values("Content-Encoding"))
	if i == -1 {
		c.Next() // we didn't find a gzip encoding, so we can skip the rest of this middleware.
		return
	}
	// remove the gzip encoding from the header.
	c.Request.Header["Content-Encoding"] = append(c.Request.Header["Content-Encoding"][:i], c.Request.Header["Content-Encoding"][i+1:]...)
	defer c.Request.Body.Close()

	// replace the request body with a streaming, decompressing reader.
	zipreader := getzipreader(c.Request.Body)
	defer putzipreader(zipreader)
	c.Request.Body = zipreader
	c.Next()
}

// GinGzipOrBrotliBodies is a gin.HandlerFunc that sniffs the client's Accept-Encoding header for 'br', 'gzip', or 'x-gzip',
// and compresses the response body with brotli or gzip, respectively, setting the response's Content-Encoding header accordingly.
func GinGzipOrBrotliBodies(c *gin.Context) {
	wc := brotli.HTTPCompressor(c.Writer, c.Request)
	defer wc.Close()
	c.Writer = &ginCompatGzipOrBrotliWriter{ginResponseWriter: c.Writer, compressWriter: wc}
	c.Next()
}

// GinGzipBodies is a gin.HandlerFunc that compresses the response body with gzip if the client accepts it. Level is in the range 1(gzip.BestSpeed) to 9(gzip.BestCompression). 0 or -1 default to 6.
func GinGzipBodies(lvl int) gin.HandlerFunc {
	lvl = checkgziplevel(lvl)
	return func(c *gin.Context) {
		i := hasGzipAt(c.Request.Header.Values("Accept-Encoding"))
		if i == -1 {
			// we didn't find a gzip encoding, so we can skip the rest of this middleware.
			c.Next()
			return
		}

		// remove 'accept-encoding: gzip' from the header: we don't want something later down the line to do it again.
		c.Request.Header["Accept-Encoding"] = append(c.Request.Header["Accept-Encoding"][:i], c.Request.Header["Accept-Encoding"][i+1:]...)
		// set the response header to indicate we're sending gzip.
		// then replace the response writer with a streaming, compressing writer.
		c.Writer.Header().Set("Content-Encoding", "gzip")
		zipwriter := getzipwriter(c.Writer, lvl)
		defer putzipwriter(zipwriter, lvl)
		c.Writer = &ginCompatGzipWriter{c.Writer, gzipWriter{rw: c.Writer, gzipw: zipwriter}}
		c.Next()
	}
}

type ginCompatGzipOrBrotliWriter struct {
	ginResponseWriter gin.ResponseWriter
	compressWriter    io.WriteCloser
	status            int
}

var _ gin.ResponseWriter = (*ginCompatGzipOrBrotliWriter)(nil)

func (g *ginCompatGzipOrBrotliWriter) Flush()              { g.ginResponseWriter.Flush() }
func (g *ginCompatGzipOrBrotliWriter) Pusher() http.Pusher { return g.ginResponseWriter.Pusher() }
func (g *ginCompatGzipOrBrotliWriter) Header() http.Header { return g.ginResponseWriter.Header() }
func (g *ginCompatGzipOrBrotliWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return g.ginResponseWriter.Hijack()
}

func (g *ginCompatGzipOrBrotliWriter) Status() int {
	if g.status != 0 {
		return g.status
	}
	return g.ginResponseWriter.Status()
}
func (g *ginCompatGzipOrBrotliWriter) WriteString(s string) (int, error) { return g.Write([]byte(s)) }
func (g *ginCompatGzipOrBrotliWriter) Written() bool                     { return g.ginResponseWriter.Written() }
func (g *ginCompatGzipOrBrotliWriter) Size() int                         { return g.ginResponseWriter.Size() }
func (g *ginCompatGzipOrBrotliWriter) CloseNotify() <-chan bool {
	return g.ginResponseWriter.CloseNotify()
}

func (g *ginCompatGzipOrBrotliWriter) WriteHeader(code int) {
	g.status = code
	g.ginResponseWriter.WriteHeader(code)
}

func (g *ginCompatGzipOrBrotliWriter) WriteHeaderNow() {
	if g.status == 0 {
		g.status = http.StatusOK
	}
	g.ginResponseWriter.WriteHeader(g.status)
}

func (g *ginCompatGzipOrBrotliWriter) Write(data []byte) (int, error) {
	if g.status == 0 {
		g.status = http.StatusOK
	}
	return g.compressWriter.Write(data)
}

// ginCompatGzipWriter implements all 10 billion methods of gin.ResponseWriter
// in order to write a simple middleware.
// I _strongly_ dislike gin, but it's what we already use...
type ginCompatGzipWriter struct {
	ginResponseWriter gin.ResponseWriter
	gzipw             gzipWriter
}

var _ gin.ResponseWriter = (*ginCompatGzipWriter)(nil)

func (g *ginCompatGzipWriter) Flush()              { g.ginResponseWriter.Flush() }
func (g *ginCompatGzipWriter) Pusher() http.Pusher { return g.ginResponseWriter.Pusher() }
func (g *ginCompatGzipWriter) Header() http.Header { return g.ginResponseWriter.Header() }
func (g *ginCompatGzipWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return g.ginResponseWriter.Hijack()
}
func (g *ginCompatGzipWriter) WriteString(s string) (int, error) { return g.Write([]byte(s)) }
func (g *ginCompatGzipWriter) Status() int {
	if g.gzipw.status != 0 {
		return g.gzipw.status
	}
	return g.ginResponseWriter.Status()
}

func (g *ginCompatGzipWriter) Written() bool { return g.ginResponseWriter.Written() }
func (g *ginCompatGzipWriter) Write(data []byte) (int, error) {
	g.gzipw.WriteHeader(http.StatusOK)
	return g.gzipw.Write(data)
}

func (g *ginCompatGzipWriter) Size() int            { return g.ginResponseWriter.Size() }
func (g *ginCompatGzipWriter) WriteHeader(code int) { g.gzipw.WriteHeader(code) }

func (g *ginCompatGzipWriter) WriteHeaderNow()          { g.ginResponseWriter.WriteHeaderNow() }
func (g *ginCompatGzipWriter) CloseNotify() <-chan bool { return g.ginResponseWriter.CloseNotify() }
