package main

import (
	"image"
	"image/color"

	"github.com/Logiraptor/mosaic/downloader"
)

type Tiler struct {
	images []Tile
}

type Tile struct {
	average color.Color
	image   image.Image
}

func NewTiler(c imageConfig) (*Tiler, error) {
	var tiler = new(Tiler)
	images, err := downloader.DownloadImages(c.TileSourceSubreddit, c.NumSamples, c.TileSize)
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
