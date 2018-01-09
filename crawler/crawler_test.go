package crawler_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"sync"

	"context"

	"github.com/influx6/faux/tests"
	"github.com/influx6/sitecrawler/crawler"
)

var (
	baseClient = &http.Client{
		Timeout: 5 * time.Second,
	}

	indexPage = []byte(`
		<!DOCTYPE html>
		<html>
		<head>
		<meta charset="UTF-8">
			<title>Mumbo Jungle</title>
		</head>
		<body>
			<a href="/services"></a>
			<a href="/contacts"></a>
		</body>
		</html>
	`)

	jsonCard = []byte(`{}`)

	contactPage = []byte(`
		<!DOCTYPE html>
		<html>
		<head>
		<meta charset="UTF-8">
			<title>Mumbo Jungle: Contact Page</title>
		</head>
		<body>
			<a href="/"></a>
			<a href="/services"></a>
			<a href="/jsoncard"></a>
			<a href="https://twitter.com/wombat"></a>
		</body>
		</html>
	`)

	servicePage = []byte(`
		<!DOCTYPE html>
		<html>
		<head>
		<meta charset="UTF-8">
			<title>Mumbo Jungle: Service Page</title>
		</head>
		<body>
			<a href="/services"></a>
		</body>
		</html>
	`)
)

func TestPageCrawler(t *testing.T) {
	server := httptest.NewServer(testHandler{})
	target, err := url.Parse(server.URL + "/")
	if err != nil {
		tests.FailedWithError(err, "Should have successfully parsed url")
	}
	tests.Passed("Should have successfully parsed url")

	waiter := new(sync.WaitGroup)
	waiter.Add(1)

	var pages crawler.PageCrawler
	pages.Target = target
	pages.MaxDepth = -1
	pages.Waiter = waiter

	reports := make(chan crawler.LinkReport)

	go pages.Run(context.Background(), baseClient, reports)

	var counter int
	for range reports {
		counter++
	}

	if counter != 4 {
		tests.Info("Expected Links: %d", 4)
		tests.Info("Received Links: %d", counter)
		tests.Failed("Should have successfully retrieved 4 links from server")
	}
	tests.Passed("Should have successfully retrieved 4 links from server")

	waiter.Wait()
	tests.Passed("Should have successfully stopped all farmers")
}

func TestBodyCrawler(t *testing.T) {
	target, err := url.Parse("http://mombo.com")
	if err != nil {
		tests.FailedWithError(err, "Should have successfully parsed url")
	}
	tests.Passed("Should have successfully parsed url")

	tests.Header("When farming links from index page")
	{
		links, err := crawler.CrawlBody(baseClient, target, bytes.NewReader(indexPage))
		if err != nil {
			tests.FailedWithError(err, "Should have successfully scanned page")
		}
		tests.Passed("Should have successfully scanned page")

		if total := len(links); total != 2 {
			tests.Info("Expected Links: %d", 2)
			tests.Info("Received Links: %d", total)
			tests.Failed("Should have farmed out 2 links from page")
		}
		tests.Passed("Should have farmed out 2 links from page")
	}

	tests.Header("When farming links from service page")
	{
		links, err := crawler.CrawlBody(baseClient, target, bytes.NewReader(servicePage))
		if err != nil {
			tests.FailedWithError(err, "Should have successfully scanned page")
		}
		tests.Passed("Should have successfully scanned page")

		if total := len(links); total != 1 {
			tests.Info("Expected Links: %d", 1)
			tests.Info("Received Links: %d", total)
			tests.Failed("Should have farmed out 1 links from page")
		}
		tests.Passed("Should have farmed out 1 links from page")
	}

	tests.Header("When farming links from contacts page")
	{
		links, err := crawler.CrawlBody(baseClient, target, bytes.NewReader(contactPage))
		if err != nil {
			tests.FailedWithError(err, "Should have successfully scanned page")
		}
		tests.Passed("Should have successfully scanned page")

		if total := len(links); total != 3 {
			tests.Info("Expected Links: %d", 3)
			tests.Info("Received Links: %d", total)
			tests.Failed("Should have farmed out 3 links from page")
		}
		tests.Passed("Should have farmed out 3 links from page")
	}

}

type testHandler struct{}

func (t testHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.ToLower(r.Method) == "head" {
		if r.URL.Path != "/jsoncard" {
			w.Header().Set("Content-Type", "text/html")
		} else {
			w.Header().Set("Content-Type", "application/json")
		}
		return
	}

	switch r.URL.Path {
	case "/jsoncard":
		w.Header().Set("Content-Type", "application/json")
		w.Write(jsonCard)
		return
	case "/":
		w.Header().Set("Content-Type", "text/html")
		w.Write(indexPage)
		return
	case "/contacts":
		w.Header().Set("Content-Type", "text/html")
		w.Write(contactPage)
		return
	case "/services":
		w.Header().Set("Content-Type", "text/html")
		w.Write(servicePage)
		return
	}

	w.WriteHeader(http.StatusBadRequest)
}
