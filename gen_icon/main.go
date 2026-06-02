package main

import (
	"image"
	"image/color"
	"image/png"
	"os"
)

func main() {
	const size = 32
	img := image.NewNRGBA(image.Rect(0, 0, size, size))

	teal  := color.NRGBA{R: 14, G: 165, B: 165, A: 255}
	white := color.NRGBA{R: 255, G: 255, B: 255, A: 255}

	// Solid teal background — no transparency
	fill(img, 0, 0, size, size, teal)

	// Left ring (outline rectangle)
	fill(img, 3, 10, 14, 22, white)
	fill(img, 5, 12, 12, 20, teal) // hollow centre

	// Right ring
	fill(img, 18, 10, 29, 22, white)
	fill(img, 20, 12, 27, 20, teal)

	// Connecting bar
	fill(img, 11, 14, 21, 18, white)

	f, _ := os.Create("icon.png")
	defer f.Close()
	png.Encode(f, img)
}

func fill(img *image.NRGBA, x1, y1, x2, y2 int, c color.NRGBA) {
	for y := y1; y < y2; y++ {
		for x := x1; x < x2; x++ {
			img.SetNRGBA(x, y, c)
		}
	}
}
