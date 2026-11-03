package auth

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"
)

func productionDeletionClient() *http.Client {
	client := deletionHTTPClient()
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil || len(ips) == 0 {
			return nil, fmt.Errorf("callback DNS unavailable")
		}
		for _, a := range ips {
			if !publicCallbackIP(a.IP) {
				return nil, fmt.Errorf("callback address forbidden")
			}
		}
		var dialErr error
		for _, a := range ips {
			connection, err := (&net.Dialer{Timeout: 2 * time.Second}).DialContext(ctx, network, net.JoinHostPort(a.IP.String(), port))
			if err == nil {
				return connection, nil
			}
			dialErr = err
		}
		return nil, dialErr
	}
	client.Transport = transport
	// Reject non-HTTPS even if the registration predates current validation.
	client.Transport = httpsCallbackTransport{transport}
	return client
}

type httpsCallbackTransport struct{ next http.RoundTripper }

func (t httpsCallbackTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.URL.Scheme != "https" || r.URL.User != nil {
		return nil, fmt.Errorf("HTTPS callback required")
	}
	return t.next.RoundTrip(r)
}
func publicCallbackIP(ip net.IP) bool {
	if !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() {
		return false
	}
	for _, cidr := range []string{"100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "2001:db8::/32"} {
		_, block, _ := net.ParseCIDR(cidr)
		if block.Contains(ip) {
			return false
		}
	}
	return true
}
