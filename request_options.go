package kibana

import "net/http"

// RequestOptionFunc can be passed to all API requests to customize the API request
type RequestOptionFunc func(*http.Request) error
