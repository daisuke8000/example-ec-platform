package middleware

// PublicEndpointMatcher determines if an endpoint is publicly accessible.
type PublicEndpointMatcher struct {
	publicEndpoints map[string]struct{}
}

// NewPublicEndpointMatcher creates a new public endpoint matcher.
func NewPublicEndpointMatcher(endpoints []string) *PublicEndpointMatcher {
	endpointMap := make(map[string]struct{}, len(endpoints))
	for _, ep := range endpoints {
		endpointMap[ep] = struct{}{}
	}

	return &PublicEndpointMatcher{
		publicEndpoints: endpointMap,
	}
}

// IsPublic checks if the procedure is in the public whitelist.
// Uses exact string matching on gRPC full method name.
func (m *PublicEndpointMatcher) IsPublic(procedure string) bool {
	_, exists := m.publicEndpoints[procedure]
	return exists
}
