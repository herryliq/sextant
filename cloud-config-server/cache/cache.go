package cache

import (
	"io/ioutil"
	"log"
	"net/http"
	"time"
)

// Cache manages a local in-memory copy of a remote file as well as a
// local on-disk copy.  It periodically read the remote file (which
// might change occasionally), update the local in-memory and on-disk
// copy.
//
// Example:
/*
func main() {
  c := cache.New(url, filename)
  http.Handle("/", handler)
}

func handler(...) {
  c.Get() // returns cache content and triggers update. No waiting.
}
*/
type Cache struct {
	filename string
	url      string
	content  []byte // Don't write into content.

	update chan int // Writing into this channel tiggers an update.
	close  chan int // Writing into this channel closes the cache.
}

const (
	loadTimeout  = 15 * time.Second
	updatePeriod = 20 * time.Second
)

// New panics if it fails to read remote nor local file; othersie it
// returns a ready-to-read in-memory cache.  To close the cache and
// free all resources, write into channel Cache.close.
func New(url, filename string) *Cache {
	c := &Cache{
		filename: filename,
		url:      url,
		content:  load(url, filename),
		update:   make(chan int, 1),
		close:    make(chan int),
	}

	go func() {
		tic := time.Tick(updatePeriod)
		for {
			select {
			case <-tic:
			case <-c.update:
			}

			if b, e := httpGet(c.url, loadTimeout); e == nil {
				c.content = b
				if e := ioutil.WriteFile(c.filename, b, 0644); e != nil {
					log.Printf("Cannot write to local file %s: %v", c.filename, e)
				}
			}

			select {
			case <-c.close:
				close(c.update)
				close(c.close)
				return
			default:
			}
		}
	}()

	return c
}

// local panics if cannot read remote nor local file.
func load(url, fn string) []byte {
	b, e := httpGet(url, loadTimeout)
	if e != nil {
		log.Printf("Cannot load from %s: %v. Try load from local file.", url, e)
		if b, e = ioutil.ReadFile(fn); e != nil {
			log.Panicf("Cannot load from local file %s either: %v", fn, e)
		}
	}
	return b
}

func httpGet(url string, timeout time.Duration) ([]byte, error) {
	client := http.Client{
		Timeout: timeout,
	}
	resp, err := client.Get(url)
	if err != nil || resp.StatusCode != 200 {
		log.Printf("%v", err)
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("%v", err)
		return nil, err
	}
	return body, nil
}

func (c *Cache) Get() []byte {
	b := c.content
	select {
	case c.update <- 1:
	default:
	}
	return b
}

func (c *Cache) Close() {
	c.close <- 1
}
