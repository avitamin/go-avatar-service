package contract

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
)

func testJPEG() []byte {
	img := testImage()
	var buf bytes.Buffer
	_ = jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90})
	return buf.Bytes()
}

func testPNG() []byte {
	img := testImage()
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

func oversizedPayload() []byte {
	data := make([]byte, maxUploadBytes+1)
	copy(data, testJPEG())
	return data
}

func testImage() image.Image {
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			img.Set(x, y, color.RGBA{
				R: uint8(20 + x*20),
				G: uint8(30 + y*20),
				B: 180,
				A: 255,
			})
		}
	}
	return img
}
