# rpcompress

rpcompress contains http middleware for clients and servers sending gzip-compressed responses.

## Go:

See [./compressmw](./compressmw/) for the middleware. See the [tests](./compressmw/compressmw_test.go) for many examples of the middleware in use.

### Clients:
- Decompression is automatically implemented by the standard `http.Transport` when the `Accept-Encoding` header is set to `gzip`.
- Wrap your http client's transport with `compressmw.ClientGzipResponse` to compress outgoing responses:
```go
import (
    "net/http"
    "github.com/runpod/rpcompress/compressmw"
)
var client = &http.Client{Transport: compressmw.ClientGzipBody(http.DefaultTransport)}

func doMyRequest() {
    body := strings.NewReader("some request body")
    req, _ := http.NewRequest("GET", "http://example.com", body) // will be automatically compressed by the transport
    resp, err := client.Do(req)
    if err != nil {
        // handle error
    }
    if resp.StatusCode != http.StatusOK {
        // handle error
    }
    b, err := ioutil.ReadAll(resp.Body) // will be automatically decompressed   
}
```

### Servers:
- Decompress incoming requests with `compressmw.ServerAcceptGzip`:
- Compress outgoing responses that set the `Accept-Encoding` header to `gzip` with `compressmw.GzipResponseBody`:
```go
import (
    "net/http"
    "github.com/runpod/rpcompress/compressmw"
)




func main() {
    var echo http.HandlerFunc = func(w http.ResponseWriter, r *http.Request) {
        // echo request body
        io.Copy(w, r.Body)
    }
    handler = compressmw.ServerAcceptGzip(handler)
    handler = compressmw.GzipResponseBody(handler)
    http.ListenAndServe(":8080", handler)
}

```
