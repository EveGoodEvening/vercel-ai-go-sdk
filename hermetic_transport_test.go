package gateway

import (
	"errors"
	"net"
	"net/http"
	"os"
	"strings"
	"testing"
)

type loopbackOnlyTransport struct {
	base http.RoundTripper
}

func (transport loopbackOnlyTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	if request == nil || request.URL == nil {
		return nil, errors.New("hermetic transport rejected request without URL")
	}
	host := request.URL.Hostname()
	allowed := strings.EqualFold(host, "localhost")
	if address := net.ParseIP(host); address != nil {
		allowed = address.IsLoopback()
	}
	if host == "" || !allowed {
		return nil, errors.New("hermetic transport rejected non-loopback destination")
	}
	base := transport.base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(request)
}

func TestMain(main *testing.M) {
	_ = os.Unsetenv(apiKeyEnvironment)
	_ = os.Unsetenv(oidcTokenEnvironment)
	_ = os.Unsetenv("AI_GATEWAY_LIVE_COST_ACK")
	original := http.DefaultTransport
	http.DefaultTransport = loopbackOnlyTransport{base: original}
	code := main.Run()
	http.DefaultTransport = original
	os.Exit(code)
}
