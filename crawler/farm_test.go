package crawler

import (
	"bytes"
	"fmt"
	"net/url"
	"testing"

	"github.com/influx6/faux/tests"
)

var (
	expectedPaths = map[string]bool{
		"/assets/member2-66485427ca4bd140e0547efb1ce12ce0.png": true,
		"/assets/member4-cfa03a1a15aed816528b8ec1ee6c95c6.png": true,
		"/assets/member5-6ee6a979c39c81e2b652f268cccaf265.png": true,
		"/contacts":                                                 true,
		"https://www.youtube.com/player_api":                        true,
		"/assets/ardan-symbol-93ee488d16f9bc56ad65659c2d8f41dc.png": true,
		"/assets/application-9709f2e1ad6d5ec24402f59507f6822b.js":   true,
		"/assets/member1-55a2b7ac0a868d49fdf50ce39f0ce1ac.png":      true,
		"/services":                                                true,
		"http://youtube.com/x8433j4i":                              true,
		"http://gracehound.com/index":                              true,
		"/assets/application-4b77637cc302ef4af6c358864df26f88.css": true,
		"/bootstrap/css/bootstrap.css":                             true,
		"/assets/application-valum.js.js":                          true,
	}

	samplePage = []byte(`
		<!DOCTYPE html>
		<html>
		<head>
		<meta charset="UTF-8">
			<title>Ardan Studios</title>
			<link href="/assets/application-4b77637cc302ef4af6c358864df26f88.css" media="screen" rel="stylesheet" />
			<link rel="stylesheet" href="/bootstrap/css/bootstrap.css">
			<script src="https://www.youtube.com/player_api"></script>
		</head>
		<body>
				<script src="/assets/application-9709f2e1ad6d5ec24402f59507f6822b.js"></script>
				<script src="/assets/application-valum.js.js"></script>

				<a href="/services"></a>
				<a href="/contacts"></a>

				<a href="http://youtube.com/x8433j4i"></a>
				<a href="http://gracehound.com/index"></a>
				<img class="ardan-symbol" src="/assets/ardan-symbol-93ee488d16f9bc56ad65659c2d8f41dc.png" />
				<img src="/assets/member1-55a2b7ac0a868d49fdf50ce39f0ce1ac.png" />
				<img src="/assets/member2-66485427ca4bd140e0547efb1ce12ce0.png" />
				<img src="/assets/member4-cfa03a1a15aed816528b8ec1ee6c95c6.png" />
				<img src="/assets/member5-6ee6a979c39c81e2b652f268cccaf265.png" />
		</body>
		</html>
	`)
)

func TestLinkFarm(t *testing.T) {
	body := bytes.NewReader(samplePage)
	target, err := url.Parse("")
	if err != nil {
		tests.Failed("Should have successfully parsed url path")
	}
	tests.Passed("Should have successfully parsed url path")

	farmedLinks, err := farm(body, target)
	if err != nil {
		tests.FailedWithError(err, "Should have successfully farmed all page links")
	}
	tests.Passed("Should have successfully farmed all page links")

	if len(farmedLinks) != len(expectedPaths) {
		tests.Info("Expected Length: %d", len(expectedPaths))
		tests.Info("Received Length: %d", len(farmedLinks))
		fmt.Printf("%+q", farmedLinks)
		tests.Failed("Should have received same length of expected path from farmed links")
	}
	tests.Passed("Should have received same length of expected path from farmed links")

	for item := range expectedPaths {
		var found bool
		for elem := range farmedLinks {
			if elem.Path != item && elem.String() != item {
				continue
			}
			found = true
		}

		if !found {
			tests.Info("Expected in Farm: %q", item)
			tests.Failed("Should have found url path in farmed links.")
		}
	}

	tests.Passed("Should have found all expected links in farmed page.")
}
