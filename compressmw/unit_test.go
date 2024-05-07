package compressmw

import "testing"

func TestHasGzipAt(t *testing.T) {
	for _, tt := range []struct {
		name   string
		header []string
		want   int
	}{
		{
			name: "no header",
			want: -1,
		},
		{
			name:   "no gzip",
			header: []string{"br, deflate", "something else"},
			want:   -1,
		},
		{
			name:   "gzip",
			header: []string{"gzip", "something else"},
			want:   0,
		},
		{
			name:   "x-gzip, later in the list",
			header: []string{"0", "1", "br, x-gzip"},
			want:   2,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasGzipAt(tt.header); got != tt.want {
				t.Errorf("hasGzipAt(%v) = %d, want %d", tt.header, got, tt.want)
			}
		})
	}
}
