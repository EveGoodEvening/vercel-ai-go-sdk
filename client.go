package gateway

import (
	"context"
	"math"
	"math/rand/v2"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"time"
)

const defaultBaseURL = "https://ai-gateway.vercel.sh/v4/ai"

// Client is an evaluation-only Vercel AI Gateway client.
type Client struct {
	config clientConfig
}

type credentialKind uint8

const (
	credentialNone credentialKind = iota
	credentialAPIKey
	credentialOIDCToken
	credentialOIDCSource
)

type clientConfig struct {
	baseURL         string
	httpClient      *http.Client
	apiKey          string
	explicitAPIKey  bool
	oidcToken       string
	oidcTokenSource TokenSource
	explicitOIDC    credentialKind
	team            string
	headers         http.Header
	retryPolicy     RetryPolicy
	retryHooks      retryHooks
	credential      resolvedCredential
}

// Option configures a Client during NewClient. Options are applied in order and
// construction stops at the first error.
type Option func(*clientConfig) error

// TokenSource supplies an OIDC bearer token for an HTTP attempt.
type TokenSource interface {
	Token(context.Context) (string, error)
}

// RetryPolicy configures bounded retries. MaxAttempts counts the initial
// request; zero and one both mean one attempt. Values from two through ten opt
// in to retries, which may duplicate billable evaluation work. When retries are
// enabled, zero InitialDelay, MaxDelay, and Multiplier resolve to 100ms, 2s,
// and 2. Jitter is a symmetric fraction in [0,1].
type RetryPolicy struct {
	MaxAttempts  int
	InitialDelay time.Duration
	MaxDelay     time.Duration
	Multiplier   float64
	Jitter       float64
}

type retryHooks struct {
	now    func() time.Time
	sleep  func(context.Context, time.Duration) error
	jitter func() float64
}

func defaultRetryHooks() retryHooks {
	return retryHooks{
		now: time.Now,
		sleep: func(ctx context.Context, delay time.Duration) error {
			timer := time.NewTimer(delay)
			defer timer.Stop()
			select {
			case <-timer.C:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		},
		jitter: rand.Float64,
	}
}

// NewClient constructs a client without performing network calls.
func NewClient(opts ...Option) (*Client, error) {
	config := clientConfig{
		baseURL:    defaultBaseURL,
		httpClient: http.DefaultClient,
		retryPolicy: RetryPolicy{
			MaxAttempts: 1,
		},
		retryHooks: defaultRetryHooks(),
	}
	for _, option := range opts {
		if option == nil {
			continue
		}
		if err := option(&config); err != nil {
			return nil, err
		}
	}
	if err := resolveCredential(&config); err != nil {
		return nil, err
	}

	return &Client{config: config}, nil
}

// WithAPIKey selects an explicit API key.
func WithAPIKey(key string) Option {
	return func(config *clientConfig) error {
		if blank(key) {
			return newConfigurationError("WithAPIKey", "must not be empty or whitespace")
		}
		config.apiKey = key
		config.explicitAPIKey = true
		return nil
	}
}

// WithOIDCToken selects a fixed explicit OIDC token. Between explicit OIDC
// forms, the last option wins.
func WithOIDCToken(token string) Option {
	return func(config *clientConfig) error {
		if blank(token) {
			return newConfigurationError("WithOIDCToken", "must not be empty or whitespace")
		}
		config.oidcToken = token
		config.oidcTokenSource = nil
		config.explicitOIDC = credentialOIDCToken
		return nil
	}
}

// WithOIDCTokenSource selects a refresh-capable OIDC token source. Between
// explicit OIDC forms, the last option wins.
func WithOIDCTokenSource(source TokenSource) Option {
	return func(config *clientConfig) error {
		if nilInterface(source) {
			return newConfigurationError("WithOIDCTokenSource", "must not be nil")
		}
		config.oidcToken = ""
		config.oidcTokenSource = source
		config.explicitOIDC = credentialOIDCSource
		return nil
	}
}

// WithBaseURL sets the Evaluation Model V4 provider base URL.
func WithBaseURL(baseURL string) Option {
	return func(config *clientConfig) error {
		if blank(baseURL) || baseURL != strings.TrimSpace(baseURL) {
			return newConfigurationError("WithBaseURL", "must not be empty or whitespace")
		}
		parsed, err := url.Parse(baseURL)
		if err != nil || !parsed.IsAbs() || parsed.Opaque != "" || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			return newConfigurationError("WithBaseURL", "must be an absolute http or https URL")
		}
		if parsed.User != nil || parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" {
			return newConfigurationError("WithBaseURL", "must not contain userinfo, query, or fragment")
		}
		parsed.Path = strings.TrimRight(parsed.Path, "/")
		parsed.RawPath = strings.TrimRight(parsed.RawPath, "/")
		config.baseURL = parsed.String()
		return nil
	}
}

