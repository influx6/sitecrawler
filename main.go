package main

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"sync"

	"net/http"
	"time"

	"os"

	"github.com/influx6/faux/flags"
	"github.com/influx6/faux/tmplutil"
	"github.com/influx6/sitecrawler/crawler"
)

var (
	sitemapTemplate = tmplutil.MustFrom("sitecrawler", `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">{{ range . }}
	<url>
		<loc>{{.Path.String }}</loc>
		<laststatus>{{.Status.LastStatus}}</laststatus>
		<lastchecked>{{.Status.At.UTC}}</lastchecked>
		<reachable>{{.Status.IsLive}}</reachable>
		<crawlable>{{.Status.IsCrawlable}}</crawlable>
		{{ if notequal .Status.Reason nil }}<reachable_error>{{.Status.Reason.Error }}</reachable_error>
		<connects>{{ range .PointsTo }}
			<link>{{.Path.String }}</link>
		{{end}}</connects>{{else}}<connects>
		{{ range .PointsTo }}
			<link>{{.Path.String }}</link>
		{{end}}</connects>{{end}}
	</url>
{{ end }}</urlset>
`)
)

func main() {
	flags.Run("sitecrawler", flags.Command{
		Name:         "crawl",
		AllowDefault: true,
		ShortDesc:    "Crawls provided website URL returning json sitemap.",
		Desc:         "Crawl is the entry command to crawl a website, it runs through all pages of giving host, ignoring externals links. It prints status and link connection as json on a per link basis.",
		Usages:       []string{"sitecrawler crawl https://monzo.com"},
		Flags: []flags.Flag{
			&flags.IntFlag{
				Name:    "depth",
				Default: -1,
				Desc:    "Sets the depth to crawl through giving site",
			},
			&flags.BoolFlag{
				Name:    "verbose",
				Default: false,
				Desc:    "Sets the flag to ensure crawler prints current target.",
			},
			&flags.BoolFlag{
				Name: "timed",
				Desc: "Sets the flag to time operation.",
			},
			&flags.DurationFlag{
				Name:    "timeout",
				Default: time.Second * 5,
				Desc:    "Sets timeout for http.Client to be used",
			},
		},
		Action: func(ctx flags.Context) error {
			if len(ctx.Args()) == 0 {
				return errors.New("must provide website url for crawling. Run `crawl help`")
			}

			start := time.Now()
			timeout, _ := ctx.GetDuration("timeout")
			verbose, _ := ctx.GetBool("verbose")

			client := &http.Client{Timeout: timeout}

			targetURL := ctx.Args()[0]
			target, err := url.Parse(targetURL)
			if err != nil {
				return fmt.Errorf("url error: %+s for %+q", err, targetURL)
			}

			waiter := new(sync.WaitGroup)
			waiter.Add(1)

			var pages crawler.PageCrawler
			pages.Target = target
			pages.MaxDepth = -1
			pages.Waiter = waiter
			pages.Verbose = verbose

			reports := make(chan crawler.LinkReport)

			go pages.Run(context.Background(), client, reports)

			var records []crawler.LinkReport
			for report := range reports {
				records = append(records, report)
			}

			waiter.Wait()

			if err := sitemapTemplate.Execute(os.Stdout, records); err != nil {
				return fmt.Errorf("parseError:  %+s", err)
			}

			if timed, _ := ctx.GetBool("timed"); timed {
				fmt.Fprintf(os.Stderr, "\nFinished: %+s.\n", time.Now().Sub(start))
			}
			return nil
		},
	})
}
