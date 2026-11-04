package analytics

import (
	"crypto/sha256"
	"encoding/hex"
	"net"
	"strings"
)

// TruncateIP reduces a client address before lookup or hashing.
// IPv4 keeps the /24 network. IPv6 keeps the /48 network.
// The original address is not returned and must not be stored.
func TruncateIP(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.ContainsAny(raw, "\r\n\x00 ") {
		return "", false
	}
	ip := net.ParseIP(raw)
	if ip == nil {
		return "", false
	}
	if v4 := ip.To4(); v4 != nil {
		out := make(net.IP, len(v4))
		copy(out, v4)
		out[3] = 0
		return out.String(), true
	}
	v6 := ip.To16()
	if v6 == nil {
		return "", false
	}
	out := make(net.IP, 16)
	copy(out, v6)
	for i := 6; i < 16; i++ {
		out[i] = 0
	}
	return out.String(), true
}

// VisitorHash is a daily visitor key. It is not the address and it changes the next UTC day.
func VisitorHash(salt, day, truncated string) string {
	sum := sha256.Sum256([]byte(salt + "|" + day + "|" + truncated))
	return hex.EncodeToString(sum[:8])
}

// SanitizePath keeps a request path safe to store and display.
// The query string and fragment are dropped. Dot-dot segments are dropped.
func SanitizePath(raw string) string {
	if strings.ContainsRune(raw, 0) || strings.ContainsAny(raw, "\r\n") {
		return "/other"
	}
	if i := strings.IndexAny(raw, "?#"); i >= 0 {
		raw = raw[:i]
	}
	if raw == "" {
		return "/"
	}
	if !strings.HasPrefix(raw, "/") {
		raw = "/" + raw
	}
	parts := strings.Split(raw, "/")
	kept := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			continue
		}
		if len(part) > 80 {
			part = part[:80]
		}
		kept = append(kept, part)
	}
	if len(kept) == 0 {
		return "/"
	}
	out := "/" + strings.Join(kept, "/")
	if len(out) > 160 {
		out = out[:160]
	}
	return out
}

// SanitizeHost normalizes an HTTP Host header. Empty is empty.
// Anything that is not a hostname or address becomes "other".
func SanitizeHost(raw string) string {
	raw = strings.TrimSpace(strings.ToLower(raw))
	if raw == "" {
		return ""
	}
	if strings.ContainsAny(raw, "\r\n\x00 /\\") {
		return "other"
	}
	if host, _, err := net.SplitHostPort(raw); err == nil {
		raw = host
	} else if i := strings.LastIndex(raw, ":"); i > 0 && strings.Count(raw, ":") == 1 {
		raw = raw[:i]
	}
	if raw == "" || len(raw) > 253 {
		return "other"
	}
	for _, c := range raw {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '.' || c == '-' || c == ':' || c == '[' || c == ']' {
			continue
		}
		return "other"
	}
	return raw
}

// SanitizeProject accepts a short project id. Invalid input is empty, not a cleaned guess.
func SanitizeProject(raw string) string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	if raw == "" || len(raw) > 63 || strings.Contains(raw, "..") {
		return ""
	}
	for _, c := range raw {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '.' || c == '_' || c == '-' {
			continue
		}
		return ""
	}
	return raw
}

// DeviceType classifies a User-Agent into a small fixed set.
func DeviceType(ua string) string {
	u := strings.ToLower(strings.TrimSpace(ua))
	if u == "" {
		return "unknown"
	}
	if strings.Contains(u, "bot") || strings.Contains(u, "spider") || strings.Contains(u, "crawler") {
		return "bot"
	}
	if strings.Contains(u, "ipad") || strings.Contains(u, "tablet") {
		return "tablet"
	}
	if strings.Contains(u, "mobile") || strings.Contains(u, "iphone") || strings.Contains(u, "android") {
		return "mobile"
	}
	return "desktop"
}
