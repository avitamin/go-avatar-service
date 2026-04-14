package imageproc

import (
	"bytes"
	"errors"
	"image"
	"image/jpeg"
	"image/png"
)

var ErrUnsupportedImage = errors.New("unsupported image")

func Sniff(data []byte) (string, error) {
	if len(data) >= 3 && data[0] == 0xff && data[1] == 0xd8 && data[2] == 0xff {
		return "image/jpeg", nil
	}
	if len(data) >= 8 && bytes.Equal(data[:8], []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}) {
		return "image/png", nil
	}
	if len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP" {
		return "image/webp", nil
	}
	return "", ErrUnsupportedImage
}

func Decode(data []byte, mime string) (image.Image, error) {
	switch mime {
	case "image/jpeg":
		return jpeg.Decode(bytes.NewReader(data))
	case "image/png":
		return png.Decode(bytes.NewReader(data))
	default:
		return nil, ErrUnsupportedImage
	}
}

func ThumbnailJPEG(data []byte, size int) ([]byte, error) {
	mime, err := Sniff(data)
	if err != nil {
		return nil, err
	}
	src, err := Decode(data, mime)
	if err != nil {
		return nil, err
	}
	dst := image.NewRGBA(image.Rect(0, 0, size, size))
	b := src.Bounds()
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			sx := b.Min.X + x*b.Dx()/size
			sy := b.Min.Y + y*b.Dy()/size
			dst.Set(x, y, src.At(sx, sy))
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: 85}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
