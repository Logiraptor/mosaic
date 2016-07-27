package main

import (
	"bytes"
	"log"

	"gopkg.in/go-redis/cache.v3"
	"gopkg.in/redis.v3"

	"image"
	"image/png"
	"strconv"
)

type redisCache struct {
	codec    *cache.Codec
	fallback ImageLoader
}

func NewRedisCache(creds Credentials, fallback ImageLoader) *redisCache {
	ring := redis.NewClient(&redis.Options{
		Addr:     creds.Host + ":" + strconv.Itoa(creds.Port),
		Password: creds.Password,
	})

	codec := &cache.Codec{
		Redis: ring,

		Marshal: func(v interface{}) ([]byte, error) {
			var buf bytes.Buffer
			err := png.Encode(&buf, v.(image.Image))
			if err != nil {
				return nil, err
			}
			return buf.Bytes(), nil
		},
		Unmarshal: func(b []byte, v interface{}) error {
			img, _, err := image.Decode(bytes.NewBuffer(b))
			if err != nil {
				return err
			}
			*(v.(*image.Image)) = img
			return nil
		},
	}

	return &redisCache{
		codec:    codec,
		fallback: fallback,
	}
}

func (r *redisCache) LoadImage(url string) (image.Image, error) {
	var image image.Image
	err := r.codec.Get(url, &image)
	if err != nil {
		log.Println("Cache error:", err.Error())
		img, err := r.fallback.LoadImage(url)
		if err != nil {
			return nil, err
		}
		r.codec.Set(&cache.Item{
			Key:    url,
			Object: img,
		})
	}
	return image, nil
}
