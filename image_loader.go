package main

import (
	"fmt"
	"image"
	"net/http"
)

type ImageLoader interface {
	LoadImage(url string) (image.Image, error)
}

type webImageLoader struct{}

func (webImageLoader) LoadImage(file string) (image.Image, error) {
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
