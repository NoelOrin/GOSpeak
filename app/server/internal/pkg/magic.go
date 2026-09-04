package pkg

import (
	"bytes"
	"net/http"
)

var (
	pngMagic  = []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	riffMagic = []byte("RIFF")
	webpMagic = []byte("WEBP")
)

// DetectContentType 基于文件头魔数嗅探 MIME 类型。
// 优先识别 jpeg/png/gif/webp，其余类型回退到 http.DetectContentType。
func DetectContentType(header []byte) string {
	if len(header) >= 3 && header[0] == 0xFF && header[1] == 0xD8 && header[2] == 0xFF {
		return "image/jpeg"
	}
	if len(header) >= len(pngMagic) && bytes.Equal(header[:len(pngMagic)], pngMagic) {
		return "image/png"
	}
	if len(header) >= 6 && (bytes.HasPrefix(header, []byte("GIF87a")) || bytes.HasPrefix(header, []byte("GIF89a"))) {
		return "image/gif"
	}
	if len(header) >= 12 && bytes.Equal(header[:4], riffMagic) && bytes.Equal(header[8:12], webpMagic) {
		return "image/webp"
	}
	return http.DetectContentType(header)
}
