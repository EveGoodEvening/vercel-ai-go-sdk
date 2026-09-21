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

const (
	defaultBaseURL       = "https://ai-gateway.vercel.sh/v4/ai"
	defaultPublicBaseURL = "https://ai-gateway.vercel.sh/v1"
)

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
	publicBaseURL   string
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

// TokenSource supplies an OIDC bearer token once per HTTP attempt using the
// Evaluate context. Errors and blank tokens become a TransportError whose
// Operation is "resolve OIDC token"; no credential fallback occurs.
type TokenSource interface {
	Token(context.Context) (string, error)
}

// RetryPolicy configures bounded retries. MaxAttempts counts the initial
// request: zero and one mean one attempt, and 2 through 10 enable retries that
// may duplicate billable, non-idempotent evaluation work. With retries enabled,
// zero InitialDelay, MaxDelay, and Multiplier resolve to 100*time.Millisecond,
// 2*time.Second, and 2; zero Jitter disables jitter. Explicit InitialDelay must
// be 1ms..1m, MaxDelay 1ms..5m and at least InitialDelay, Multiplier 1..10,
// and Jitter 0..1. Delay before retry r (starting at 1) is the saturated
// min(MaxDelay, InitialDelay*Multiplier^(r-1)), with symmetric jitter applied
// as a fraction and rounded to a time.Duration nanosecond. Valid Retry-After
// replaces and is capped by MaxDelay. Waiting is context-cancellable.
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

// NewClient applies options in order and constructs a client without network
// calls. Credential precedence is explicit API key, AI_GATEWAY_API_KEY,
// explicit OIDC token/source, then VERCEL_OIDC_TOKEN. Blank environment values
// are absent. Missing credentials and invalid options return ConfigurationError.
func NewClient(opts ...Option) (*Client, error) {
	config := clientConfig{
		baseURL:       defaultBaseURL,
		publicBaseURL: defaultPublicBaseURL,
		httpClient:    http.DefaultClient,
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

// WithAPIKey selects an explicit API key, which has highest precedence. It
// rejects empty/all-Unicode-whitespace input and otherwise preserves it exactly.
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

// WithOIDCToken selects a fixed explicit OIDC token. It rejects blank input and
// otherwise preserves it exactly. The last explicit OIDC form wins, but neither
// explicit OIDC form displaces an explicit or environment API key.
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

// WithOIDCTokenSource selects a non-nil refresh-capable OIDC source. It rejects
// nil and typed-nil sources. The last explicit OIDC form wins, but neither form
// displaces an explicit or environment API key.
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

// WithBaseURL sets the provider base URL. It requires a whitespace-exact,
// absolute, non-opaque HTTP(S) URL with host and without userinfo, query, or
// fragment. Existing paths are allowed. Only trailing path slashes are removed;
// scheme, host, port, escaping, and the remaining path are preserved before
// Evaluate appends /evaluation-model.
func WithBaseURL(baseURL string) Option {
	return withBaseURL("WithBaseURL", baseURL, false, func(config *clientConfig, normalized string) {
		config.baseURL = normalized
	})
}

// WithPublicBaseURL sets the base URL for public Gateway APIs. It rejects the
// same invalid URLs as WithBaseURL plus any literal fragment delimiter, while
// preserving escaped path data. It does not affect the provider-protocol URL.
func WithPublicBaseURL(baseURL string) Option {
	return withBaseURL("WithPublicBaseURL", baseURL, true, func(config *clientConfig, normalized string) {
		config.publicBaseURL = normalized
	})
}

func withBaseURL(option, baseURL string, rejectFragmentDelimiter bool, set func(*clientConfig, string)) Option {
	return func(config *clientConfig) error {
		if blank(baseURL) || baseURL != strings.TrimSpace(baseURL) {
			return newConfigurationError(option, "must not be empty or whitespace")
		}
		parsed, err := url.Parse(baseURL)
		if err != nil || !parsed.IsAbs() || parsed.Opaque != "" || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			return newConfigurationError(option, "must be an absolute http or https URL")
		}
		if parsed.User != nil || parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" || (rejectFragmentDelimiter && strings.Contains(baseURL, "#")) {
			return newConfigurationError(option, "must not contain userinfo, query, or fragment")
		}
		parsed.Path = strings.TrimRight(parsed.Path, "/")
		parsed.RawPath = strings.TrimRight(parsed.RawPath, "/")
		set(config, parsed.String())
		return nil
	}
}

func (config *clientConfig) providerEndpoint(path string) string {
	return appendEndpoint(config.baseURL, path)
}

func (config *clientConfig) publicEndpoint(path string) string {
	return appendEndpoint(config.publicBaseURL, path)
}

func appendEndpoint(baseURL, path string) string {
	return baseURL + path
}

// WithHTTPClient rejects nil and stores the supplied pointer without cloning or
// mutating the client or transport. Subsequent concurrent mutation is the
// caller's responsibility under the same rules as http.Client.
func WithHTTPClient(client *http.Client) Option {
	return func(config *clientConfig) error {
		if client == nil {
			return newConfigurationError("WithHTTPClient", "must not be nil")
		}
		config.httpClient = client
		return nil
	}
}

// WithTeam sets a nonblank Vercel team ID or slug and preserves it exactly.
// There is no environment team fallback.
func WithTeam(teamIDOrSlug string) Option {
	return func(config *clientConfig) error {
		if blank(teamIDOrSlug) {
			return newConfigurationError("WithTeam", "must not be empty or whitespace")
		}
		config.team = teamIDOrSlug
		return nil
	}
}

// WithHeaders accepts nil or clones the supplied map when applied and again per
// request, preserving values, order, duplicates, and nil/empty slices. It
// rejects protocol/auth/team-owned names case-insensitively even without values:
// Authorization, Content-Type, Ai-Gateway-Protocol-Version,
// Ai-Gateway-Auth-Method, Ai-Evaluation-Model-Specification-Version,
// Ai-Model-Id, and X-Vercel-Ai-Gateway-Team.
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

// WithRetryPolicy copies, validates, resolves, and stores policy. Invalid fields
// return ConfigurationError with Option "WithRetryPolicy" and a stable Reason.
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
