package crawler

import (
	"bytes"
	"io"
	"log"
	"os"
	"strings"
	"testing"
)

// TestCrawlConcurrent is the test case for our crawler.
// It checks for two main things:
// 1. That the crawler finds all pages in our fake dataset.
// 2. That it doesn't fetch the same URL twice.
func TestCrawlConcurrent(t *testing.T) {
	// Capture the output of the Crawl function
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	log.SetOutput(w)

	// Run the crawler
	Crawl("https://golang.org/", 4, fetcher)

	// Restore stdout and read the captured output
	w.Close()
	os.Stdout = old
	log.SetOutput(os.Stdout)

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	// The expected URLs that should be found by the crawler
	expectedUrls := []string{
		"https://golang.org/",
		"https://golang.org/pkg/",
		"https://golang.org/pkg/fmt/",
		"https://golang.org/pkg/os/",
	}

	// 1. Check if all expected URLs were printed
	for _, url := range expectedUrls {
		if !strings.Contains(output, url) {
			t.Errorf("Expected to find URL %q in output, but didn't.\nOutput:\n%s", url, output)
		}
	}

	// 2. Check for duplicate fetching.
	// We count the occurrences of "found:" in the output.
	// If it's more than the number of unique pages, a URL was fetched more than once.
	lines := strings.Split(output, "\n")
	foundCount := 0
	for _, line := range lines {
		if strings.HasPrefix(line, "found:") {
			foundCount++
		}
	}

	if foundCount > len(expectedUrls) {
		t.Errorf("Detected duplicate fetching. Expected %d unique URLs, but found %d 'found:' lines.\nOutput:\n%s", len(expectedUrls), foundCount, output)
	}

	if foundCount < len(expectedUrls) {
		t.Errorf("Didn't find all URLs. Expected %d, but only found %d 'found:' lines.\nOutput:\n%s", len(expectedUrls), foundCount, output)
	}
}