package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"

	"golang.org/x/image/draw"

	"image"
	"image/jpeg"
	_ "image/png"
)

const numWorkers = 10

var imageSize = image.Rect(0, 0, 32, 32)

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

func main() {
	n := flag.Int("n", 100, "number of images to download")
	subreddit := flag.String("subreddit", "pics", "subreddit to scrape")
	flag.Parse()

	var (
		wg   = new(sync.WaitGroup)
		jobs = make(chan job)
	)

	wg.Add(numWorkers)
	for i := 0; i < numWorkers; i++ {
		go imageFetcher(jobs, wg)
	}

	go loadPages(*subreddit, jobs, *n)

	wg.Wait()
}

func loadPages(sub string, jobs chan<- job, total int) {
	defer close(jobs)
	var (
		errs           = make(chan error, numWorkers)
		success        = make(chan int, numWorkers)
		posts          []Post
		submittedCount int
		errCount       int
		successCount   int
		after          string
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
				errCount++
				fmt.Println("Err:", err)
			case <-success:
				successCount++
				fmt.Printf("%d/%d\n", successCount, total)
				if successCount >= total {
					break outer
				}
			}
		}
	}
}

type job struct {
	index   int
	url     string
	err     chan error
	success chan int
}

func imageFetcher(work <-chan job, wg *sync.WaitGroup) {
	for job := range work {
		err := fetchImage(job.index, job.url)
		if err != nil {
			job.err <- err
		} else {
			job.success <- job.index
		}
	}
	wg.Done()
}

func fetchImage(i int, url string) error {
	if !strings.HasSuffix(url, ".jpg") {
		url += ".jpg"
	}
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if !strings.Contains(resp.Header.Get("Content-Type"), "image/") {
		return fmt.Errorf("the response was not an image: %s", resp.Header.Get("Content-Type"))
	}

	out, err := os.Create(fmt.Sprintf("images/%d.jpg", i))
	if err != nil {
		return err
	}
	defer out.Close()

	img, _, err := image.Decode(resp.Body)
	if err != nil {
		return err
	}

	output := resize(img)
	err = jpeg.Encode(out, output, &jpeg.Options{Quality: 90})
	if err != nil {
		return err
	}
	return nil
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

func resize(img image.Image) image.Image {
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
