package main

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"sync"

	"golang.org/x/net/html"
)

type Fetcher interface {
	Fetch(url string) (body string, urls []string, err error)
}

func Crawl(url string, depth int, wg *sync.WaitGroup, fetcher Fetcher) {
	defer wg.Done()
	if depth <= 0 {
		return
	}
	body, urls, err := fetcher.Fetch(url)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Printf("found: %s %q\n", url, body)
	for _, u := range urls {
		wg.Add(1)
		go Crawl(u, depth-1, wg, fetcher)
		// Crawl(u, depth-1, wg, fetcher)
	}
	return
}

func main() {
	var wg sync.WaitGroup
	wg.Add(1)
	go Crawl("https://www.omise.co", 2, &wg, NewWebFetcher())
	wg.Wait()
	// Crawl("https://www.omise.co", 2, &wg, NewWebFetcher())
}

type WebFetcher struct {
	urls map[string]struct{}
	mu   sync.Mutex
}

func NewWebFetcher() *WebFetcher {
	return &WebFetcher{
		urls: make(map[string]struct{}),
	}
}

func (t *WebFetcher) Track(url string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	_, ok := t.urls[url]
	if !ok {
		t.urls[url] = struct{}{}
		return true
	}
	return false
}

func (t *WebFetcher) Fetch(url string) (string, []string, error) {
	t.Track(url)
	resp, err := http.Get(url)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", nil, nil
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", nil, err
	}
	urls := t.parseLink(bytes.NewReader(body))
	return string(body), urls, nil
}

func (t *WebFetcher) parseLink(r io.Reader) []string {
	z := html.NewTokenizer(r)
	var urls []string

	for {
		tt := z.Next()
		if tt == html.ErrorToken {
			return urls
		}

		if tt == html.StartTagToken {
			if tagName, hasAttr := z.TagName(); string(tagName) == "a" && hasAttr {
				for {
					attrName, attrValue, moreAttr := z.TagAttr()
					if string(attrName) == "href" {
						u, _ := url.Parse(string(attrValue))
						if u.Host != "" && t.Track(string(attrValue)) {
							urls = append(urls, string(attrValue))
						}
					}
					if !moreAttr {
						break
					}
				}
			}
		}
	}
	return urls
}
