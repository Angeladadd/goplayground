package crawler

import (
	"fmt"
	"sync"
)

// Fetcher is the interface that provides the Fetch method.
// We'll use a fake one for testing.
type Fetcher interface {
	// Fetch returns the body of a URL and a slice of URLs found on that page.
	Fetch(url string) (body string, urls []string, err error)
}

// Crawl uses a Fetcher to recursively crawl pages starting with url,
// to a maximum depth. It should use goroutines to fetch pages concurrently.
func Crawl(url string, depth int, fetcher Fetcher) {
	queue := []string{url}
	// TODO: bloom filter to minimize the seen map size
	seen := map[string]struct{}{url: {}}
	mu := sync.Mutex{}
	batchSize := 50
	for level := 1; level <= depth; level++ {
		nextqueue := []string{}
		// batch get url to prevent socket runout
		batchNum := (len(queue) + batchSize - 1) / batchSize
		for i:=0;i<batchNum;i++ {
			wg := sync.WaitGroup{}
			for _, ele := range queue[i*batchSize: min((i+1)*batchSize, len(queue))] {
				wg.Add(1)
				go func(url string) {
					defer wg.Done()
					body, urls, err := fetcher.Fetch(url)
					mu.Lock()
					defer mu.Unlock()
					if err != nil {
						fmt.Printf("failed to fetch %s %v\n", url, err)
						return
					} 
					fmt.Printf("found: %s %q\n", url, body)
					for _, ele := range urls {
						if _, ok := seen[ele]; !ok {
							seen[ele] = struct{}{}
							nextqueue = append(nextqueue, ele)
						}
					}
				}(ele)
			}
			wg.Wait()
		}
		if len(nextqueue) == 0 {
			break
		}
		queue = nextqueue
	}
}

func main() {
	// We'll use this main function to manually test our crawler.
	// The unit tests are the primary way to verify correctness.
	fmt.Println("Starting crawl...")
	Crawl("https://golang.org/", 4, fetcher)
	fmt.Println("Crawl finished.")
}

// fetcher is a populated fake Fetcher.
var fetcher = &fakeFetcher{
	"https://golang.org/": &fakeResult{
		"The Go Programming Language",
		[]string{
			"https://golang.org/pkg/",
			"https://golang.org/cmd/",
		},
	},
	"https://golang.org/pkg/": &fakeResult{
		"Packages",
		[]string{
			"https://golang.org/",
			"https://golang.org/cmd/",
			"https://golang.org/pkg/fmt/",
			"https://golang.org/pkg/os/",
		},
	},
	"https://golang.org/pkg/fmt/": &fakeResult{
		"Package fmt",
		[]string{
			"https://golang.org/",
			"https://golang.org/pkg/",
		},
	},
	"https://golang.org/pkg/os/": &fakeResult{
		"Package os",
		[]string{
			"https://golang.org/",
			"https://golang.org/pkg/",
		},
	},
}

// fakeFetcher is a Fetcher that returns canned results.
type fakeFetcher map[string]*fakeResult

type fakeResult struct {
	body string
	urls []string
}

func (f *fakeFetcher) Fetch(url string) (string, []string, error) {
	if res, ok := (*f)[url]; ok {
		return res.body, res.urls, nil
	}
	return "", nil, fmt.Errorf("not found: %s", url)
}