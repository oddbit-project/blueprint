package s3

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"net/http"
)

// httpForceTransport is a custom RoundTripper that converts HTTPS requests to HTTP
type httpForceTransport struct {
	base http.RoundTripper
}

func (t *httpForceTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Force HTTP instead of HTTPS for MinIO
	if req.URL.Scheme == "https" {
		req.URL.Scheme = "http"
		// Also update the host if needed
		req.URL.Host = req.URL.Host // Keep the same host
	}
	return t.base.RoundTrip(req)
}

// customEndpointResolver implements s3.EndpointResolver for HTTP endpoints
type customEndpointResolver struct {
	endpointURL string
}

// ResolveEndpoint resolves the endpoint for HTTP MinIO connections
func (r *customEndpointResolver) ResolveEndpoint(region string, options s3.EndpointResolverOptions) (aws.Endpoint, error) {
	return aws.Endpoint{
		URL:               r.endpointURL,
		HostnameImmutable: true,
		SigningRegion:     region,
	}, nil
}
