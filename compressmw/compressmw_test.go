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

	"github.com/gin-gonic/gin"
	"github.com/runpod/rpcompress/compressmw"
)

// echo copies the incoming request body to the response body.
var echo http.HandlerFunc = func(w http.ResponseWriter, r *http.Request) { io.Copy(w, r.Body) }

func TestGzipRoundTrip(t *testing.T) {
	t.Parallel()
	for lvl := -1; lvl <= 9; lvl++ {
		lvl := lvl
		t.Run(fmt.Sprintf("l%+2d", lvl), func(t *testing.T) {
			t.Parallel()

			handler := compressmw.ServerAcceptGzip(echo)
			const want = "<this is the body>"
			s := httptest.NewServer(handler)
			t.Cleanup(s.Close)
			client := &http.Client{Transport: compressmw.ClientGzipBody(http.DefaultTransport, gzip.DefaultCompression)}
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
		t.Run(fmt.Sprintf("l%+2d", lvl), func(t *testing.T) {
			t.Parallel()
			router := gin.New()
			router.Use(compressmw.GinGzipBodies(lvl))
			router.POST("/foo", func(c *gin.Context) {
				io.Copy(c.Writer, c.Request.Body)
			})
			const want = "<this is the body>"
			s := httptest.NewServer(router)
			t.Cleanup(s.Close)
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
			defer resp.Body.Close()
		})
	}
}

func TestServerGzip(t *testing.T) {
	t.Parallel()
	for lvl := -1; lvl <= 9; lvl++ {
		lvl := lvl
		t.Run(fmt.Sprintf("l%+2d", lvl), func(t *testing.T) {
			t.Parallel()
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
			defer resp.Body.Close()
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
