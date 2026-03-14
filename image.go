package wallpaper

import (
	"bytes"
	"embed"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"sync"
)

//go:embed mask_raw.png
var embeddedAssets embed.FS

const (
	bottomCrop     = 79
	blackMargin    = 50
	colorMixFactor = 0.3
	saturationGain = 1.75
)

var (
	maskOnce sync.Once
	maskImg  *image.NRGBA
	maskErr  error
)

func ProcessSatelliteImage(source image.Image) (*image.NRGBA, error) {
	mask, err := loadMaskImage()
	if err != nil {
		return nil, err
	}
	return processSatelliteImageWithMask(source, mask, bottomCrop, blackMargin)
}

func processSatelliteImageWithMask(source image.Image, mask *image.NRGBA, cropPixels, marginPixels int) (*image.NRGBA, error) {
	src := toNRGBA(source)
	if src.Bounds().Dx() != mask.Bounds().Dx() || src.Bounds().Dy() != mask.Bounds().Dy() {
		return nil, fmt.Errorf("mask size %dx%d does not match image size %dx%d", mask.Bounds().Dx(), mask.Bounds().Dy(), src.Bounds().Dx(), src.Bounds().Dy())
	}
	height := src.Bounds().Dy() - cropPixels
	if height <= 0 {
		return nil, fmt.Errorf("source image too short: %d", src.Bounds().Dy())
	}
	width := src.Bounds().Dx()
	dst := image.NewNRGBA(image.Rect(0, 0, width+2*marginPixels, height+2*marginPixels))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			srcOffset := src.PixOffset(x, y)
			maskOffset := mask.PixOffset(x, y)
			maskStrength := (float64(mask.Pix[maskOffset]) + float64(mask.Pix[maskOffset+1]) + float64(mask.Pix[maskOffset+2])) / (3 * 255)

			r := float64(src.Pix[srcOffset]) / 255
			g := float64(src.Pix[srcOffset+1]) / 255
			b := float64(src.Pix[srcOffset+2]) / 255

			r *= 1 - maskStrength
			g *= 1 - maskStrength
			b *= 1 - maskStrength

			genuineNorm := r*r + g*g + b*b
			newR := (1-colorMixFactor)*r + colorMixFactor*g
			newG := (1-1.5*colorMixFactor)*g + colorMixFactor*r + 0.5*colorMixFactor*b
			newB := b
			norm := newR*newR + newG*newG + newB*newB
			if norm > 0 {
				scale := genuineNorm / norm
				newR *= scale
				newG *= scale
				newB *= scale
			} else {
				newR, newG, newB = 0, 0, 0
			}

			newR = saturateValue(newR, saturationGain)
			newG = saturateValue(newG, saturationGain)
			newB = saturateValue(newB, saturationGain)

			dstOffset := dst.PixOffset(x+marginPixels, y+marginPixels)
			dst.Pix[dstOffset] = floatToByte(newR)
			dst.Pix[dstOffset+1] = floatToByte(newG)
			dst.Pix[dstOffset+2] = floatToByte(newB)
			dst.Pix[dstOffset+3] = 0xff
		}
	}
	return dst, nil
}

func loadMaskImage() (*image.NRGBA, error) {
	maskOnce.Do(func() {
		data, err := embeddedAssets.ReadFile("mask_raw.png")
		if err != nil {
			maskErr = err
			return
		}
		img, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			maskErr = err
			return
		}
		maskImg = toNRGBA(img)
	})
	return maskImg, maskErr
}

func toNRGBA(src image.Image) *image.NRGBA {
	if img, ok := src.(*image.NRGBA); ok {
		return img
	}
	bounds := src.Bounds()
	dst := image.NewNRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	draw.Draw(dst, dst.Bounds(), src, bounds.Min, draw.Src)
	return dst
}

func saturateValue(x, factor float64) float64 {
	result := math.Tanh((x-0.5)*factor)/math.Tanh(0.5*factor)/2 + 0.5
	if result < 0 {
		return 0
	}
	if result > 1 {
		return 1
	}
	return result
}

