// sync.Pools for cutting allocation pressure under high load.
package compressmw

import (
	"bytes"
	"compress/gzip"
	"io"
	"sync"
)

var (
	// pools, indexed by gzip compression level.
	// we "waste" one to cut out a bit of arithmetic.
	writezippool = [10]sync.Pool{
		// technically, it would already nil pointer deref, but this is a bit more explicit.
		0: {New: func() interface{} { panic("this should never be called") }},
		1: {New: func() interface{} { return newWriterLevel(1) }},
		2: {New: func() interface{} { return newWriterLevel(2) }},
		3: {New: func() interface{} { return newWriterLevel(3) }},
		4: {New: func() interface{} { return newWriterLevel(4) }},
		5: {New: func() interface{} { return newWriterLevel(5) }},
		6: {New: func() interface{} { return newWriterLevel(6) }},
		7: {New: func() interface{} { return newWriterLevel(7) }},
		8: {New: func() interface{} { return newWriterLevel(8) }},
		9: {New: func() interface{} { return newWriterLevel(9) }},
	}

	readzippool = sync.Pool{New: func() interface{} { return new(gzip.Reader) }}
	bufpool     = sync.Pool{New: func() interface{} { return new(bytes.Buffer) }}
)

func getzipreader(r io.Reader) *gzip.Reader {
	z := readzippool.Get().(*gzip.Reader)
	z.Reset(r)
	return z
}

// eofreader is a reader that always returns io.EOF.
// we use it as a placeholder for the gzip.Reader's underlying reader when we want to stick a gzip.Reader back in the pool.
type eofreader struct{}

func (eofreader) Read(p []byte) (int, error) { return 0, io.EOF }

// putzipreader returns a *gzip.Reader to the pool.
func putzipreader(z *gzip.Reader) {
	z.Close()            // read to EOF
	z.Reset(eofreader{}) // get rid of our reference to z.r so the GC can collect it. eofreader is a ZST, so it's cheap to keep around.
	readzippool.Put(z)
}
func getbuf() *bytes.Buffer    { return bufpool.Get().(*bytes.Buffer) }
func putbuf(buf *bytes.Buffer) { buf.Reset(); bufpool.Put(buf) }

// getzipwriter initializes a *gzip.Writer from the pool using w.
func getzipwriter(w io.Writer, lvl int) *gzip.Writer {
	z := writezippool[lvl].Get().(*gzip.Writer)
	z.Reset(w)
	return z
}

func newWriterLevel(level int) *gzip.Writer {
	z, err := gzip.NewWriterLevel(nil, level)
	if err != nil {
		panic(err)
	}
	return z
}

func putzipwriter(z *gzip.Writer, lvl int) {
	z.Close()
	z.Reset(io.Discard)
	writezippool[lvl].Put(z)
}
