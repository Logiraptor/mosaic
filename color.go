package main

import (
	"image"
	"image/color"
)

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
