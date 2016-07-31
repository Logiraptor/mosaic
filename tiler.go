package main

import (
	"image"
	"image/color"
)

type Tiler struct {
	images []Tile
}

type Tile struct {
	average color.Color
	image   image.Image
}

func NewTiler(images []image.Image) (*Tiler, error) {
	var tiler = new(Tiler)
	for _, img := range images {
		tiler.images = append(tiler.images, Tile{
			image:   img,
			average: averageColor(img),
		})
	}
	return tiler, nil
}

func (t *Tiler) match(in color.Color) image.Image {
	if len(t.images) == 0 {
		panic("No images loaded into tiler")
	}

	var (
		bestFit     = t.images[0]
		minDistance = colorDistance(in, bestFit.average)
	)
	for _, tile := range t.images[1:] {
		d := colorDistance(tile.average, in)
		if d < minDistance {
			bestFit = tile
			minDistance = d
		}
	}
	return bestFit.image
}
