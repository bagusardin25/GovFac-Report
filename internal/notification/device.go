package notification

import "strings"

type uaToken struct {
	token, name string
}

// Order matters: Edge and Opera UAs also contain "Chrome", Chrome's contains
// "Safari", iOS UAs contain "Mac OS X" and Android UAs contain "Linux".
var (
	browsers = []uaToken{
		{"Edg/", "Edge"},
		{"OPR/", "Opera"},
		{"SamsungBrowser/", "Samsung Internet"},
		{"Firefox/", "Firefox"},
		{"Chrome/", "Chrome"},
		{"Safari/", "Safari"},
	}
	systems = []uaToken{
		{"Windows", "Windows"},
		{"Android", "Android"},
		{"iPhone", "iPhone"},
		{"iPad", "iPad"},
		{"Mac OS X", "macOS"},
		{"Linux", "Linux"},
	}
)

// We only need something a person can recognise ("Chrome di Windows"), not a
// full UA parser.
func describeDevice(userAgent string) string {
	browser := firstMatch(userAgent, browsers)
	system := firstMatch(userAgent, systems)
	switch {
	case browser != "" && system != "":
		return browser + " di " + system
	case browser != "":
		return browser
	case system != "":
		return system
	}

	product, _, _ := strings.Cut(strings.TrimSpace(userAgent), " ")
	if product == "" {
		return "perangkat tidak dikenal"
	}
	if len(product) > 40 {
		product = product[:40]
	}
	return product
}

func firstMatch(userAgent string, candidates []uaToken) string {
	for _, c := range candidates {
		if strings.Contains(userAgent, c.token) {
			return c.name
		}
	}
	return ""
}
