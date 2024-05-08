package compressmw_test

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/andybalholm/brotli"
	"github.com/gin-gonic/gin"
	"github.com/runpod/rpcompress/compressmw"
)

// echo copies the incoming request body to the response body.
var echo http.HandlerFunc = func(w http.ResponseWriter, r *http.Request) { io.Copy(w, r.Body) }

func TestGzipRoundTrip(t *testing.T) {
	t.Parallel()
	for lvl := -1; lvl <= 9; lvl++ {
		lvl := lvl
		t.Run(fmt.Sprintf("%+2d", lvl), func(t *testing.T) {
			t.Parallel()
			testGzipRoundTrip(t, lvl)
		})
	}
}

func TestGinAcceptGzip(t *testing.T) {
	t.Parallel()
	const want = "<this is the body>"
	var src bytes.Buffer
	gw := gzip.NewWriter(&src)
	gw.Write([]byte(want))
	gw.Close()
	server := gin.New()
	server.Use(compressmw.GinAcceptGzip)
	server.POST("/foo", func(c *gin.Context) {
		io.Copy(c.Writer, c.Request.Body)
	})
	s := httptest.NewServer(server)
	t.Cleanup(s.Close)
	req, err := http.NewRequest("POST", s.URL+"/foo", bytes.NewReader(src.Bytes()))
	if err != nil {
		t.Fatal(err)
	}

	req.Header.Set("Content-Encoding", "gzip")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusOK)
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != want {
		t.Errorf("got %q, want %q", b, want)
	}
}

func TestGinGzipBodies(t *testing.T) {
	t.Parallel()
	for lvl := -1; lvl <= 9; lvl++ {
		lvl := lvl
		t.Run(fmt.Sprintf("%+2d", lvl), func(t *testing.T) {
			t.Parallel()
			testGinGzipBodies(t, lvl)
		})
	}
}

func TestServerGzip(t *testing.T) {
	t.Parallel()
	for lvl := -1; lvl <= 9; lvl++ {
		lvl := lvl
		t.Run(fmt.Sprintf("%+2d", lvl), func(t *testing.T) {
			t.Parallel()
			testServerGzip(t, lvl)
		})
	}
}

