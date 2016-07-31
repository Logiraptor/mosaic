package main

import (
	"encoding/json"
	"image"
	_ "image/jpeg"
	"image/png"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/gorilla/schema"

	_ "expvar"

	"fmt"
)

type Credentials struct {
	Host     string
	Password string
	Port     int
}

type Service struct {
	Credentials Credentials
}

type VCapServices struct {
	Redis []Service `json:"p-redis"`
}

func readVcap(vcapServices string) (VCapServices, error) {
	var vcap VCapServices
	err := json.NewDecoder(strings.NewReader(vcapServices)).Decode(&vcap)
	return vcap, err
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	cache := NewRedisCache(webImageLoader{})

	http.Handle("/generate", &MosaicGenerator{
		ImageLoader: cache,
	})
	http.Handle("/cached", cache)
	http.Handle("/", http.FileServer(http.Dir("public")))
	log.Println(http.ListenAndServe(":"+port, nil))
}

type ImageConfig struct {
	SampleSize           int
	NumSamples, TileSize int
	TileSourceSubreddit  string
	InputImageURL        string
}

type MosaicGenerator struct {
	ImageLoader
}

func (m *MosaicGenerator) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(rw, "panic: %v", r)
		}
	}()
	var config ImageConfig
	req.ParseForm()
	err := schema.NewDecoder().Decode(&config, req.Form)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusBadRequest)
		return
	}

	img, err := m.LoadImage(config.InputImageURL)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusBadRequest)
		return
	}

	config.TileSize = 25

	idealTileWidth := float32(config.TileSize * 150)
	maxDim := float32(max(img.Bounds().Dx(), img.Bounds().Dy()))

	config.SampleSize = int((maxDim / idealTileWidth) * float32(config.TileSize))

	after, err := m.process(config, img)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusBadRequest)
		return
	}

	err = png.Encode(rw, after)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusBadRequest)
		return
	}
}

func max(x, y int) int {
	if x > y {
		return x
	}
	return y
}

type SubImage struct {
	rect image.Rectangle
	image.Image
}

func (s SubImage) Bounds() image.Rectangle {
	return s.rect
}

func (m *MosaicGenerator) process(c ImageConfig, in image.Image) (image.Image, error) {
	images, err := DownloadImages(m.ImageLoader, c.TileSourceSubreddit, c.NumSamples, c.TileSize)
	if err != nil {
		return nil, err
	}

	tiler, err := NewTiler(images)
	if err != nil {
		return nil, err
	}
	return c.mosaic(tiler.match, in), nil
}

func (c *ImageConfig) mosaic(strategy func(image.Image) image.Image, in image.Image) image.Image {

	in = cropToMultiple(in, c.SampleSize)
	bounds := in.Bounds().Canon()

	numTilesX := bounds.Dx() / c.SampleSize
	numTilesY := bounds.Dy() / c.SampleSize

	output := image.NewRGBA(image.Rect(0, 0, numTilesX*c.TileSize, numTilesY*c.TileSize))

	parallelMap(numTilesX, func(i int) {
		parallelMap(numTilesY, func(j int) {

			result := strategy(SubImage{
				rect: image.Rect(
					i*c.SampleSize, j*c.SampleSize,
					i*c.SampleSize+c.SampleSize, j*c.SampleSize+c.SampleSize,
				),
				Image: in,
			})

			for x := 0; x < c.TileSize; x++ {
				for y := 0; y < c.TileSize; y++ {

					pX := i*c.TileSize + x
					pY := j*c.TileSize + y

					output.Set(pX, pY, result.At(x, y))
				}
			}
		})
	})

	return output
}

func cropToMultiple(img image.Image, tileSize int) image.Image {
	min := img.Bounds().Min
	width := roundTo(img.Bounds().Dx(), tileSize)
	height := roundTo(img.Bounds().Dy(), tileSize)
	return crop(img, image.Rect(min.X, min.Y, width+min.X, height+min.Y))
}

func roundTo(x, div int) int {
	return (x / div) * div
}

func crop(img image.Image, rect image.Rectangle) image.Image {
	return SubImage{
		Image: img,
		rect:  rect,
	}
}
