package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"gopkg.in/go-redis/cache.v3/lrucache"

	"image"
)

type redisCache struct {
	codec    *lrucache.Cache
	fallback ImageLoader
}

func NewRedisCache(fallback ImageLoader) *redisCache {
	return &redisCache{
		codec:    lrucache.New(time.Hour, 1000),
		fallback: fallback,
	}
}

func (r *redisCache) LoadImage(url string) (image.Image, error) {
	img, ok := r.codec.Get(url)
	if !ok {
		log.Println("Cache miss:", url)
		img, err := r.fallback.LoadImage(url)
		if err != nil {
			return nil, err
		}
		r.codec.Set(url, img)
		return img, nil
	}
	return img.(image.Image), nil
}

func (r *redisCache) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	fmt.Fprintf(rw, "%d/%d Cached", r.codec.Len(), 1000)
}
