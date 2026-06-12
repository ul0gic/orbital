package visitor

import (
	"net/http"
	"strconv"
	"strings"
	"sync"
)

// Registry numbers distinct clients for the live log. The real address is
// only used as a map key and never leaves this package — the log shows
// "visitor N" so sender and recipient never learn each other's IPs.
type Registry struct {
	mu   sync.Mutex
	seen map[string]int
}

func NewRegistry() *Registry {
	return &Registry{seen: make(map[string]int)}
}

func (g *Registry) Hint(r *http.Request) string {
	tag := g.tag(r.Header.Get("CF-Connecting-IP"))
	family := uaFamily(r.UserAgent())
	if family == "" {
		return tag
	}
	return family + ", " + tag
}

func (g *Registry) tag(ip string) string {
	g.mu.Lock()
	defer g.mu.Unlock()
	n, ok := g.seen[ip]
	if !ok {
		n = len(g.seen) + 1
		g.seen[ip] = n
	}
	return "visitor " + strconv.Itoa(n)
}

func uaFamily(ua string) string {
	ua = strings.ToLower(ua)
	switch {
	case ua == "":
		return ""
	case strings.Contains(ua, "edg/"):
		return "Edge"
	case strings.Contains(ua, "chrome/"):
		return "Chrome"
	case strings.Contains(ua, "firefox/"):
		return "Firefox"
	case strings.Contains(ua, "safari/"):
		return "Safari"
	case strings.Contains(ua, "curl/"):
		return "curl"
	case strings.Contains(ua, "wget/"):
		return "wget"
	default:
		return "other"
	}
}
