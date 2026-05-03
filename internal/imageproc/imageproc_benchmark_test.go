package imageproc

import (
	"bytes"
	"fmt"
	"image/jpeg"
	"image/png"
	"testing"
)

func BenchmarkSniff(b *testing.B) {
	benchmarks := []struct {
		name string
		data []byte
	}{
		{name: "jpeg", data: benchmarkEncodeJPEG(b)},
		{name: "png", data: benchmarkEncodePNG(b)},
		{name: "webp", data: append([]byte("RIFF\x10\x00\x00\x00WEBPVP8 "), make([]byte, 16)...)},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, _ = Sniff(bm.data)
			}
		})
	}
}

func BenchmarkDecode(b *testing.B) {
	benchmarks := []struct {
		name string
		data []byte
		mime string
	}{
		{name: "jpeg", data: benchmarkEncodeJPEG(b), mime: "image/jpeg"},
		{name: "png", data: benchmarkEncodePNG(b), mime: "image/png"},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, _ = Decode(bm.data, bm.mime)
			}
		})
	}
}

func BenchmarkThumbnailJPEG(b *testing.B) {
	data := benchmarkEncodePNG(b)

	for _, size := range []int{100, 300} {
		b.Run(fmt.Sprintf("%dx%d", size, size), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, _ = ThumbnailJPEG(data, size)
			}
		})
	}
}

func benchmarkEncodeJPEG(b *testing.B) []byte {
	b.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, testImage(), nil); err != nil {
		b.Fatal(err)
	}
	return buf.Bytes()
}

func benchmarkEncodePNG(b *testing.B) []byte {
	b.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, testImage()); err != nil {
		b.Fatal(err)
	}
	return buf.Bytes()
}