// WithHTTPClient stores and uses client without cloning or mutating it.
func WithHTTPClient(client *http.Client) Option {
	return func(config *clientConfig) error {
		if client == nil {
			return newConfigurationError("WithHTTPClient", "must not be nil")
		}
		config.httpClient = client
		return nil
	}
}

// WithTeam sets the Vercel team ID or slug.
func WithTeam(teamIDOrSlug string) Option {
	return func(config *clientConfig) error {
		if blank(teamIDOrSlug) {
			return newConfigurationError("WithTeam", "must not be empty or whitespace")
		}
		config.team = teamIDOrSlug
		return nil
	}
}

// WithHeaders clones caller-supplied headers and rejects names owned by the
// Gateway protocol, regardless of their casing or values.
func WithHeaders(headers http.Header) Option {
	return func(config *clientConfig) error {
		if containsProtectedHeader(headers) {
			return newConfigurationError("WithHeaders", "contains protected header")
		}
		if headers == nil {
			config.headers = nil
		} else {
			config.headers = headers.Clone()
		}
		return nil
	}
}

// WithRetryPolicy validates and stores a resolved retry policy.
func WithRetryPolicy(policy RetryPolicy) Option {
	return func(config *clientConfig) error {
		resolved, err := resolveRetryPolicy(policy)
		if err != nil {
			return err
		}
		config.retryPolicy = resolved
		return nil
	}
}

func withRetryHooks(hooks retryHooks) Option {
	return func(config *clientConfig) error {
		config.retryHooks = hooks
		return nil
	}
}

func resolveRetryPolicy(policy RetryPolicy) (RetryPolicy, error) {
	if policy.MaxAttempts < 0 || policy.MaxAttempts > 10 {
		return RetryPolicy{}, newConfigurationError("WithRetryPolicy", "invalid MaxAttempts")
	}
	if policy.MaxAttempts <= 1 {
		policy.MaxAttempts = 1
	}
	if policy.InitialDelay < 0 || policy.InitialDelay != 0 && (policy.InitialDelay < time.Millisecond || policy.InitialDelay > time.Minute) {
		return RetryPolicy{}, newConfigurationError("WithRetryPolicy", "invalid InitialDelay")
	}
	if policy.MaxDelay < 0 || policy.MaxDelay != 0 && (policy.MaxDelay < time.Millisecond || policy.MaxDelay > 5*time.Minute) {
		return RetryPolicy{}, newConfigurationError("WithRetryPolicy", "invalid MaxDelay")
	}
	if math.IsNaN(policy.Multiplier) || math.IsInf(policy.Multiplier, 0) || policy.Multiplier != 0 && (policy.Multiplier < 1 || policy.Multiplier > 10) {
		return RetryPolicy{}, newConfigurationError("WithRetryPolicy", "invalid Multiplier")
	}
	if math.IsNaN(policy.Jitter) || math.IsInf(policy.Jitter, 0) || policy.Jitter < 0 || policy.Jitter > 1 {
		return RetryPolicy{}, newConfigurationError("WithRetryPolicy", "invalid Jitter")
	}
	if policy.MaxAttempts > 1 {
		if policy.InitialDelay == 0 {
			policy.InitialDelay = 100 * time.Millisecond
		}
		if policy.MaxDelay == 0 {
			policy.MaxDelay = 2 * time.Second
		}
		if policy.Multiplier == 0 {
			policy.Multiplier = 2
		}
	}
	if policy.MaxDelay != 0 && policy.MaxDelay < policy.InitialDelay {
		return RetryPolicy{}, newConfigurationError("WithRetryPolicy", "MaxDelay is less than InitialDelay")
	}
	return policy, nil
}

func blank(value string) bool {
	return strings.TrimSpace(value) == ""
}

func nilInterface(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}
