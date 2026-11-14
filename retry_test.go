package kibana_test

import (
	"context"
	"crypto/x509"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ottramst/go-kibana"
)

// retryServer answers with the given status codes in turn, then 200, and
// records the attempts.
type retryServer struct {
	mu       sync.Mutex
	statuses []int
	header   http.Header
	attempts int
	bodies   []string
}

func (s *retryServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.attempts++
	s.bodies = append(s.bodies, string(body))
	if len(s.statuses) == 0 {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"marketing","name":"Marketing"}`))
		return
	}
	status := s.statuses[0]
	s.statuses = s.statuses[1:]
	for k, v := range s.header {
		w.Header()[k] = v
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(`{"statusCode":` + strconv.Itoa(status) + `,"error":"x","message":"try again"}`))
}

// n returns the number of attempts.
func (s *retryServer) n() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.attempts
}

// requestBodies returns the request bodies of the attempts.
func (s *retryServer) requestBodies() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.bodies)
}

// reset sets the status codes to answer with.
func (s *retryServer) reset(statuses ...int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.statuses, s.attempts, s.bodies = statuses, 0, nil
}

// retryClient returns a client for srv that retries with policy and records
// the waits instead of sleeping.
func retryClient(t *testing.T, srv http.Handler, policy kibana.RetryPolicy) (*kibana.Client, *[]time.Duration) {
	t.Helper()
	server := httptest.NewServer(srv)
	t.Cleanup(server.Close)
	var waits []time.Duration
	sleep := func(ctx context.Context, d time.Duration) error {
		waits = append(waits, d)
		return ctx.Err()
	}
	client, err := kibana.NewClient(server.URL, kibana.WithRetry(policy), kibana.WithRetrySleep(sleep))
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	return client, &waits
}

func TestWithRetry_success(t *testing.T) {
	srv := &retryServer{statuses: []int{503, 502, 504}}
	client, waits := retryClient(t, srv, kibana.RetryPolicy{MinBackoff: time.Second, MaxBackoff: 3 * time.Second})
	space, _, err := client.Spaces.Get(context.Background(), "marketing", nil)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if space.Name != "Marketing" || srv.n() != 4 {
		t.Errorf("Get() = %+v after %d attempts, want Marketing after 4", space, srv.n())
	}
	// Retry n waits between MinBackoff and min(MaxBackoff, MinBackoff×2^(n-1)).
	ceilings := []time.Duration{time.Second, 2 * time.Second, 3 * time.Second}
	if len(*waits) != 3 {
		t.Fatalf("waits = %v, want 3", *waits)
	}
	for i, d := range *waits {
		if d < time.Second || d > ceilings[i] {
			t.Errorf("wait %d = %v, want between 1s and %v", i+1, d, ceilings[i])
		}
	}
}

func TestWithRetry_exhausted(t *testing.T) {
	srv := &retryServer{statuses: []int{503, 503, 503, 503, 503}}
	client, _ := retryClient(t, srv, kibana.RetryPolicy{})
	_, _, err := client.Spaces.Get(context.Background(), "marketing", nil)
	// The last response is returned with its body.
	var errResp *kibana.ErrorResponse
	if !errors.As(err, &errResp) || errResp.StatusCode != 503 || errResp.Message != "try again" {
		t.Errorf("Get() error = %v, want the last 503 with its message", err)
	}
	if srv.n() != 4 {
		t.Errorf("attempts = %d, want 4", srv.n())
	}
}

func TestWithRetry_noRetries(t *testing.T) {
	srv := &retryServer{statuses: []int{503}}
	client, _ := retryClient(t, srv, kibana.RetryPolicy{MaxAttempts: 1})
	if _, _, err := client.Spaces.Get(context.Background(), "marketing", nil); !errors.Is(err, kibana.ErrServiceUnavailable) {
		t.Errorf("Get() error = %v, want ErrServiceUnavailable", err)
	}
	if srv.n() != 1 {
		t.Errorf("attempts = %d, want 1", srv.n())
	}
}

func TestWithRetry_statusCodes(t *testing.T) {
	// 500 is not retried by default, but is with RetryOn.
	srv := &retryServer{statuses: []int{500}}
	client, _ := retryClient(t, srv, kibana.RetryPolicy{})
	if _, _, err := client.Spaces.Get(context.Background(), "marketing", nil); err == nil || srv.n() != 1 {
		t.Errorf("Get() error = %v after %d attempts, want an error after 1", err, srv.n())
	}
	srv = &retryServer{statuses: []int{500, 503}}
	client, _ = retryClient(t, srv, kibana.RetryPolicy{RetryOn: []int{500}})
	if _, _, err := client.Spaces.Get(context.Background(), "marketing", nil); !errors.Is(err, kibana.ErrServiceUnavailable) || srv.n() != 2 {
		t.Errorf("Get() error = %v after %d attempts, want ErrServiceUnavailable after 2", err, srv.n())
	}
}

func TestWithRetry_retryAfter(t *testing.T) {
	tests := []struct {
		name       string
		retryAfter string
		wantWait   time.Duration
	}{
		{"seconds", "2", 2 * time.Second},
		{"date", time.Now().Add(time.Hour).UTC().Format(http.TimeFormat), 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := &retryServer{statuses: []int{429}, header: http.Header{"Retry-After": {tt.retryAfter}}}
			client, waits := retryClient(t, srv, kibana.RetryPolicy{MaxBackoff: 2 * time.Hour})
			if _, _, err := client.Spaces.Get(context.Background(), "marketing", nil); err != nil {
				t.Fatalf("Get() error = %v", err)
			}
			if len(*waits) != 1 {
				t.Fatalf("waits = %v, want 1", *waits)
			}
			got := (*waits)[0]
			if tt.wantWait != 0 && got != tt.wantWait || tt.wantWait == 0 && (got < 59*time.Minute || got > time.Hour) {
				t.Errorf("wait = %v, want %s", got, tt.retryAfter)
			}
		})
	}
}

func TestWithRetry_retryAfterCapped(t *testing.T) {
	// Kibana's startup 503 asks for 30 seconds; the wait is capped at
	// MaxBackoff.
	srv := &retryServer{statuses: []int{503}, header: http.Header{"Retry-After": {"30"}}}
	client, waits := retryClient(t, srv, kibana.RetryPolicy{MaxBackoff: 10 * time.Second})
	if _, _, err := client.Spaces.Get(context.Background(), "marketing", nil); err != nil {
		t.Errorf("Get() error = %v, want a retry", err)
	}
	if srv.n() != 2 || len(*waits) != 1 || (*waits)[0] != 10*time.Second {
		t.Errorf("attempts = %d, waits = %v; want 2 attempts and a 10s wait", srv.n(), *waits)
	}
}

func TestWithRetry_post(t *testing.T) {
	retryAfter := http.Header{"Retry-After": {"1"}}
	tests := []struct {
		name         string
		status       int
		header       http.Header
		wantAttempts int
	}{
		// Kibana may have processed the request.
		{"502", 502, nil, 1},
		{"504", 504, nil, 1},
		{"503", 503, nil, 1},
		// Kibana has not processed the request.
		{"429", 429, nil, 2},
		{"503 with Retry-After", 503, retryAfter, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := &retryServer{statuses: []int{tt.status}, header: tt.header}
			client, _ := retryClient(t, srv, kibana.RetryPolicy{})
			_, _, _ = client.Spaces.Create(context.Background(), &kibana.Space{ID: "marketing", Name: "Marketing"}, nil)
			if srv.n() != tt.wantAttempts {
				t.Errorf("attempts = %d, want %d", srv.n(), tt.wantAttempts)
			}
			for i, body := range srv.requestBodies() {
				if !strings.Contains(body, `"id":"marketing"`) {
					t.Errorf("body %d = %q, want the space", i+1, body)
				}
			}
		})
	}
}

func TestWithRetry_rewindsBody(t *testing.T) {
	srv := &retryServer{statuses: []int{503, 503}}
	client, _ := retryClient(t, srv, kibana.RetryPolicy{})
	if _, _, err := client.Spaces.Update(context.Background(), "marketing", &kibana.Space{Name: "Marketing"}, nil); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	bodies := srv.requestBodies()
	if len(bodies) != 3 || bodies[0] == "" || bodies[1] != bodies[0] || bodies[2] != bodies[0] {
		t.Errorf("bodies = %q, want the same body 3 times", bodies)
	}
}

func TestWithRetry_streamedBody(t *testing.T) {
	// The import file is streamed, so it cannot be sent again.
	srv := &retryServer{statuses: []int{429}}
	client, _ := retryClient(t, srv, kibana.RetryPolicy{})
	_, _, err := client.SavedObjects.Import(context.Background(), strings.NewReader(`{"type":"tag","id":"t"}`+"\n"), nil)
	if !errors.Is(err, kibana.ErrTooManyRequests) || srv.n() != 1 {
		t.Errorf("Import() error = %v after %d attempts, want ErrTooManyRequests after 1", err, srv.n())
	}
}

func TestWithRetry_deadline(t *testing.T) {
	// A wait that would end after the deadline returns the last response.
	srv := &retryServer{statuses: []int{503, 503}}
	client, waits := retryClient(t, srv, kibana.RetryPolicy{MinBackoff: 10 * time.Second})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, _, err := client.Spaces.Get(ctx, "marketing", nil); !errors.Is(err, kibana.ErrServiceUnavailable) {
		t.Errorf("Get() error = %v, want ErrServiceUnavailable", err)
	}
	if srv.n() != 1 || len(*waits) != 0 {
		t.Errorf("attempts = %d, waits = %v; want 1 attempt and no wait", srv.n(), *waits)
	}
}

func TestWithRetry_httpClientTimeout(t *testing.T) {
	// The timeout of the HTTP client is a deadline for the transport too.
	srv := &retryServer{statuses: []int{503, 503}}
	server := httptest.NewServer(srv)
	t.Cleanup(server.Close)
	client, err := kibana.NewClient(server.URL,
		kibana.WithHTTPClient(&http.Client{Timeout: 5 * time.Second}),
		kibana.WithRetry(kibana.RetryPolicy{MinBackoff: 10 * time.Second}),
	)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	start := time.Now()
	if _, _, err := client.Spaces.Get(context.Background(), "marketing", nil); !errors.Is(err, kibana.ErrServiceUnavailable) {
		t.Errorf("Get() error = %v, want ErrServiceUnavailable", err)
	}
	if srv.n() != 1 || time.Since(start) > 4*time.Second {
		t.Errorf("attempts = %d after %v, want 1 without waiting", srv.n(), time.Since(start))
	}
}

func TestWithRetry_canceled(t *testing.T) {
	srv := &retryServer{statuses: []int{503, 503}}
	server := httptest.NewServer(srv)
	t.Cleanup(server.Close)
	ctx, cancel := context.WithCancel(context.Background())
	client, err := kibana.NewClient(server.URL,
		kibana.WithRetry(kibana.RetryPolicy{}),
		kibana.WithRetrySleep(func(ctx context.Context, _ time.Duration) error {
			cancel()
			<-ctx.Done()
			return ctx.Err()
		}),
	)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	if _, _, err := client.Spaces.Get(ctx, "marketing", nil); !errors.Is(err, context.Canceled) {
		t.Errorf("Get() error = %v, want context.Canceled", err)
	}
	if srv.n() != 1 {
		t.Errorf("attempts = %d, want 1", srv.n())
	}
}

func TestWithRetry_transportErrors(t *testing.T) {
	tests := []struct {
		name         string
		method       string
		err          error
		wantAttempts int
	}{
		{"GET", http.MethodGet, errors.New("connection reset"), 2},
		{"POST", http.MethodPost, errors.New("connection reset"), 1},
		{"certificate", http.MethodGet, x509.UnknownAuthorityError{}, 1},
		{"rejected certificate", http.MethodGet, &net.OpError{Op: "remote error", Err: errors.New("tls: bad certificate")}, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := &retryServer{}
			server := httptest.NewServer(srv)
			t.Cleanup(server.Close)
			attempts := 0
			transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
				attempts++
				if attempts == 1 {
					if r.Body != nil {
						_ = r.Body.Close()
					}
					return nil, tt.err
				}
				return http.DefaultTransport.RoundTrip(r)
			})
			client, err := kibana.NewClient(server.URL,
				kibana.WithHTTPClient(&http.Client{Transport: transport}),
				kibana.WithRetry(kibana.RetryPolicy{}),
				kibana.WithRetrySleep(func(context.Context, time.Duration) error { return nil }),
			)
			if err != nil {
				t.Fatalf("NewClient() error = %v", err)
			}
			req, err := client.NewRequest(context.Background(), tt.method, "/api/spaces/space/marketing", nil)
			if err != nil {
				t.Fatalf("NewRequest() error = %v", err)
			}
			_, err = client.Do(req, nil)
			if (err == nil) != (tt.wantAttempts > 1) || attempts != tt.wantAttempts {
				t.Errorf("Do() error = %v after %d attempts, want %d attempts", err, attempts, tt.wantAttempts)
			}
		})
	}
}

func TestWithRetry_httpClient(t *testing.T) {
	srv := &retryServer{statuses: []int{503}}
	server := httptest.NewServer(srv)
	t.Cleanup(server.Close)
	noSleep := kibana.WithRetrySleep(func(context.Context, time.Duration) error { return nil })
	for _, order := range []string{"WithRetry first", "WithHTTPClient first"} {
		t.Run(order, func(t *testing.T) {
			srv.reset(503)
			hc := &http.Client{}
			opts := []kibana.ClientOption{kibana.WithRetry(kibana.RetryPolicy{}), kibana.WithHTTPClient(hc), noSleep}
			if order == "WithHTTPClient first" {
				opts = []kibana.ClientOption{kibana.WithHTTPClient(hc), kibana.WithRetry(kibana.RetryPolicy{}), noSleep}
			}
			client, err := kibana.NewClient(server.URL, opts...)
			if err != nil {
				t.Fatalf("NewClient() error = %v", err)
			}
			if _, _, err := client.Spaces.Get(context.Background(), "marketing", nil); err != nil {
				t.Errorf("Get() error = %v, want a retry", err)
			}
			if hc.Transport != nil {
				t.Errorf("the caller's client was changed: Transport = %T", hc.Transport)
			}
		})
	}
}

func TestWithRetry_invalid(t *testing.T) {
	for name, opts := range map[string][]kibana.ClientOption{
		"twice":              {kibana.WithRetry(kibana.RetryPolicy{}), kibana.WithRetry(kibana.RetryPolicy{})},
		"negative attempts":  {kibana.WithRetry(kibana.RetryPolicy{MaxAttempts: -1})},
		"negative backoff":   {kibana.WithRetry(kibana.RetryPolicy{MinBackoff: -time.Second})},
		"min above max":      {kibana.WithRetry(kibana.RetryPolicy{MinBackoff: 2 * time.Second, MaxBackoff: time.Second})},
		"min above default":  {kibana.WithRetry(kibana.RetryPolicy{MinBackoff: time.Minute})},
		"success status":     {kibana.WithRetry(kibana.RetryPolicy{RetryOn: []int{200}})},
		"out of range state": {kibana.WithRetry(kibana.RetryPolicy{RetryOn: []int{600}})},
	} {
		if _, err := kibana.NewClient("http://localhost:5601", opts...); err == nil {
			t.Errorf("NewClient(%s) error = nil, want error", name)
		}
	}
}

func TestRetryAfter(t *testing.T) {
	future := time.Now().Add(time.Minute).UTC().Format(http.TimeFormat)
	past := time.Now().Add(-time.Minute).UTC().Format(http.TimeFormat)
	tests := []struct {
		value  string
		want   time.Duration
		wantOK bool
	}{
		{"", 0, false},
		{"5", 5 * time.Second, true},
		{"0", 0, true},
		{"-1", 0, false},
		{"soon", 0, false},
		{"99999999999999", 24 * time.Hour, true},
		{past, 0, true},
	}
	for _, tt := range tests {
		got, ok := kibana.RetryAfter(tt.value)
		if got != tt.want || ok != tt.wantOK {
			t.Errorf("retryAfter(%q) = %v, %v; want %v, %v", tt.value, got, ok, tt.want, tt.wantOK)
		}
	}
	if got, ok := kibana.RetryAfter(future); !ok || got < 58*time.Second || got > time.Minute {
		t.Errorf("retryAfter(%q) = %v, %v; want about a minute", future, got, ok)
	}
}

func TestWithRetry_realSleep(t *testing.T) {
	srv := &retryServer{statuses: []int{503}}
	server := httptest.NewServer(srv)
	t.Cleanup(server.Close)
	client, err := kibana.NewClient(server.URL, kibana.WithRetry(kibana.RetryPolicy{MinBackoff: time.Millisecond, MaxBackoff: time.Millisecond}))
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	if _, _, err := client.Spaces.Get(context.Background(), "marketing", nil); err != nil || srv.n() != 2 {
		t.Errorf("Get() error = %v after %d attempts, want success after 2", err, srv.n())
	}

	// Canceling the context ends the wait.
	srv.reset(503)
	client, err = kibana.NewClient(server.URL, kibana.WithRetry(kibana.RetryPolicy{MinBackoff: time.Minute, MaxBackoff: time.Minute}))
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(50*time.Millisecond, cancel)
	if _, _, err := client.Spaces.Get(ctx, "marketing", nil); !errors.Is(err, context.Canceled) || srv.n() != 1 {
		t.Errorf("Get() error = %v after %d attempts, want context.Canceled after 1", err, srv.n())
	}
}
