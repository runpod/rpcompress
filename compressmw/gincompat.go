// gincompat.go creates gin-compatible versions of the ServerAcceptCompression and ServerCompressBodies functions from compressmw.go.
// I'd prefer we don't use gin, but since we already do, we need to make sure our middleware is compatible with it.
package compressmw

import (
	"bufio"
	"net"
	"net/http"

	"github.com/gin-gonic/gin"
)

func GinAcceptGzip(c *gin.Context) {
	switch c.Request.Header.Get("Content-Encoding") {
	default:
		c.Next()
		return
	case "gzip", "x-gzip":
		c.Request.Header.Del("Content-Encoding")
		defer c.Request.Body.Close()
		zipreader := getzipreader(c.Request.Body)
		defer putzipreader(zipreader)
		c.Request.Body = zipreader
		c.Next()
		return
	}
}

// GinGzipBodies is a gin.HandlerFunc that compresses the response body with gzip if the client accepts it. Level is in the range 1(gzip.BestSpeed) to 9(gzip.BestCompression). 0 or -1 default to 6.
func GinGzipBodies(lvl int) gin.HandlerFunc {
	lvl = checkgziplevel(lvl)
	return func(c *gin.Context) {
		switch c.Request.Header.Get("Accept-Encoding") {
		default:
			c.Next()
			return
		case "gzip", "x-gzip":
			c.Writer.Header().Set("Content-Encoding", "gzip")
			zipwriter := getzipwriter(c.Writer, lvl)
			defer putzipwriter(zipwriter, lvl)
			c.Writer = &ginCompatGzipWriter{c.Writer, gzipWriter{rw: c.Writer, gzipw: zipwriter}}
			c.Next()
			return
		}
	}
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
