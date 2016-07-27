package main

import (
	"fmt"
	"image"
	_ "image/jpeg"
	"image/png"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/schema"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}
	http.HandleFunc("/generate", generateHandler)
	http.Handle("/", http.FileServer(http.Dir("public")))
	http.ListenAndServe(":"+port, nil)
}

type imageConfig struct {
	NumSamples, TileSize int
	TileSourceSubreddit  string
	InputImageURL        string
}

func generateHandler(rw http.ResponseWriter, req *http.Request) {
	var config imageConfig
	req.ParseForm()
	err := schema.NewDecoder().Decode(&config, req.Form)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusBadRequest)
		return
	}

	img, err := loadImage(config.InputImageURL)
	if err != nil {
		log.Fatal(err)
	}

	after, err := config.process(img)
	if err != nil {
		log.Fatal(err)
	}

	err = png.Encode(rw, after)
	if err != nil {
		log.Fatal(err)
	}
}

func loadImage(file string) (image.Image, error) {
	resp, err := http.Get(file)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	img, _, err := image.Decode(resp.Body)
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

func (c *imageConfig) process(in image.Image) (image.Image, error) {
	tiler, err := NewTiler(*c)
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
