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
	http.ListenAndServe(":"+port, nil)
}

type imageConfig struct {
	NumSamples, TileSize int
	TileSourceSubreddit  string
	InputImageURL        string
}

type MosaicGenerator struct {
	ImageLoader
}

func (m *MosaicGenerator) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	var config imageConfig
	req.ParseForm()
	err := schema.NewDecoder().Decode(&config, req.Form)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusBadRequest)
		return
	}

	img, err := m.LoadImage(config.InputImageURL)
	if err != nil {
		log.Fatal(err)
	}

	after, err := m.process(config, img)
	if err != nil {
		log.Fatal(err)
	}

	err = png.Encode(rw, after)
	if err != nil {
		log.Fatal(err)
	}
}

type SubImage struct {
	rect image.Rectangle
	image.Image
}

func (s SubImage) Bounds() image.Rectangle {
	return s.rect
}

func (m *MosaicGenerator) process(c imageConfig, in image.Image) (image.Image, error) {
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

func roundTo(x, div int) int {
	return (x / div) * div
}

func (c *imageConfig) mosaic(strategy func(image.Image) image.Image, in image.Image) image.Image {
	bounds := in.Bounds().Canon()

	width := roundTo(bounds.Max.X-bounds.Min.X, c.TileSize) + bounds.Min.X
	height := roundTo(bounds.Max.Y-bounds.Min.Y, c.TileSize) + bounds.Min.Y

	output := image.NewRGBA(image.Rect(bounds.Min.X, bounds.Min.Y, width, height))

	parallelMap(width/c.TileSize, step(bounds.Min.X, c.TileSize, func(i int) {
		parallelMap(height/c.TileSize, step(bounds.Min.Y, c.TileSize, func(j int) {
			color := strategy(SubImage{
				rect:  image.Rect(i, j, i+c.TileSize, j+c.TileSize),
				Image: in,
			})
			for x := 0; x < c.TileSize; x++ {
				for y := 0; y < c.TileSize; y++ {
					output.Set(i+x, j+y, color.At(x, y))
				}
			}
		}))
	}))

	return output
}
