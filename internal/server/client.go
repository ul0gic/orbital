package server

import (
	"net/http"
	"strings"
)

// clientHint derives the events.Client field from Cloudflare's real-client
// header plus a coarse user-agent family. CF-Connecting-IP is the authentic
// remote address behind the tunnel; RemoteAddr is always the local proxy.
func clientHint(r *http.Request) string {
	ip := r.Header.Get("CF-Connecting-IP")
	if ip == "" {
		ip = "unknown"
	}
	family := uaFamily(r.UserAgent())
	if family == "" {
		return ip
	}
	return family + ", " + ip
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
