package wallpaper

import (
	"image"
	"image/color"
	"testing"
)

func TestProcessSatelliteImageWithMask(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	mask := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			src.SetNRGBA(x, y, color.NRGBA{R: 200, G: 150, B: 100, A: 255})
			mask.SetNRGBA(x, y, color.NRGBA{R: 0, G: 0, B: 0, A: 255})
		}
	}
	mask.SetNRGBA(1, 1, color.NRGBA{R: 255, G: 255, B: 255, A: 255})

	img, err := processSatelliteImageWithMask(src, mask, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Dx() != 6 || img.Bounds().Dy() != 5 {
		t.Fatalf("bounds = %v, want 6x5", img.Bounds())
	}
	if img.NRGBAAt(2, 2).R != 0 {
		t.Fatalf("masked pixel should be dark, got %+v", img.NRGBAAt(2, 2))
	}
}
