package main

import (
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	_ "image/png"
	"log"
	"os"
	"path/filepath"
)

const tileSize = 16

func main() {
	img, err := loadImage("trump.jpg")
	if err != nil {
		log.Fatal(err)
	}

	after, err := process(img)
	if err != nil {
		log.Fatal(err)
	}

	out, err := os.Create("output.jpg")
	if err != nil {
		log.Fatal(err)
	}
	err = jpeg.Encode(out, after, &jpeg.Options{Quality: 90})
	if err != nil {
		log.Fatal(err)
	}

}

func loadImage(file string) (image.Image, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("%s: %v", file, err)
	}
	return img, nil
}

type SubImage struct {
	rect image.Rectangle
	image.Image
}

func (s SubImage) Bounds() image.Rectangle {
	return s.rect
}

func process(in image.Image) (image.Image, error) {
	tiler, err := NewTiler("downloader/images")
	if err != nil {
		return nil, err
	}
	return mosaic(tiler.imageMatch, in), nil
}

func roundTo(x, div int) int {
	return (x / div) * div
}

func mosaic(strategy func(image.Image) image.Image, in image.Image) image.Image {
	bounds := in.Bounds().Canon()

	width := roundTo(bounds.Max.X-bounds.Min.X, tileSize) + bounds.Min.X
	height := roundTo(bounds.Max.Y-bounds.Min.Y, tileSize) + bounds.Min.Y

	output := image.NewRGBA(image.Rect(bounds.Min.X, bounds.Min.Y, width, height))

	for i := bounds.Min.X; i < width; i += tileSize {
		for j := bounds.Min.Y; j < height; j += tileSize {
			color := strategy(SubImage{
				rect:  image.Rect(i, j, i+tileSize, j+tileSize),
				Image: in,
			})

			for x := 0; x < tileSize; x++ {
				for y := 0; y < tileSize; y++ {
					output.Set(i+x, j+y, color.At(x, y))
				}
			}
		}
	}

	return output
}

type Tiler struct {
	images []Tile
}

type Tile struct {
	average color.Color
	image   image.Image
}

func NewTiler(dir string) (*Tiler, error) {
	var tiler = new(Tiler)
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		fmt.Println("loading", path)
		if info.IsDir() {
			fmt.Println("skipping dir")
			return nil
		}
		img, err := loadImage(path)
		if err != nil {
			return err
		}
		tiler.images = append(tiler.images, Tile{
			image:   img,
			average: averageColor(img),
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return tiler, nil
}

func (t *Tiler) match(in image.Image) image.Image {
	average := averageColor(in)

	if len(t.images) == 0 {
		panic("No images loaded into tiler")
	}

	var (
		bestFit     = t.images[0]
		minDistance = colorDistance(average, bestFit.average)
	)
	for _, tile := range t.images[1:] {
		d := colorDistance(tile.average, average)
		if d < minDistance {
			bestFit = tile
			minDistance = d
		}
	}
	return bestFit.image
}

func (t *Tiler) imageMatch(in image.Image) image.Image {
	if len(t.images) == 0 {
		panic("No images loaded into tiler")
	}

	var (
		bestFit     = t.images[0]
		minDistance = imageDistance(in, bestFit.image)
	)
	for _, tile := range t.images[1:] {
		d := imageDistance(in, tile.image)
		if d < minDistance {
			bestFit = tile
			minDistance = d
		}
	}
	return bestFit.image
}

func imageDistance(x, y image.Image) uint32 {
	diff := image.NewRGBA(x.Bounds())
	bounds := x.Bounds()

	numPixels := uint32(bounds.Dx() * bounds.Dy())

	// Compute diff
	for i := bounds.Min.X; i < bounds.Max.X; i++ {
		for j := bounds.Min.Y; j < bounds.Max.Y; j++ {
			xC := x.At(i, j)
			yC := y.At(i, j)

			diff.Set(i, j, colorDiff(xC, yC))
		}
	}

	averageDiff := averageColor(diff)
	ar, ag, ab, aa := averageDiff.RGBA()

	variance := RGBAColor{}

	// Compute variance
	for i := bounds.Min.X; i < bounds.Max.X; i++ {
		for j := bounds.Min.Y; j < bounds.Max.Y; j++ {
			r, g, b, a := diff.At(i, j).RGBA()
			variance.r += (r - ar) * (r - ar)
			variance.g += (g - ag) * (g - ag)
			variance.b += (b - ab) * (b - ab)
			variance.a += (a - aa) * (a - aa)
		}
	}
	variance.r /= numPixels
	variance.g /= numPixels
	variance.b /= numPixels
	variance.a /= numPixels

	return colorDistance(variance, color.Black)
}

func colorDiff(x, y color.Color) color.Color {
	xr, xg, xb, xa := x.RGBA()
	yr, yg, yb, ya := y.RGBA()

	return RGBAColor{
		r: absDiff(xr, yr),
		g: absDiff(xg, yg),
		b: absDiff(xb, yb),
		a: absDiff(xa, ya),
	}
}

func absDiff(a, b uint32) uint32 {
	if a < b {
		return b - a
	}
	return a - b
}

func colorDistance(x, y color.Color) uint32 {
	r1, g1, b1, a1 := x.RGBA()
	r2, g2, b2, a2 := y.RGBA()

	dr := absDiff(r1, r2)
	dg := absDiff(g1, g2)
	db := absDiff(b1, b2)
	da := absDiff(a1, a2)

	return dr*dr + dg*dg + db*db + da*da
}

type RGBAColor struct {
	r, g, b, a uint32
}

func (r RGBAColor) RGBA() (uint32, uint32, uint32, uint32) {
	return r.r, r.g, r.b, r.a
}

func averageColorImage(in image.Image) image.Image {
	return image.NewUniform(averageColor(in))
}

func averageColor(in image.Image) color.Color {
	bounds := in.Bounds().Canon()

	numPixels := uint32((bounds.Max.X - bounds.Min.X) * (bounds.Max.Y - bounds.Min.Y))
	var rSum, gSum, bSum, aSum uint32
	for i := bounds.Min.X; i < bounds.Max.X; i++ {
		for j := bounds.Min.Y; j < bounds.Max.Y; j++ {
			color := in.At(i, j)
			r, g, b, a := color.RGBA()
			rSum += r
			gSum += g
			bSum += b
			aSum += a
		}
	}

	return RGBAColor{
		r: rSum / numPixels,
		g: gSum / numPixels,
		b: bSum / numPixels,
		a: aSum / numPixels,
	}
}
