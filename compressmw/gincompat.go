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
			c.Writer = &ginCompatWriter{
				grw:        c.Writer,
				realWriter: &compressingWriter{rw: c.Writer, compressor: zipwriter},
				status:     0,
			}
			c.Next()
			return
		}
	}
}

// ginCompatWriter implements all 10 billion methods of gin.ResponseWriter
// in order to write a simple middleware.
// I _strongly_ dislike gin, but it's what we already use...
type ginCompatWriter struct {
	grw        gin.ResponseWriter
	realWriter http.ResponseWriter
	status     int
}

var _ gin.ResponseWriter = (*ginCompatWriter)(nil)

func (g *ginCompatWriter) Flush()                                       { g.grw.Flush() }
func (g *ginCompatWriter) Pusher() http.Pusher                          { return g.grw.Pusher() }
func (g *ginCompatWriter) Header() http.Header                          { return g.grw.Header() }
func (g *ginCompatWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) { return g.grw.Hijack() }
func (g *ginCompatWriter) WriteString(s string) (int, error)            { return g.Write([]byte(s)) }
func (g *ginCompatWriter) Status() int {
	if g.status != 0 {
		return g.status
	}
	return g.grw.Status()
}

func (g *ginCompatWriter) Written() bool { return g.grw.Written() }
func (g *ginCompatWriter) Write(data []byte) (int, error) {
	g.WriteHeader(http.StatusOK)
	return g.realWriter.Write(data)
}

func (g *ginCompatWriter) Size() int { return g.grw.Size() }
func (g *ginCompatWriter) WriteHeader(code int) {
	if g.status == 0 {
		g.status = code
		g.realWriter.WriteHeader(code)
	}
}

func (g *ginCompatWriter) WriteHeaderNow()          { g.grw.WriteHeaderNow() }
func (g *ginCompatWriter) CloseNotify() <-chan bool { return g.grw.CloseNotify() }
