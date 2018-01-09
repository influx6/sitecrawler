package crawler

import (
	"context"
	"errors"
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

	// Waiter is the waitgroup supplied by user to ensure end of all goroutines
	// launched by crawler.
	Waiter *sync.WaitGroup

	current int
	seen    *HasSet
	child   bool
}

// Run initializes the target url crawling all pages url paths retrieved from
// the target's body content. It crawls deeply into all pages based on giving depth
// desired.
func (pc PageCrawler) Run(ctx context.Context, client *http.Client, reports chan<- LinkReport) {
	defer pc.Waiter.Done()

	// if we are the root, launch a routine to wait on the wait group, before closing the report channel.
	if !pc.child {
		go func() {
			pc.Waiter.Wait()
			close(reports)
		}()
	}

	// if MaxDepth was left unset, set it to infinity(-1).
	if pc.MaxDepth == 0 {
		pc.MaxDepth = -1
	}

	// if we have have an attached seen map, then check if requests
	// has already being added to the seen map and marked as processed or
	// in-process.
	if pc.seen != nil && pc.seen.Has(pc.Target.String()) {
		return
	}

	// Have we max'ed out desired depth, then stop.
	if pc.MaxDepth > 0 && pc.current >= pc.MaxDepth {
		return
	}

	// Get Has Map if available else create new one.
	seenMap := pc.seen
	if seenMap == nil {
		seenMap = NewHasSet()
	}

	// Add target into seen map immediately.
	seenMap.Add(pc.Target.String())

	select {
	case <-ctx.Done():
		// We are told to stop, so quit immediately.
		return
	default:
		var report LinkReport
		report.Path = pc.Target
		report.Status = getURLStatus(client, pc.Target)

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

		// Use BodyCrawler to retrieve page's internal children links.
		// Skip if we failed to get children.
		// TODO: Should we update isLive status here? Does failure here warrant change?
		report.PointsTo, err = (BodyCrawler{Target: pc.Target, Body: pathBody}).Run(client)
		if err != nil {
			reports <- report
			return
		}

		// Issue new PageCrawlers for target's kids and update waitgroup worker count.
		for _, kid := range report.PointsTo {
			pc.Waiter.Add(1)

			go (PageCrawler{
				child:    true,
				seen:     seenMap,
				Target:   kid.Path,
				Waiter:   pc.Waiter,
				MaxDepth: pc.MaxDepth,
				current:  pc.current + 1,
			}).Run(ctx, client, reports)
		}

		// Deliver target's report.
		reports <- report
	}
}

// BodyCrawler attempts to retrieve a provided target url scanning it's associated
// body and retrieving all routes within path.
type BodyCrawler struct {
	Target *url.URL
	Body   io.Reader
}

// Run starts the internal logic of the body crawler to retrieve all
// internal routes of the target page. It takes into account all paths
// that are relative to the target's root.
// The crawler is strict in that it will only crawl path in the same host
// as the root. So paths like web.monzo.com is not within root of monzo.com,
// and will not be crawled.
func (bc BodyCrawler) Run(client *http.Client) ([]LinkReport, error) {
	links, err := farm(bc.Body, bc.Target)
	if err != nil {
		return nil, err
	}

	var kids []LinkReport

	for link := range links {
		if link.Host != bc.Target.Host {
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
	res, err := client.Head(target.String())
	if err != nil {
		return Status{
			Reason:     err,
			At:         time.Now(),
			LastStatus: http.StatusInternalServerError,
		}
	}

	if res.StatusCode < 200 || res.StatusCode > 299 {
		return Status{
			At:         time.Now(),
			LastStatus: res.StatusCode,
			Reason:     ErrPageFailed,
		}
	}

	if !strings.Contains(res.Header.Get("Content-Type"), "text/html") &&
		!strings.Contains(res.Header.Get("Content-Type"), "text/xhtml") {
		return Status{
			At:         time.Now(),
			IsLive:     true,
			LastStatus: res.StatusCode,
			Reason:     ErrNonHTMLURL,
		}
	}

	return Status{
		LastStatus:  res.StatusCode,
		IsLive:      true,
		At:          time.Now(),
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

// farm takes a given url and retrieves the needed links associated with
// that URL.
func farm(content io.Reader, rootURL *url.URL) (map[*url.URL]struct{}, error) {
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
