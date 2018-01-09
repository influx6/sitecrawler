package crawler_test

import (
	"bytes"
	"context"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"

	"github.com/influx6/sitecrawler/crawler"
)

var server = httptest.NewServer(testHandler{})

func BenchmarkBodyCrawler_Run(b *testing.B) {
	b.StopTimer()
	b.ReportAllocs()

	target, err := url.Parse("http://mombo.com")
	if err != nil {
		panic(err)
	}

	body := bytes.NewReader(indexPage)

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		crawler.CrawlBody(baseClient, target, body)
	}
	b.StopTimer()
}

func BenchmarkPageCrawler_Run(b *testing.B) {
	b.StopTimer()
	b.ReportAllocs()

	target, err := url.Parse(server.URL + "/")
	if err != nil {
		panic(err)
	}

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		waiter := new(sync.WaitGroup)
		waiter.Add(1)

		var pages crawler.PageCrawler
		pages.Target = target
		pages.MaxDepth = -1
		pages.Waiter = waiter

		reports := make(chan crawler.LinkReport, 10)
		go pages.Run(context.Background(), baseClient, reports)
		waiter.Wait()
		emptyChan(reports)
	}
	b.StopTimer()
}

func emptyChan(r chan crawler.LinkReport) {
	for range r {
	}
}
