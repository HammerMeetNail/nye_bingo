package httpx

import (
	"context"
	"net"
	"net/http"
	"net/netip"
)

type trustedForwardedHeadersKey struct{}

// WithTrustedForwardedHeaders marks whether X-Forwarded-* headers should be trusted for this request.
func WithTrustedForwardedHeaders(r *http.Request, trusted bool) *http.Request {
	ctx := context.WithValue(r.Context(), trustedForwardedHeadersKey{}, trusted)
	return r.WithContext(ctx)
}

// TrustedForwardedHeaders reports whether X-Forwarded-* headers should be trusted for this request.
func TrustedForwardedHeaders(r *http.Request) bool {
	v := r.Context().Value(trustedForwardedHeadersKey{})
	trusted, ok := v.(bool)
	return ok && trusted
}

type TrustedProxyChecker struct {
	prefixes []netip.Prefix
}

// NewTrustedProxyChecker builds a checker from CIDR strings. If no CIDRs are provided, the checker trusts
// loopback and RFC1918/private addresses by default (suitable for local reverse proxies / tunnels).
func NewTrustedProxyChecker(trustedProxyCIDRs []string) (*TrustedProxyChecker, error) {
	prefixes := make([]netip.Prefix, 0, len(trustedProxyCIDRs))
	for _, raw := range trustedProxyCIDRs {
		if raw == "" {
			continue
		}
		p, err := netip.ParsePrefix(raw)
		if err != nil {
			return nil, err
		}
		prefixes = append(prefixes, p)
	}
	return &TrustedProxyChecker{prefixes: prefixes}, nil
}

func (c *TrustedProxyChecker) IsTrustedRequest(r *http.Request) bool {
	ip, ok := remoteIP(r)
	if !ok {
		return false
	}
	return c.IsTrustedIP(ip)
}

func (c *TrustedProxyChecker) IsTrustedIP(ip netip.Addr) bool {
	if len(c.prefixes) == 0 {
		return ip.IsLoopback() || ip.IsPrivate()
	}
	for _, p := range c.prefixes {
		if p.Contains(ip) {
			return true
		}
	}
	return false
}

func remoteIP(r *http.Request) (netip.Addr, bool) {
	host := r.RemoteAddr
	if h, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		host = h
	}
	ip, err := netip.ParseAddr(host)
	if err != nil {
		return netip.Addr{}, false
	}
	return ip, true
}
