package crawler

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"sync"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

// errors ...
var (
	ErrPageFailed = errors.New("url path failed to respond, possible dead")
	ErrNonHTMLURL = errors.New("path points to a non html path")
)

// Status embodies data used to represent a giving links state status.
type Status struct {
	IsLive      bool      `json:"is_live"`
	IsCrawlable bool      `json:"is_crawlable"`
	LastStatus  int       `json:"last_status"`
	At          time.Time `json:"at"`
	Reason      error     `json:"reason,omitemtpy"`
}

// LinkReport embodies a the data reports for a giving path.
type LinkReport struct {
	Path     *url.URL     `json:"path"`
	Status   Status       `json:"status"`
	PointsTo []LinkReport `json:"points_to"`
}

// PageCrawler implements a web crawler which runs through a provided
// target path retrieving all links that lies relative to the host of
// the target path.
type PageCrawler struct {
	// MaxDepth sets the maximum total depth to be searched by the crawler until
	// it stops.
	MaxDepth int

	// Target is the parsed target url to be crawled.
	Target *url.URL

	// Verbose dictates that PageCrawler print current scanning target.
	Verbose bool

	current int
	seen    *HasSet
	child   bool
	report  *LinkReport
	waiter  *sync.WaitGroup
}

// Run initializes the target url crawling all pages url paths retrieved from
// the target's body content. It crawls deeply into all pages based on giving depth
// desired.
func (pc PageCrawler) Run(ctx context.Context, client *http.Client, pool WorkerPool, reports chan<- LinkReport) {
	if pc.waiter == nil {
		pc.waiter = new(sync.WaitGroup)
	}

	if pc.seen == nil {
		pc.seen = NewHasSet()
	}

	if !pc.child {
		pc.waiter.Add(1)
		go func() {
			pc.waiter.Wait()
			close(reports)
		}()
	}

	defer pc.waiter.Done()

	if pc.Verbose {
		defer fmt.Printf("Done scanning %+q from %q.\n", pc.Target.Path, pc.Target.Host)
	}

	// if MaxDepth was left unset, set it to infinity(-1).
	if pc.MaxDepth == 0 {
		pc.MaxDepth = -1
	}

	trimmed := strings.TrimSuffix(pc.Target.Path, "/")
	if trimmed == "" {
		trimmed = "/"
	}

	// if we have have an attached seen map, then check if requests
	// has already being added to the seen map and marked as processed or
	// in-process.
	if pc.seen.Has(trimmed) {
		return
	}

	// Have we max'ed out desired depth, then stop.
	if pc.MaxDepth > 0 && pc.current >= pc.MaxDepth {
		return
	}

	// Add target into seen map immediately.
	pc.seen.Add(trimmed)

	select {
	case <-ctx.Done():
		return
	default:
		if pc.Verbose {
			fmt.Printf("Scanning %+q from %q.\n", pc.Target.Path, pc.Target.Host)
		}

		var report LinkReport
		if pc.report == nil {
			report.Path = pc.Target
			report.Status = getURLStatus(client, pc.Target)
		} else {
			report = *pc.report
			if pc.Verbose {
				fmt.Printf("Scanning %+q from source %+q.\n", pc.Target.Path, report.Path.Path)
			}
		}

		// check url status if the page is live, else skip.
		if !report.Status.IsLive {
			reports <- report
			return
		}

		// if report indicates it's a live page but not something we can crawl with, maybe due to content-type, then skip.
		if report.Status.IsLive && !report.Status.IsCrawlable {
			reports <- report
			return
		}

		// Retrieve path's body for scanning, else skip if and update status.
		pathBody, err := exploreURL(client, pc.Target)
		if err != nil {
			report.Status.IsLive = false
			reports <- report
			return
		}

		defer pathBody.Close()

		// Use BodyCrawler to retrieve page's internal children links.
		// Skip if we failed to get children.
		// TODO: Should we update isLive status here? Does failure here warrant change?
		report.PointsTo, err = CrawlBody(client, pc.Target, pathBody)
		if err != nil {
			reports <- report
			return
		}

		// Deliver target's report.
		reports <- report

		nextDepth := pc.current + 1

		// Issue new PageCrawlers for target's kids and update waitgroup worker count.
		for _, kid := range report.PointsTo {
			kidPath := strings.TrimSuffix(kid.Path.Path, "/")
			if kidPath == "" {
				continue
			}

			if !kid.Status.IsCrawlable {
				continue
			}

			if pc.seen.Has(kidPath) {
				continue
			}

			pc.waiter.Add(1)

			// Attempt to secure worker service, if failed, drop request counter.
			// Fix issue with kid report leaking into future goroutines.
			go func(k LinkReport) {
				kidCrawler := PageCrawler{
					child:    true,
					report:   &k,
					Target:   k.Path,
					seen:     pc.seen,
					waiter:   pc.waiter,
					Verbose:  pc.Verbose,
					MaxDepth: pc.MaxDepth,
					current:  nextDepth,
				}

				if err := pool.Add(func() { kidCrawler.Run(ctx, client, pool, reports) }); err != nil {
					pc.waiter.Done()
				}
			}(kid)
		}
	}
}

// CrawlBody starts the internal logic of the body crawler to retrieve all
// internal routes of the target page. It takes into account all paths
// that are relative to the target's root.
// The crawler is strict in that it will only crawl path in the same host
// as the root. So paths like web.monzo.com is not within root of monzo.com,
// and will not be crawled.
func CrawlBody(client *http.Client, target *url.URL, body io.Reader) ([]LinkReport, error) {
	var kids []LinkReport

	links := farmWithHTML(body, target)
	for link := range links {
		if link.Host != target.Host {
			continue
		}

		kids = append(kids, LinkReport{
			Path:   link,
			Status: getURLStatus(client, link),
		})
	}

	return kids, nil
}

