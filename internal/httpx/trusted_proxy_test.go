package httpx

import (
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
)

func TestWithTrustedForwardedHeaders(t *testing.T) {
	tests := []struct {
		name     string
		trusted  bool
		expected bool
	}{
		{"Sets trusted to true", true, true},
		{"Sets trusted to false", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req = WithTrustedForwardedHeaders(req, tt.trusted)
			if got := TrustedForwardedHeaders(req); got != tt.expected {
				t.Errorf("TrustedForwardedHeaders() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestTrustedForwardedHeaders_NotSet(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if got := TrustedForwardedHeaders(req); got != false {
		t.Errorf("TrustedForwardedHeaders() on fresh request = %v, want false", got)
	}
}

func TestNewTrustedProxyChecker(t *testing.T) {
	tests := []struct {
		name    string
		cidrs   []string
		wantErr bool
	}{
		{"Empty list", []string{}, false},
		{"Valid CIDRs", []string{"192.168.1.0/24", "10.0.0.0/8"}, false},
		{"Single host CIDR", []string{"192.168.1.1/32"}, false},
		{"IPv6 CIDR", []string{"2001:db8::/32"}, false},
		{"Empty string in list", []string{"", "192.168.1.0/24", ""}, false},
		{"Invalid CIDR", []string{"not-a-cidr"}, true},
		{"Missing prefix", []string{"192.168.1.1"}, true},
		{"Invalid IP", []string{"999.999.999.999/24"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker, err := NewTrustedProxyChecker(tt.cidrs)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewTrustedProxyChecker() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && checker == nil {
				t.Error("NewTrustedProxyChecker() returned nil checker without error")
			}
		})
	}
}

func TestTrustedProxyChecker_IsTrustedIP(t *testing.T) {
	tests := []struct {
		name     string
		cidrs    []string
		ip       string
		expected bool
	}{
		// No CIDRs: trust loopback and private
		{"No CIDRs, loopback IPv4", []string{}, "127.0.0.1", true},
		{"No CIDRs, loopback IPv6", []string{}, "::1", true},
		{"No CIDRs, private 10.x", []string{}, "10.0.0.1", true},
		{"No CIDRs, private 172.16.x", []string{}, "172.16.0.1", true},
		{"No CIDRs, private 192.168.x", []string{}, "192.168.1.1", true},
		{"No CIDRs, public IP", []string{}, "8.8.8.8", false},

		// With explicit CIDRs: only match those
		{"With CIDR, matching IP", []string{"192.168.1.0/24"}, "192.168.1.100", true},
		{"With CIDR, non-matching IP", []string{"192.168.1.0/24"}, "192.168.2.1", false},
		{"With CIDR, loopback not trusted", []string{"192.168.1.0/24"}, "127.0.0.1", false},
		{"With multiple CIDRs, first matches", []string{"10.0.0.0/8", "172.16.0.0/12"}, "10.255.255.255", true},
		{"With multiple CIDRs, second matches", []string{"10.0.0.0/8", "172.16.0.0/12"}, "172.31.0.1", true},
		{"With multiple CIDRs, none match", []string{"10.0.0.0/8", "172.16.0.0/12"}, "8.8.8.8", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker, err := NewTrustedProxyChecker(tt.cidrs)
			if err != nil {
				t.Fatalf("NewTrustedProxyChecker() error = %v", err)
			}
			ip := netip.MustParseAddr(tt.ip)
			if got := checker.IsTrustedIP(ip); got != tt.expected {
				t.Errorf("IsTrustedIP(%q) = %v, want %v", tt.ip, got, tt.expected)
			}
		})
	}
}

func TestTrustedProxyChecker_IsTrustedRequest(t *testing.T) {
	tests := []struct {
		name       string
		cidrs      []string
		remoteAddr string
		expected   bool
	}{
		// Test with port in RemoteAddr (typical case)
		{"Trusted with port", []string{}, "127.0.0.1:12345", true},
		{"Untrusted with port", []string{"10.0.0.0/8"}, "8.8.8.8:443", false},
		{"Trusted matching CIDR with port", []string{"10.0.0.0/8"}, "10.1.2.3:8080", true},

		// Test without port (edge case)
		{"Trusted without port", []string{}, "127.0.0.1", true},
		{"Untrusted without port", []string{"10.0.0.0/8"}, "8.8.8.8", false},

		// IPv6 with port (brackets)
		{"IPv6 loopback with port", []string{}, "[::1]:8080", true},

		// Invalid RemoteAddr
		{"Invalid RemoteAddr", []string{}, "not-an-ip", false},
		{"Empty RemoteAddr", []string{}, "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker, err := NewTrustedProxyChecker(tt.cidrs)
			if err != nil {
				t.Fatalf("NewTrustedProxyChecker() error = %v", err)
			}
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tt.remoteAddr
			if got := checker.IsTrustedRequest(req); got != tt.expected {
				t.Errorf("IsTrustedRequest() with RemoteAddr=%q = %v, want %v", tt.remoteAddr, got, tt.expected)
			}
		})
	}
}

func TestRemoteIP(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		wantIP     string
		wantOK     bool
	}{
		{"IPv4 with port", "192.168.1.1:8080", "192.168.1.1", true},
		{"IPv4 without port", "192.168.1.1", "192.168.1.1", true},
		{"IPv6 with port", "[2001:db8::1]:443", "2001:db8::1", true},
		{"IPv6 without port", "2001:db8::1", "2001:db8::1", true},
		{"Loopback with port", "127.0.0.1:12345", "127.0.0.1", true},
		{"IPv6 loopback with port", "[::1]:8080", "::1", true},
		{"Invalid IP", "not-an-ip:8080", "", false},
		{"Empty string", "", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tt.remoteAddr
			ip, ok := remoteIP(req)
			if ok != tt.wantOK {
				t.Errorf("remoteIP() ok = %v, want %v", ok, tt.wantOK)
			}
			if ok && ip.String() != tt.wantIP {
				t.Errorf("remoteIP() = %v, want %v", ip.String(), tt.wantIP)
			}
		})
	}
}