func TestServerAccept(t *testing.T) {
	const want = "<this is the body>"

	var src bytes.Buffer
	gw := gzip.NewWriter(&src)
	gw.Write([]byte(want))
	gw.Close()

	// first, try it WITHOUT our middleware and make sure it fails...
	req, err := http.NewRequest("POST", "/foo", bytes.NewReader(src.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	echo.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("got status %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Body.String(); got == want {
		t.Fatalf("I haven't accepted compression: why is this decompressed?")
	}

	// now try it WITH and show that it works

	rec = httptest.NewRecorder()
	req, err = http.NewRequest("POST", "/foo", bytes.NewReader(src.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Encoding", "gzip")

	compressmw.ServerAcceptGzip(echo).ServeHTTP(rec, req)
	got := rec.Body.String()
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestGinGzipOrBrotliBodies(t *testing.T) {
	for _, tt := range []struct {
		encoding string
		read     func(io.Reader) (string, error)
	}{
		{
			encoding: "gzip",
			read: func(r io.Reader) (string, error) {
				gzipR, err := gzip.NewReader(r)
				if err != nil {
					return "", err
				}
				b, err := io.ReadAll(gzipR)
				return string(b), err
			},
		},
		{
			encoding: "br",
			read: func(r io.Reader) (string, error) {
				b, err := io.ReadAll(brotli.NewReader(r))
				return string(b), err
			},
		},
	} {
		t.Run(tt.encoding, func(t *testing.T) {
			const wantBody = "<this is the body>"
			req, err := http.NewRequest("POST", "/foo", strings.NewReader(wantBody))
			if err != nil {
				t.Fatal(err)
			}
			// say we can accept brotli
			req.Header.Set("Accept-Encoding", tt.encoding)

			router := gin.New()
			router.Use(compressmw.GinGzipOrBrotliBodies) // set up the router to use the middleware when it sees "br" in the Accept-Encoding header
			router.POST("/foo", func(c *gin.Context) {
				io.Copy(c.Writer, c.Request.Body)
			})
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Errorf("got status %d, want %d", rec.Code, http.StatusOK)
			}
			if rec.Header().Get("Content-Encoding") != tt.encoding {
				t.Errorf("got Content-Encoding %q, want %q", rec.Header().Get("Content-Encoding"), "br")
			}
			if got, err := tt.read(rec.Body); err != nil {
				t.Errorf("error reading response body: %v", err)
			} else if got != wantBody {
				t.Errorf("got %q, want %q", got, wantBody)
			}
		})
	}
}

// implementation of TestGinGzipBodies per-level
func testGinGzipBodies(t *testing.T, lvl int) {
	router := gin.New()
	router.Use(compressmw.GinGzipBodies(lvl))
	router.POST("/foo", func(c *gin.Context) {
		io.Copy(c.Writer, c.Request.Body)
	})
	const want = "<this is the body>"
	s := httptest.NewServer(router)
	t.Cleanup(s.Close)
	t.Run("no-op", func(t *testing.T) {
		r, err := http.NewRequest("POST", s.URL+"/foo", strings.NewReader(want))
		if err != nil {
			t.Fatal(err)
		}
		resp, err := http.DefaultClient.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusOK)
		}
	})

	for _, headers := range [][]string{
		{"gzip", "something else"},
		{"0", "1", "br, x-gzip"},
		{"gzip, br"},
	} {
		t.Run(fmt.Sprintf("headers=%v", headers), func(t *testing.T) {
			req, err := http.NewRequest("POST", s.URL+"/foo", strings.NewReader(want))
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Accept-Encoding", "gzip")
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusOK)
			}
			if resp.Header.Get("Content-Encoding") != "gzip" {
				t.Errorf("got Content-Encoding %q, want %q", resp.Header.Get("Content-Encoding"), "gzip")
			}
			b, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatal(err)
			}
			if string(b) == want {
				t.Fatalf("already decompressed: got %q, want %q", b, want)
			}
			// now, decompress it ourselves
			reader, err := gzip.NewReader(bytes.NewReader(b))
			if err != nil {
				t.Fatal(err)
			}
			defer reader.Close()
			got, err := io.ReadAll(reader)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != want {
				t.Errorf("got %q, want %q", got, want)
			}
			// show that it gets transparently decompressed if we don't explicitly ask for compression
			req, err = http.NewRequest("POST", s.URL+"/foo", strings.NewReader(want))
			if err != nil {
				t.Fatal(err)
			}
			resp, err = http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			resp.Body.Close()
		})
	}
}

// implementation of TestGzipRoundTrip per-level
func testGzipRoundTrip(t *testing.T, lvl int) {
	handler := compressmw.ServerAcceptGzip(echo)
	const want = "<this is the body>"
	s := httptest.NewServer(handler)
	t.Cleanup(s.Close)
	client := &http.Client{Transport: compressmw.ClientGzipBody(http.DefaultTransport, lvl)}
	req, err := http.NewRequest("POST", s.URL+"/foo", strings.NewReader(want))
	if err != nil {
		t.Fatal(err)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusOK)
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != want {
		t.Errorf("got %q, want %q", b, want)
	}
}

// implementation of TestServerGzip per-level
func testServerGzip(t *testing.T, lvl int) {
	handler := compressmw.ServerGzipResponseBody(echo, lvl)
	const want = "<this is the body>"
	s := httptest.NewServer(handler)
	t.Cleanup(s.Close)
	// the go http client will automatically decompress the response, so we need to disable that to "prove"
	// that the response is compressed

	req, err := http.NewRequest("POST", s.URL+"/foo", strings.NewReader(want))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Accept-Encoding", "gzip")

	resp, err := (&http.Client{Transport: &http.Transport{DisableCompression: true}}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if resp.Header.Get("Content-Encoding") != "gzip" {
		t.Errorf("got Content-Encoding %q, want %q", resp.Header.Get("Content-Encoding"), "gzip")
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) == want {
		t.Fatalf("already decompressed: got %q, want %q", b, want)
	}
	// now, decompress it ourselves
	reader, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	got, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Errorf("got %q, want %q", got, want)
	}
	// show that it gets transparently decompressed if we don't explicitly ask for compression
	req, err = http.NewRequest("POST", s.URL+"/foo", strings.NewReader(want))
	if err != nil {
		t.Fatal(err)
	}
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
}
