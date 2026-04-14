package imageproc

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"
)

func TestSniff(t *testing.T) {
	jpegData := encodeJPEG(t)
	pngData := encodePNG(t)
	webpData := append([]byte("RIFF\x10\x00\x00\x00WEBPVP8 "), make([]byte, 16)...)

	tests := []struct {
		name    string
		data    []byte
		want    string
		wantErr bool
	}{
		{name: "jpeg", data: jpegData, want: "image/jpeg"},
		{name: "png", data: pngData, want: "image/png"},
		{name: "webp", data: webpData, want: "image/webp"},
		{name: "invalid", data: []byte("not image"), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Sniff(tt.data)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Sniff() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("Sniff() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestThumbnailJPEG(t *testing.T) {
	data := encodePNG(t)
	thumb, err := ThumbnailJPEG(data, 100)
	if err != nil {
		t.Fatalf("ThumbnailJPEG() error = %v", err)
	}
	if mime, err := Sniff(thumb); err != nil || mime != "image/jpeg" {
		t.Fatalf("thumbnail mime = %q, err %v, want image/jpeg", mime, err)
	}
	img, err := jpeg.Decode(bytes.NewReader(thumb))
	if err != nil {
		t.Fatalf("decode thumbnail: %v", err)
	}
	if got := img.Bounds().Dx(); got != 100 {
		t.Fatalf("thumbnail width = %d, want 100", got)
	}
	if got := img.Bounds().Dy(); got != 100 {
		t.Fatalf("thumbnail height = %d, want 100", got)
	}
}

func encodeJPEG(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, testImage(), nil); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func encodePNG(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, testImage()); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func testImage() image.Image {
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 20), G: uint8(y * 20), B: 180, A: 255})
		}
	}
	return img
}
