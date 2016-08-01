package main

import (
	"image"
	"image/color"
)

type Mosaic struct {
	tileSize int
	images   [][]image.Image
}

func NewMosaic(width, height, tileSize int) Mosaic {
	images := make([][]image.Image, width)
	for i := 0; i < width; i++ {
		images[i] = make([]image.Image, height)
	}
	return Mosaic{
		tileSize: tileSize,
		images:   images,
	}
}

var _ image.Image = Mosaic{}

func (m Mosaic) At(x, y int) color.Color {
	tx := x / m.tileSize
	ty := y / m.tileSize
	lx := x % m.tileSize
	ly := y % m.tileSize

	return m.images[tx][ty].At(lx, ly)
}

func (m Mosaic) Bounds() image.Rectangle {
	return image.Rect(0, 0, m.tileSize*len(m.images), m.tileSize*len(m.images[0]))
}

func (m Mosaic) ColorModel() color.Model {
	return m.images[0][0].ColorModel()
}
