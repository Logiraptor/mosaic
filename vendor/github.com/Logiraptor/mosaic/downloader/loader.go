package downloader

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"

	"golang.org/x/image/draw"

	"image"
	_ "image/png"
)

const numWorkers = 10

type SubReddit struct {
	Data struct {
		Children []Post
		After    string
	}
}

type Post struct {
	Data struct {
		Url string
	}
}

func DownloadImages(subreddit string, n, size int) ([]image.Image, error) {
	var imageSize = image.Rect(0, 0, size, size)

	var (
		wg   = new(sync.WaitGroup)
		jobs = make(chan job)
	)

	wg.Add(numWorkers)
	for i := 0; i < numWorkers; i++ {
		go imageFetcher(jobs, wg, imageSize)
	}

	images := loadPages(subreddit, jobs, n)

	wg.Wait()

	return images, nil
}

func loadPages(sub string, jobs chan<- job, total int) []image.Image {
	defer close(jobs)
	var (
		errs           = make(chan error, numWorkers)
		success        = make(chan image.Image, numWorkers)
		posts          []Post
		submittedCount int
		after          string

		images []image.Image
	)

outer:
	for {
		posts, after = loadSubredditPage(sub, after)
		for _, p := range posts {
			j := job{
				index:   submittedCount,
				url:     p.Data.Url,
				err:     errs,
				success: success,
			}
			select {
			case jobs <- j:
				submittedCount++
			case err := <-errs:
				fmt.Println("Err:", err)
			case img := <-success:
				images = append(images, img)
				fmt.Printf("%d/%d\n", len(images), total)
				if len(images) >= total {
					break outer
				}
			}
		}
	}
	return images
}

type job struct {
	index   int
	url     string
	err     chan error
	success chan image.Image
}

func imageFetcher(work <-chan job, wg *sync.WaitGroup, imageSize image.Rectangle) {
	for job := range work {
		image, err := fetchImage(job.url)
		if err != nil {
			job.err <- err
		} else {
			job.success <- resize(image, imageSize)
		}
	}
	wg.Done()
}

func fetchImage(url string) (image.Image, error) {
	if !strings.HasSuffix(url, ".jpg") {
		url += ".jpg"
	}
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if !strings.Contains(resp.Header.Get("Content-Type"), "image/") {
		return nil, fmt.Errorf("the response was not an image: %s", resp.Header.Get("Content-Type"))
	}

	img, _, err := image.Decode(resp.Body)
	if err != nil {
		return nil, err
	}

	return img, nil
}

func loadSubredditPage(subreddit string, after string) ([]Post, string) {
	req, err := http.NewRequest("GET", "https://www.reddit.com/r/"+subreddit+".json?after="+after, nil)
	if err != nil {
		log.Fatal(err)
	}

	req.Header.Set("User-Agent", "linux:mosaic-photos:0.0.1 (by /u/Logiraptorr)")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	var sub SubReddit
	err = json.NewDecoder(resp.Body).Decode(&sub)
	if err != nil {
		log.Fatal(err)
	}

	return sub.Data.Children, sub.Data.After
}

func resize(img image.Image, imageSize image.Rectangle) image.Image {
	output := image.NewRGBA(imageSize)
	draw.BiLinear.Scale(output, imageSize, img, maxSquareInRect(img.Bounds()), draw.Over, nil)
	return output
}

func maxSquareInRect(source image.Rectangle) image.Rectangle {
	size := source.Size()
	var (
		min   int
		diffY int
		diffX int
	)
	if size.Y < size.X {
		min = size.Y
		diffX = (size.X - min) / 2
		diffY = (size.Y - min) / 2
	} else {
		min = size.X
		diffY = (size.Y - min) / 2
		diffX = (size.X - min) / 2
	}
	return image.Rect(0, 0, min, min).Add(image.Point{X: diffX, Y: diffY})
}