func getURLStatus(client *http.Client, target *url.URL) Status {
	now := time.Now()
	res, err := client.Head(target.String())
	if err != nil {
		return Status{
			Reason:     err,
			At:         now,
			LastStatus: http.StatusInternalServerError,
		}
	}

	if res.StatusCode < 200 || res.StatusCode > 299 {
		return Status{
			At:         now,
			LastStatus: res.StatusCode,
			Reason:     ErrPageFailed,
		}
	}

	if !strings.Contains(res.Header.Get("Content-Type"), "text/html") &&
		!strings.Contains(res.Header.Get("Content-Type"), "text/xhtml") {
		return Status{
			At:         now,
			IsLive:     true,
			LastStatus: res.StatusCode,
			Reason:     ErrNonHTMLURL,
		}
	}

	return Status{
		LastStatus:  res.StatusCode,
		IsLive:      true,
		At:          now,
		IsCrawlable: true,
	}
}

// exploreURL attempts to retrieve content of path and validate that path is a valid html
// link which can be crawled.
func exploreURL(client *http.Client, target *url.URL) (io.ReadCloser, error) {
	res, err := client.Get(target.String())
	if err != nil {
		return nil, err
	}

	if res.StatusCode < 200 || res.StatusCode > 299 {
		return nil, ErrPageFailed
	}

	if !strings.Contains(res.Header.Get("Content-Type"), "text/html") &&
		!strings.Contains(res.Header.Get("Content-Type"), "text/xhtml") {
		return nil, ErrNonHTMLURL
	}

	return res.Body, nil
}

// farmWithGoquery takes a given url and retrieves the needed links associated with
// that URL.
func farmWithGoquery(content io.Reader, rootURL *url.URL) (map[*url.URL]struct{}, error) {
	doc, err := goquery.NewDocumentFromReader(content)
	if err != nil {
		return nil, err
	}

	urlMap := make(map[*url.URL]struct{}, 0)

	// Collect all href links within the document. This way we can capture
	// external,internal and stylesheets within the page.
	hrefs := doc.Find("[href]")
	for i := 0; i < hrefs.Length(); i++ {
		if item, ok := getAttr(hrefs.Get(i).Attr, "href"); ok {
			trimmedPath := strings.TrimSpace(item.Val)
			if !strings.Contains(trimmedPath, "javascript:void(0)") {
				if parsedPath, err := parsePath(trimmedPath, rootURL); err == nil {
					urlMap[parsedPath] = struct{}{}
				}
			}
		}
	}

	// Collect all src links within the document. This way we can capture
	// external,internal and stylesheets within the page.
	srcs := doc.Find("[src]")
	for i := 0; i < srcs.Length(); i++ {
		if item, ok := getAttr(srcs.Get(i).Attr, "src"); ok {
			trimmedPath := strings.TrimSpace(item.Val)
			if !strings.Contains(trimmedPath, "javascript:void(0)") {
				if parsedPath, err := parsePath(trimmedPath, rootURL); err == nil {
					urlMap[parsedPath] = struct{}{}
				}
			}
		}
	}

	return urlMap, nil
}

func farmWithHTML(content io.Reader, rootURL *url.URL) map[*url.URL]struct{} {
	tokenizer := html.NewTokenizer(content)
	urlMap := make(map[*url.URL]struct{}, 0)

	for {
		switch tokenizer.Next() {
		case html.ErrorToken:
			return urlMap
		case html.CommentToken:
			continue
		case html.SelfClosingTagToken, html.StartTagToken:
			token := tokenizer.Token()

			// if we dont have any attribute then skip.
			if len(token.Attr) == 0 {
				continue
			}

			for _, attr := range token.Attr {
				switch strings.ToLower(attr.Key) {
				case "href":
					if strings.Contains(attr.Val, "javascript:void(0)") {
						continue
					}

					if parsedPath, err := parsePath(attr.Val, rootURL); err == nil {
						urlMap[parsedPath] = struct{}{}
					}
				case "src":
					if strings.Contains(attr.Val, "javascript:void(0)") {
						continue
					}

					if parsedPath, err := parsePath(attr.Val, rootURL); err == nil {
						urlMap[parsedPath] = struct{}{}
					}
				case "srcset":
					for _, item := range strings.Split(attr.Val, ",") {
						if strings.Contains(item, "javascript:void(0)") {
							continue
						}

						if parsedPath, err := parsePath(item, rootURL); err == nil {
							urlMap[parsedPath] = struct{}{}
						}
					}
				}
			}
		}
	}

	return urlMap
}

// getAttr returns the giving attribute for a specific name type if found.
func getAttr(attrs []html.Attribute, key string) (attr html.Attribute, found bool) {
	for _, attr = range attrs {
		if attr.Key == key {
			found = true
			return
		}
	}
	return
}

// parsePath re-evaluates a giving path string using a root URL path, else
// returns an error if it fails to validate path as a valid url.
func parsePath(path string, index *url.URL) (*url.URL, error) {
	pathURI, err := url.Parse(path)
	if err != nil {
		return nil, err
	}

	if !pathURI.IsAbs() {
		pathURI = index.ResolveReference(pathURI)
	}

	return pathURI, nil
}
