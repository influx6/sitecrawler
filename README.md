SiteCrawler
----------------
Sitecrawler is a sample project showcasing a simple web crawler which generates a simple sitemap for relative paths/links of a giving website. It does not explore external links.


## Install

```bash
> go get -u github.com/influx6/sitecrawler
```


## Run

- Run `sitecrawler crawl [target_url]` to crawl target website. 


```bash
> sitecrawler crawl https://monzo.com
```

- Run `sitecrawler` to see CLI options

```bash
> sitecrawler 

Usage: sitecrawler [flags] [command] 

⡿ COMMANDS:
	⠙ crawl        Crawls provided website URL returning json sitemap.

⡿ HELP:
	Run [command] help

⡿ OTHERS:
	Run 'sitecrawler flags' to print all flags of all commands.

⡿ WARNING:
	Uses internal flag package so flags must precede command name. 
	e.g 'sitecrawler -cmd.flag=4 run'
```

- Run `sitecrawler crawl help` to see CLI options


```bash
Command: sitecrawler [flags] crawl 

⡿ DESC:
	Crawl is the entry command to crawl a website, it runs through all pages of giving host, ignoring externals links. It prints status and link connection as json on a per link basis.

⡿ Flags:
	
	⠙ crawl.depth		Default: -1	Desc: Sets the depth to crawl through giving site
	
	⠙ crawl.verbose		Default: false	Desc: Sets the flag to ensure crawler prints current target.
	
	⠙ crawl.timed		Default: false	Desc: Sets the flag to time operation.
	
	⠙ crawl.timeout		Default: 5s	Desc: Sets timeout for http.Client to be used
	
⡿ Examples:
	
	⠙ sitecrawler crawl https://monzo.com
	
⡿ USAGE:
	
	⠙ sitecrawler -crawl.depth=-1 crawl 
	
	⠙ sitecrawler -crawl.verbose=false crawl 
	
	⠙ sitecrawler -crawl.timed=false crawl 
	
	⠙ sitecrawler -crawl.timeout=5s crawl 
	
⡿ OTHERS:
	Commands which respect context.Context, can set timeout by using the -timeout flag.
	e.g -timeout=4m, -timeout=4h

⡿ WARNING:
	Uses internal flag package so flags must precede command name. 
	e.g 'sitecrawler -cmd.flag=4 run'

```