func floatToByte(value float64) uint8 {
	if value <= 0 {
		return 0
	}
	if value >= 1 {
		return 255
	}
	return uint8(math.Round(value * 255))
}

func renderFittedWallpaper(sourcePath, destinationPath string, width, height, offsetY int) error {
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	sourceImage, _, err := image.Decode(sourceFile)
	if err != nil {
		return err
	}
	src := toNRGBA(sourceImage)
	resized := resizeToFit(src, width, height)
	canvas := image.NewNRGBA(image.Rect(0, 0, width, height))
	draw.Draw(canvas, canvas.Bounds(), &image.Uniform{C: color.Black}, image.Point{}, draw.Src)
	offsetX := (width - resized.Bounds().Dx()) / 2
	y := (height-resized.Bounds().Dy())/2 + offsetY
	draw.Draw(canvas, image.Rect(offsetX, y, offsetX+resized.Bounds().Dx(), y+resized.Bounds().Dy()), resized, image.Point{}, draw.Over)
	return writePNGAtomic(destinationPath, canvas)
}

func resizeToFit(src *image.NRGBA, maxWidth, maxHeight int) *image.NRGBA {
	if maxWidth <= 0 || maxHeight <= 0 {
		return src
	}
	srcW := src.Bounds().Dx()
	srcH := src.Bounds().Dy()
	if srcW == 0 || srcH == 0 {
		return src
	}
	scale := math.Min(float64(maxWidth)/float64(srcW), float64(maxHeight)/float64(srcH))
	if scale <= 0 {
		return src
	}
	targetW := int(math.Round(float64(srcW) * scale))
	targetH := int(math.Round(float64(srcH) * scale))
	if targetW < 1 {
		targetW = 1
	}
	if targetH < 1 {
		targetH = 1
	}
	return resizeBilinear(src, targetW, targetH)
}

func resizeBilinear(src *image.NRGBA, targetW, targetH int) *image.NRGBA {
	dst := image.NewNRGBA(image.Rect(0, 0, targetW, targetH))
	srcW := src.Bounds().Dx()
	srcH := src.Bounds().Dy()
	if targetW == srcW && targetH == srcH {
		draw.Draw(dst, dst.Bounds(), src, image.Point{}, draw.Src)
		return dst
	}
	xScale := float64(srcW) / float64(targetW)
	yScale := float64(srcH) / float64(targetH)
	for y := 0; y < targetH; y++ {
		srcY := (float64(y)+0.5)*yScale - 0.5
		y0 := clampInt(int(math.Floor(srcY)), 0, srcH-1)
		y1 := clampInt(y0+1, 0, srcH-1)
		wy := srcY - math.Floor(srcY)
		for x := 0; x < targetW; x++ {
			srcX := (float64(x)+0.5)*xScale - 0.5
			x0 := clampInt(int(math.Floor(srcX)), 0, srcW-1)
			x1 := clampInt(x0+1, 0, srcW-1)
			wx := srcX - math.Floor(srcX)

			c00 := src.NRGBAAt(x0, y0)
			c10 := src.NRGBAAt(x1, y0)
			c01 := src.NRGBAAt(x0, y1)
			c11 := src.NRGBAAt(x1, y1)

			dst.SetNRGBA(x, y, color.NRGBA{
				R: interpolateChannel(c00.R, c10.R, c01.R, c11.R, wx, wy),
				G: interpolateChannel(c00.G, c10.G, c01.G, c11.G, wx, wy),
				B: interpolateChannel(c00.B, c10.B, c01.B, c11.B, wx, wy),
				A: interpolateChannel(c00.A, c10.A, c01.A, c11.A, wx, wy),
			})
		}
	}
	return dst
}

func interpolateChannel(c00, c10, c01, c11 uint8, wx, wy float64) uint8 {
	top := float64(c00)*(1-wx) + float64(c10)*wx
	bottom := float64(c01)*(1-wx) + float64(c11)*wx
	return uint8(math.Round(top*(1-wy) + bottom*wy))
}

func clampInt(value, minValue, maxValue int) int {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func writePNGAtomic(path string, img image.Image) error {
	if err := ensureDir(filepath.Dir(path)); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*.png")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := png.Encode(tmp, img); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}
