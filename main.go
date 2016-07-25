package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	"image/png"
	"log"
	"os"
	"sync"

	"github.com/Logiraptor/mosaic/downloader"
)

var (
	tileSize *int
	samples  *int
	sub      *string
)

func main() {
	samples = flag.Int("samples", 10, "number of images to use in mosaic")
	sub = flag.String("subreddit", "pics", "which subreddit to pull images from")
	tileSize = flag.Int("tileSize", 16, "size of mosaic tiles")
	input := flag.String("input", "trump.jpg", "input file")
	flag.Parse()

	img, err := loadImage(*input)
	if err != nil {
		log.Fatal(err)
	}

	after, err := process(img)
	if err != nil {
		log.Fatal(err)
	}

	out, err := os.Create(fmt.Sprintf("%s-%s.png", *sub, *input))
	if err != nil {
		log.Fatal(err)
	}
	err = png.Encode(out, after)
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
	tiler, err := NewTiler(*sub)
	if err != nil {
		return nil, err
	}
	return mosaic(tiler.match, in), nil
}

func roundTo(x, div int) int {
	return (x / div) * div
}

func mosaic(strategy func(image.Image) image.Image, in image.Image) image.Image {
	bounds := in.Bounds().Canon()

	width := roundTo(bounds.Max.X-bounds.Min.X, *tileSize) + bounds.Min.X
	height := roundTo(bounds.Max.Y-bounds.Min.Y, *tileSize) + bounds.Min.Y

	output := image.NewRGBA(image.Rect(bounds.Min.X, bounds.Min.Y, width, height))

	parallelMap(width / *tileSize, step(bounds.Min.X, *tileSize, func(i int) {
		parallelMap(height / *tileSize, step(bounds.Min.Y, *tileSize, func(j int) {
			color := strategy(SubImage{
				rect:  image.Rect(i, j, i+*tileSize, j+*tileSize),
				Image: in,
			})
			for x := 0; x < *tileSize; x++ {
				for y := 0; y < *tileSize; y++ {
					output.Set(i+x, j+y, color.At(x, y))
				}
			}
		}))
	}))

	return output
}

type Tiler struct {
	images []Tile
}

type Tile struct {
	average color.Color
	image   image.Image
}

func NewTiler(sub string) (*Tiler, error) {
	var tiler = new(Tiler)
	images, err := downloader.DownloadImages(sub, *samples, *tileSize)
	if err != nil {
		return nil, err
	}
	for _, img := range images {
		tiler.images = append(tiler.images, Tile{
			image:   img,
			average: averageColor(img),
		})
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
	bounds := x.Bounds()
	numPixels := uint32(bounds.Dx() * bounds.Dy())

	diff := imageDiff(x, y)

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

func combine(x, y image.Image, f func(x, y int, a, b color.Color)) {
	xBounds := x.Bounds()
	yBounds := y.Bounds()

	dx := xBounds.Dx()
	dy := xBounds.Dy()

	for i := 0; i < dx; i++ {
		for j := 0; j < dy; j++ {
			cX := x.At(i+xBounds.Min.X, j+xBounds.Min.Y)
			cY := y.At(i+yBounds.Min.X, j+yBounds.Min.Y)

			f(i, j, cX, cY)
		}
	}
}

func imageDiff(x, y image.Image) image.Image {
	diff := image.NewRGBA(x.Bounds().Sub(x.Bounds().Min))

	// Compute diff
	combine(x, y, func(x, y int, a, b color.Color) {
		diff.Set(x, y, colorDiff(a, b))
	})

	return diff
}

func colorDiff(x, y color.Color) color.Color {
	xr, xg, xb, _ := x.RGBA()
	yr, yg, yb, _ := y.RGBA()

	xy, xu, xv := color.RGBToYCbCr(uint8(xr>>8), uint8(xg>>8), uint8(xb>>8))
	yy, yu, yv := color.RGBToYCbCr(uint8(yr>>8), uint8(yg>>8), uint8(yb>>8))

	return RGBAColor{
		r: absDiff(uint32(xy), uint32(yy)),
		g: absDiff(uint32(xu), uint32(yu)),
		b: absDiff(uint32(xv), uint32(yv)),
		a: 1,
	}
}

func absDiff(a, b uint32) uint32 {
	if a < b {
		return b - a
	}
	return a - b
}

func colorDistance(x, y color.Color) uint32 {
	d := colorDiff(x, y).(RGBAColor)
	return d.r*d.r + d.g*d.g + d.b*d.b + d.a*d.a
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

func step(start, step int, inner func(int)) func(int) {
	return func(i int) {
		inner(start + i*step)
	}
}

func parallelMap(n int, f func(i int)) {
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			f(i)
			wg.Done()
		}(i)
	}

	wg.Wait()
}
