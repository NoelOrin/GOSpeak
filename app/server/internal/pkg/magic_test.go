package pkg

import "testing"

func TestDetectContentType(t *testing.T) {
	cases := []struct {
		name string
		data []byte
		want string
	}{
		{name: "jpeg", data: []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10}, want: "image/jpeg"},
		{name: "png", data: []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}, want: "image/png"},
		{name: "gif", data: []byte("GIF89a"), want: "image/gif"},
		{name: "webp", data: append([]byte("RIFF"), append(make([]byte, 4), []byte("WEBP")...)...), want: "image/webp"},
		{name: "plain", data: []byte("hello"), want: "text/plain; charset=utf-8"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := DetectContentType(tc.data); got != tc.want {
				t.Fatalf("DetectContentType(%q) = %q, want %q", tc.name, got, tc.want)
			}
		})
	}
}
