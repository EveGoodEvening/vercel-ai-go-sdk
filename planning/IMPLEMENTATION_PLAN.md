# Vercel AI Go SDK — Implementation Plan

## Status legend

- `[ ]` not started
- `[-]` in progress; the active branch/commit must name the chunk
- `[x]` implemented, reviewed, and accepted with the listed checks
- `[!]` blocked; the item must include the blocker, evidence, and the decision needed to unblock it

Only mark a standalone chunk complete after its code, tests, documentation changes, and acceptance checks are in the same reviewable completion boundary. An explicitly named encompassing feature item may use sequential focused commits for code subchunks, review fixes, and public documentation, but every subchunk remains incomplete until the encompassing boundary contains all required code, tests, docs, changelog/status accounting, and clean acceptance review; no committed state may be marked complete while its public documentation still says unsupported. Execute locally actionable chunks in order and do not combine unrelated work. A later independently approved feature chunk may begin when every earlier chunk is either complete, an explicitly incomplete subchunk of the same encompassing feature under the rule above, or locally complete with only explicitly named external release/publication gates remaining; those exceptions do not mark the incomplete or externally blocked chunk complete, clear any gate, or authorize release/publication work. Under this rule Chunks 01–23 and 25 are complete, Chunk 24 remains externally blocked, and the 2026-09-21 search-request continuation at the end of this plan defines the next dependency-ready sequence beginning with Chunk 26.

**Acyclic tracker-accounting rule:** every tracker-only accounting commit must record the exact full hashes, exact subjects, and exact changed paths of every landed predecessor commit it relies on. It must not claim to contain its own final hash: a commit cannot embed that hash because changing the tracker to add it changes the commit object and therefore the hash. Instead, the tracker-only commit records its exact intended subject and that its sole changed path is `planning/IMPLEMENTATION_PLAN.md`; the next dependent chunk must, before starting work, verify the landed commit's full hash, subject, and sole path from Git and record that commit as a predecessor in its own durable accounting. This handoff is the finite verification point and must never regress into another tracker-only commit whose sole purpose is to add its own hash. For the final Chunk 30 tracker-only closure, the same rule applies: it records every predecessor exactly plus its own intended subject and sole tracker path; its landed hash is verified by the first subsequent release/tag/accounting boundary before that boundary proceeds, not embedded in Chunk 30 itself.

## Goal

Build a small, maintainable Go port modeled specifically on Vercel's official `ai` / `@ai-sdk/gateway` **provider protocol for evaluation**, not a generic OpenAI-compatible wrapper. The initial SDK must call:

`POST https://ai-gateway.vercel.sh/v4/ai/evaluation-model`

It must represent the AI SDK Evaluation Model V4 contract: one JSON-compatible shared state, a nonempty keyed question map, boolean/choice/score questions, validated typed answers, warnings, usage, rounding, provider metadata, response metadata, authentication/header behavior, and typed Gateway errors.

The initial release is deliberately narrow. It must not claim parity with the large JavaScript `ai` package or all `@ai-sdk/gateway` modalities.

## Evidence baseline

Planning is pinned to the released upstream behavior observed on 2026-09-20:

- `ai@7.0.107`
- `@ai-sdk/gateway@4.0.87`
- `@ai-sdk/provider@4.0.17`
- `@ai-sdk/provider-utils@5.0.45`
- Evaluation entered core in `ai@7.0.103`; Gateway evaluation entered `@ai-sdk/gateway@4.0.85`.
- The provider base URL is `https://ai-gateway.vercel.sh/v4/ai`; evaluation posts to `/evaluation-model`.
- The separate public REST API `POST /v1/evaluate` is not the provider protocol and is out of scope.
- Evaluation is experimental and compatibility-sensitive.

Primary upstream references:

- <https://github.com/vercel/ai/blob/ai%407.0.107/packages/gateway/src/gateway-evaluation-model.ts>
- <https://github.com/vercel/ai/blob/ai%407.0.107/packages/gateway/src/gateway-provider.ts>
- <https://github.com/vercel/ai/blob/ai%407.0.107/packages/gateway/src/gateway-evaluation-model.test.ts>
- <https://github.com/vercel/ai/blob/ai%407.0.107/packages/ai/src/evaluate/validate-evaluation.ts>
- <https://github.com/vercel/ai/tree/ai%407.0.107/packages/provider/src/evaluation-model/v4>
- <https://vercel.com/docs/ai-gateway/modalities/evaluation>
- <https://go.dev/dl/> and <https://go.dev/doc/devel/release> for the maintained Go releases/support policy observed on 2026-09-20: Go 1.27.1 and Go 1.26.8.

## Initial supported surface

### Supported

- AI SDK Gateway provider protocol at `/v4/ai/evaluation-model` only.
- Arbitrary model IDs as strings, including the currently known `typesafe-ai/jev`; model IDs are not a closed enum.
- API-key and OIDC bearer authentication with explicit precedence.
- Optional team scope and caller-supplied headers, with protected protocol/auth headers owned by the SDK.
- Evaluation state that is any JSON-compatible string, object, or array.
- Boolean, choice, and score questions and their discriminated answer forms.
- Exact local validation of requests and responses where the Evaluation Model V4 contract defines invariants.
- Typed HTTP/Gateway errors with safe raw response retention and retry metadata.
- Caller-controlled cancellation through `context.Context`.
- Bounded retry support only where explicitly safe and configured.
- Hermetic tests using `httptest.Server`; separately gated, opt-in live contract checks.

### Explicit non-goals for the initial release

- No OpenAI-compatible `/v1/chat/completions` or `/v1/responses` wrapper.
- No public `/v1/evaluate` client.
- No language generation, text streaming, embeddings, images, video, reranking, speech, transcription, realtime, batches, credits, spend, generation lookup, model discovery, or gateway-executed search helpers.
- No `generateText`-style orchestration, agents, automatic tool execution, UI helpers, schema framework, or global default-provider registry.
- No full-parity claim with `ai` or `@ai-sdk/gateway`.
- No hard-coded complete model catalog.
- No automatic fallback from an explicitly configured but rejected API key to OIDC.
- No automatic retry of evaluation requests by default; retries may duplicate billable/non-idempotent work.
- No native `x_search` claim through Gateway until a live contract test proves an authenticated, documented wire contract.
- No direct xAI client in the initial Gateway module. Direct xAI Responses support is a separate future feature and package decision, not an alias for Gateway search.

Language generation and streaming are not justified by the evaluation-first goal. They remain excluded rather than adding a second protocol, event model, and error lifecycle before the evaluation contract is stable.

## Fixed architecture and API decisions

### Module and packages

- Module: `github.com/EveGoodEvening/vercel-ai-go-sdk`
- Root package: `gateway`; public evaluation client and all public contract types.
- Internal package: `internal/httpx`; bounded body reading, header parsing, Retry-After parsing, and shared HTTP response helpers. It must not contain modality policy.
- Internal package: `internal/testserver`; test-only request capture/fixture helpers, imported only by tests.
- Examples: `examples/evaluate/main.go`; runnable only with credentials and never invoked by hermetic tests.

Do not create one package per question type or mirror the JavaScript monorepo hierarchy. Go types and files within `gateway` are sufficient for the initial surface.

### Public client shape

The intended exported API is fixed for implementation review:

```go
package gateway

type Client struct { /* private */ }

type Option func(*clientConfig) error

func NewClient(opts ...Option) (*Client, error)
func WithAPIKey(key string) Option
func WithOIDCToken(token string) Option
func WithOIDCTokenSource(source TokenSource) Option
func WithBaseURL(baseURL string) Option
func WithHTTPClient(client *http.Client) Option
func WithTeam(teamIDOrSlug string) Option
func WithHeaders(headers http.Header) Option
func WithRetryPolicy(policy RetryPolicy) Option

type TokenSource interface {
    Token(context.Context) (string, error)
}

func (c *Client) Evaluate(ctx context.Context, modelID string, req EvaluationRequest) (*EvaluationResult, error)
```

Environment resolution performed by `NewClient`, in strict order:

1. Explicit `WithAPIKey`.
2. `AI_GATEWAY_API_KEY`.
3. Explicit `WithOIDCToken` or `WithOIDCTokenSource`.
4. `VERCEL_OIDC_TOKEN`.
5. Otherwise construction fails with a typed configuration error.

The environment API key wins even when an explicit OIDC token or source is configured. An explicit API key always wins, even if the server later rejects it. There is no request-time credential fallback.

Option acceptance and ownership are exact:

- `WithBaseURL` accepts only a nonempty URL string with no leading or trailing Unicode whitespace that parses as an absolute, non-opaque `http` or `https` URL with a nonempty host and no userinfo, query, or fragment. An existing path is allowed. Normalize only by removing every trailing `/` from the path; if that makes the path empty, retain the origin with no trailing slash. Preserve scheme, host, port, escaping, and the remaining path exactly, then append `/evaluation-model`. Do not trim or otherwise repair invalid input.
- `WithHTTPClient` rejects nil. The client stores and uses the supplied pointer; it does not clone or mutate the `http.Client` or its transport. Concurrent safety is therefore the caller's responsibility to the same extent as for `http.Client`.
- `WithAPIKey` and `WithOIDCToken` reject empty or all-Unicode-whitespace values and otherwise preserve the exact string, including leading/trailing whitespace in a nonblank value. `WithOIDCTokenSource` rejects a nil interface and any typed-nil source. Supplying both explicit OIDC forms is last-option-wins; neither can displace an explicit or environment API key under the precedence above. A selected source is called once per HTTP attempt with the Evaluate context; an error, empty token, or all-whitespace token is a request-time `TransportError` and never triggers fallback.
- Environment credential values use the same blank test: blank values are treated as absent; nonblank values are preserved exactly. Missing credentials after precedence resolution is the only credential-selection construction failure.
- `WithTeam` rejects empty or all-Unicode-whitespace values and otherwise preserves the exact string. There is no environment team fallback.
- `WithHeaders(nil)` is valid and means no caller headers. Non-nil input is deep-cloned when the option is applied using `http.Header.Clone`: original key spelling, slice lengths, value order, duplicate values, nil/empty slices, and empty strings are preserved. Later caller mutation cannot affect the client. Each request clones the stored header again, so request construction never mutates stored or caller maps.
- Protected names are matched case-insensitively after `http.CanonicalHeaderKey`: `Authorization`, `Content-Type`, `Ai-Gateway-Protocol-Version`, `Ai-Gateway-Auth-Method`, `Ai-Evaluation-Model-Specification-Version`, `Ai-Model-Id`, and `X-Vercel-Ai-Gateway-Team`. Merely containing a protected key is invalid even when its value slice is nil or empty. Caller headers otherwise preserve all values and are added before the SDK writes its single owned value for every protocol header.

Options are applied in call order and stop at the first error. Every accepted option is copied/stored during `NewClient`; no option retains a caller-owned map or string buffer. `WithRetryPolicy` copies the value struct and follows the fixed normalization below.

### Evaluation type decisions

Use a closed interface implemented by concrete question types so invalid cross-type fields cannot be represented accidentally:

```go
type EvaluationRequest struct {
    State           any
    Questions       map[string]Question
    ProviderOptions map[string]map[string]any
}

type Question interface { questionType() string }
type BooleanQuestion struct {
    Instructions any
    Criteria     *BooleanCriteria
}
type ChoiceQuestion struct {
    Instructions any
    Criteria     map[string]any
}
type ScoreQuestion struct {
    Instructions any
    Criteria     []any
}

// OptionalJSON distinguishes an omitted value from an explicitly present JSON null.
// Set=false omits the containing key and requires Value=nil; Set=true emits Value,
// including nil as JSON null.
type OptionalJSON struct {
    Set   bool
    Value any
}

type BooleanCriteria struct {
    True  OptionalJSON
    False OptionalJSON
}

type EvaluationResult struct {
    Answers          map[string]Answer
    Rounding         *Rounding
    Usage            *Usage
    Warnings         []Warning
    ProviderMetadata map[string]map[string]any
    Response          ResponseMetadata
}

// Rounding reports decimal places used by the provider. A nil field means the
// corresponding wire key was absent; a non-nil pointer may contain zero.
type Rounding struct {
    ProbabilityDecimals *int
    ScoreDecimals       *int
}

// Usage reports token counts. A nil field means the corresponding wire key was
// absent; a non-nil pointer may contain zero. Present counts must be finite,
// non-negative integers representable as int64.
type Usage struct {
    InputTokens  *int64
    OutputTokens *int64
}

type WarningType string

const (
    WarningUnsupported   WarningType = "unsupported"
    WarningCompatibility WarningType = "compatibility"
    WarningDeprecated    WarningType = "deprecated"
    WarningOther         WarningType = "other"
)

// Warning is the lossless public form of one wire warning. Unsupported and
// compatibility warnings require Feature and may set Details; deprecated
// warnings require Setting and Message; other warnings require Message.
// Nil Details distinguishes an omitted details key from an explicitly empty string.
type Warning struct {
    Type    WarningType
    Feature string
    Details *string
    Setting string
    Message string
}

// ResponseMetadata describes the successful HTTP response. ModelID is always
// the caller-supplied model ID, Headers is a clone of the received headers, and
// Body is a defensive copy of the raw successful-response bytes before JSON
// decoding, bounded by the package's fixed 1 MiB response-body limit. Headers
// and Body are non-nil on a successfully read response, including an empty
// body; their zero values mean no response metadata.
type ResponseMetadata struct {
    ModelID string
    Headers http.Header
    Body    []byte
}

type Answer interface { answerType() string }
type BooleanAnswer struct { Probability float64 }
type ChoiceAnswer struct { Choice string; Probabilities map[string]float64 }
type ScoreAnswer struct { Score float64; Probabilities map[string]float64 }

Implementation may use private wire DTOs and custom marshal/unmarshal code. Public data must not expose raw discriminator bookkeeping. Every question requires `Instructions`; `State`, `Instructions`, and every non-null criteria description must be a JSON-compatible string, object, or array. General JSON-compatible values may contain null, booleans, finite numbers, strings, arrays, and string-keyed objects. `ProviderOptions` uses an outer provider-name map whose values must each be a non-nil JSON object; its nested values follow the general JSON rules. All recursive checks happen before network I/O and reject unsupported values, non-string map keys, cycles, NaN, and infinities.

`BooleanQuestion.Criteria == nil` omits `criteria`. A non-nil zero-value `BooleanCriteria{}` emits `{}`. `OptionalJSON{Set:false, Value:nil}` omits that key; `OptionalJSON{Set:true, Value:nil}` emits explicit JSON null; `Set:false` with non-nil `Value` is invalid rather than silently discarded. This presence-bearing API is the sole public representation of boolean criteria presence.

The response wire mapping is fixed. `rounding.probabilityDecimals` and `rounding.scoreDecimals` are optional JSON integer numbers; an absent `rounding` object yields `nil`, while a present empty object yields `&Rounding{}`. `usage.inputTokens` and `usage.outputTokens` are optional JSON integer numbers with the same absent-versus-present-object behavior. `warnings` is optional on the wire and normalizes only absence to a non-nil empty slice; an explicit JSON `null` or malformed warning is invalid. `providerMetadata` is optional and must be an outer string-keyed object whose values are non-null JSON objects; absence yields `nil`, while `{}` remains a non-nil empty map. Warning fields not permitted for their discriminator are rejected rather than discarded. All HTTP bodies share one fixed unexported `1<<20` byte (1 MiB) retained-body limit and are read through a limit-plus-one detector; successful-body overflow or read failure is a `TransportError`, while error-body overflow retains the first 1 MiB and sets `BodyTruncated`. The SDK clones headers and raw bytes when constructing `ResponseMetadata`; because its fields are exported, callers that retain a result must treat their own subsequent mutations as local and must sanitize `Body` before logging or persistence because it can contain echoed state or provider options.

Choice and score answer probability presence is exact: the `probabilities` wire key is optional. If absent, the exported `Probabilities` field is `nil` and distribution/max/weighted-mean checks that require probabilities are skipped. If present, its value must be a non-null JSON object; JSON `null` is invalid, `{}` is invalid, and the object must contain exactly the complete expected key set before sum, maximum, or weighted-mean validation. Decoding never normalizes absent, null, or empty maps into one another. These rules are identical for choice and score answers.

Validation rules:

- Questions are nonempty, IDs are nonempty, and every question has valid required `Instructions`.
- Choice criteria contain at least one option. Returned choice must be known. When the `probabilities` key is present, its map must satisfy the fixed presence/completeness rules above and no other probability may exceed the selected probability by more than the fixed base tolerance `1e-6`; when absent, `Probabilities` is nil and the maximum check is skipped.
- Score criteria contain at least two ordered levels. Returned score is in `[0,n-1]`. When the `probabilities` key is present, its map must satisfy the fixed presence/completeness rules above and the score must equal the probability-weighted mean within the exact tolerance below; when absent, `Probabilities` is nil and the weighted-mean check is skipped.
- Boolean probability is finite and in `[0,1]` and means `P(true)`.
- Every present probability map is complete and contains only expected keys; values are finite in `[0,1]`. An absent map is permitted only for choice and score answers under the rule above; explicit null and empty objects are invalid.
- Let `baseTolerance = 1e-6`. For a declared decimal count `d`, `roundingError(d) = 0.5 * 10^(-d)`; when the count is absent, its rounding error is zero. Decimal counts must be integers in `[0,15]`.
- For a distribution with `k` expected keys, accept its sum iff `abs(sum-1) <= baseTolerance + k*probabilityError`; reject only when strictly greater.
- For score keys `0..n-1`, let `mean = Σ(i*p_i)` and `meanRoundingError = Σ(i*probabilityError)`. Accept iff `abs(mean-score) <= baseTolerance + meanRoundingError + scoreError`; reject only when strictly greater. Missing rounding metadata therefore uses exactly `1e-6` for both sum and mean comparisons.
- Response contains exactly one answer for every question and no unknown answer IDs; answer types match question types.
- Malformed provider output is returned as a response-validation error; it is never normalized silently.

`ValidationError.Path()` uses one canonical grammar for all request validation. The root is `$`. Object/map members append `[` plus a JSON-quoted UTF-8 key plus `]`, using Go `strconv.Quote` semantics (therefore quotes, backslashes, controls, and non-printable code points are escaped deterministically); array/slice elements append `[<base-10 index>]` with no leading zero. Examples are `$["state"]`, `$["questions"]["quality.score"]`, `$["criteria"][0]`, and `$["providerOptions"]["acme"]["a\\\"b"]`. A failure about a container as a whole uses that container's path; a bad map value or array element uses the offending child path. A cycle is reported at the child path whose edge first points to a value already on the active recursion stack, not at the earlier value. Traversal order is struct-defined fields first and otherwise lexicographically sorted string keys; arrays use ascending indexes, so the first returned error is deterministic. Paths describe the public request shape, never private DTO fields.

`ValidationError.Reason()` is one of these exact stable strings and no other value: `required`, `must be nonempty`, `must be a string, object, or array`, `must be JSON-compatible`, `map keys must be strings`, `number must be finite`, `cycle detected`, `must contain at least one option`, `must contain at least two levels`, `must be a non-nil question`, `question type is unsupported`, `must be omitted when Set is false`, or `must be a non-nil object`. Use `required` for absent/nil required values (including instructions); `must be nonempty` for empty model/question IDs or the empty questions map; `must be a string, object, or array` for a JSON-compatible scalar/null where the input-only restricted shape is required; `must be JSON-compatible` for unsupported Go values such as funcs/channels/complex values; `map keys must be strings` at the offending map container; `number must be finite` at NaN/Inf; `cycle detected` under the rule above; the two cardinality reasons at their criteria containers; `must be a non-nil question` at a nil question entry; `question type is unsupported` at an unknown implementation; `must be omitted when Set is false` at the offending `OptionalJSON` member; and `must be a non-nil object` at a nil provider object. Exact compatibility applies to `Path` and `Reason`; human-readable `Error()` prose is not a compatibility surface.

`ResponseValidationError.Path()` uses the same `$` plus JSON-quoted member/index grammar as `ValidationError`; paths name wire-response locations, not Go fields. The fixed top-level response members are `$["answers"]`, `$["rounding"]`, `$["usage"]`, `$["warnings"]`, and `$["providerMetadata"]`. Answer failures use `$["answers"][<question ID>]`; discriminator/value fields append `["type"]`, `["probability"]`, `["choice"]`, `["score"]`, or `["probabilities"]`, and probability entries append their choice/decimal score key. Rounding uses `["probabilityDecimals"]`/`["scoreDecimals"]`; usage uses `["inputTokens"]`/`["outputTokens"]`; warning fields use `$["warnings"][<index>][<field>]`; provider metadata uses `$["providerMetadata"][<provider>]` and then its nested JSON member/index path. A missing required member or missing expected answer/probability key uses the path where that member should have appeared. An extra/unknown/forbidden member uses its actual child path. A whole-container invariant uses the container path. JSON syntax errors and trailing JSON values use `$`; duplicate keys use the path of the duplicated key in the innermost object being decoded.

`ResponseValidationError.Reason()` is one of these exact stable strings and no other value: `malformed JSON`, `trailing JSON value`, `duplicate field`, `unknown field`, `required`, `must be an object`, `must be an array`, `must be a string`, `must be a number`, `must be an integer`, `must be non-null`, `must be nonempty`, `number must be finite`, `must be between 0 and 1`, `must be between 0 and 15`, `answer is missing`, `answer is unexpected`, `answer type does not match question`, `answer type is unsupported`, `choice is not in criteria`, `probability key is missing`, `probability key is unexpected`, `probabilities must sum to 1 within tolerance`, `selected choice is not maximal`, `score is out of range`, `score does not match weighted mean`, `warning type is unsupported`, `field is forbidden for warning type`, or `must be a non-nil object`. Use the structural reasons before semantic validation: null uses `must be non-null` except a provider-metadata provider value uses `must be a non-nil object`; a wrong JSON kind uses the corresponding `must be ...`; absent required wire fields use `required`. Empty present probability objects use `must be nonempty`. Missing/extra answer IDs and probability keys use their dedicated reasons. Integer range, probability range, score range, distribution, maximum, and weighted-mean failures use the dedicated reasons above.

Response JSON decoding is strict and finite. `Evaluate(nil, ...)` returns `ValidationError` at `$["context"]` with `required` before credential-source or transport work. Only HTTP status `200` is successful; every other status, including other `2xx` values, is a `ResponseError`. Response `Content-Type` is not enforced because intermediaries may omit or rewrite it. A status-200 body must contain exactly one JSON value followed only by JSON whitespace. Duplicate object keys are rejected throughout the entire response, including answer/probability/provider maps and recursively nested provider metadata. Unknown fields are rejected at every fixed DTO level: top-level result, each answer variant, `rounding`, `usage`, and each warning variant. The only intentionally open objects are the answer map, probability maps, provider-name map, and recursively JSON-compatible provider metadata values; openness does not waive duplicate-key, key-set, or value invariants. No decoder silently accepts a second value, duplicate key, or forward-compatible unknown fixed field.

### Provider wire contract

Every evaluation request uses:

- `POST {baseURL}/evaluation-model`
- Default base URL `https://ai-gateway.vercel.sh/v4/ai`, normalized without a trailing slash.
- `Authorization: Bearer <credential>`
- `Content-Type: application/json`
- `ai-gateway-protocol-version: 0.0.1`
- `ai-gateway-auth-method: api-key` or `oidc`
- `ai-evaluation-model-specification-version: 4`
- `ai-model-id: <modelID>`
- Optional `x-vercel-ai-gateway-team`.

The JSON body contains `state`, `questions`, and `providerOptions` only. It does not contain `model`. Preserve an explicitly supplied empty provider-options object; omit provider options only when nil. User headers may add values but may not override Authorization, content type, model/spec/protocol/auth-method, or team headers.

### Errors and retries

The exported retry API is fixed:

```go
type RetryPolicy struct {
    MaxAttempts  int
    InitialDelay time.Duration
    MaxDelay     time.Duration
    Multiplier   float64
    Jitter       float64
}
```

`MaxAttempts` counts the initial request. `0` and `1` both mean exactly one attempt and no retry; retries occur only for values `2..10`. When retries are enabled, zero `InitialDelay`, `MaxDelay`, or `Multiplier` resolve respectively to `100*time.Millisecond`, `2*time.Second`, and `2`; `Jitter==0` means no jitter. Explicit values require `InitialDelay` in `[time.Millisecond, time.Minute]`, `MaxDelay` in `[time.Millisecond, 5*time.Minute]` and not below the resolved initial delay, `Multiplier` in `[1,10]`, and `Jitter` in `[0,1]`; negative durations, negative attempts, attempts above 10, NaN/infinite floats, and nonzero values outside those ranges make `NewClient` return a `ConfigurationError` from applying `WithRetryPolicy`. Backoff before retry number `r` (where the first retry is `r=1`) is `min(MaxDelay, InitialDelay*Multiplier^(r-1))`, computed with saturation rather than overflowing. If jitter is nonzero and the private jitter hook returns `u` in `[0,1)`, the delay becomes `delay * (1 + Jitter*(2*u-1))`, rounded to the nearest `time.Duration` nanosecond and capped again at `MaxDelay`. A valid `Retry-After` replaces the computed/jittered backoff and is capped at `MaxDelay`; malformed or negative `Retry-After` is treated as absent. All durations are Go `time.Duration`. Waiting is context-cancellable.

Retry test control is private: `clientConfig` contains an unexported `retryHooks` value with `now func() time.Time`, `sleep func(context.Context, time.Duration) error`, and `jitter func() float64`; production defaults are `time.Now`, a timer-based context-aware sleeper, and a concurrency-safe uniform source in `[0,1)`. Package-internal tests may install hooks through an unexported `withRetryHooks(retryHooks) Option`; tests may return `0` or the largest representable value below `1` to cover the jitter bounds. No clock, sleeper, random source, or hook is exported.

Export this exact error hierarchy, compatible with `errors.As`; all data fields are private so zero/safety semantics cannot be bypassed:

```go
type ConfigurationError struct { /* private */ }
func (e *ConfigurationError) Error() string
func (e *ConfigurationError) Option() string
func (e *ConfigurationError) Reason() string

type ValidationError struct { /* private */ }
func (e *ValidationError) Error() string
func (e *ValidationError) Path() string
func (e *ValidationError) Reason() string

type TransportError struct { /* private */ }
func (e *TransportError) Error() string
func (e *TransportError) Operation() string
func (e *TransportError) Unwrap() error

type ResponseError struct { /* private */ }
func (e *ResponseError) Error() string
func (e *ResponseError) Unwrap() error
func (e *ResponseError) StatusCode() int
func (e *ResponseError) Message() string
func (e *ResponseError) Type() string
func (e *ResponseError) Code() string
func (e *ResponseError) Param() any
func (e *ResponseError) GenerationID() string
func (e *ResponseError) RequestID() string
func (e *ResponseError) ResponseID() string
func (e *ResponseError) RetryAfter() (time.Duration, bool)
func (e *ResponseError) Retryable() bool
func (e *ResponseError) BodyTruncated() bool
func (e *ResponseError) RawResponseBody() []byte

type ResponseValidationError struct { /* private */ }
func (e *ResponseValidationError) Error() string
func (e *ResponseValidationError) Unwrap() error
func (e *ResponseValidationError) StatusCode() int
func (e *ResponseValidationError) Path() string
func (e *ResponseValidationError) Reason() string
func (e *ResponseValidationError) RequestID() string
func (e *ResponseValidationError) ResponseID() string
func (e *ResponseValidationError) BodyTruncated() bool
func (e *ResponseValidationError) RawResponseBody() []byte
```

`ConfigurationError` replaces the previously provisional name `ConfigError`; no alias is provided. `Option`, `Path`, `Reason`, `Operation`, envelope strings, and IDs return `""` when unavailable. A string wire `error.code` is returned unchanged; a numeric code is returned in its original JSON lexical form; absent or null code returns `""`. `Param` returns nil for absent/null and otherwise a recursively defensive-copied JSON-compatible value on every call. The known error envelope is `{ "error": { "message", "type", "code", "param" }, "generationId", "requestId", "responseId" }`; IDs are read only from those top-level JSON string fields, so missing or mistyped IDs remain empty and no undocumented header is reinterpreted as an ID. `StatusCode` returns `0` before an HTTP response exists; `ResponseError` always has a positive HTTP status, and `ResponseValidationError` always has status 200. `RetryAfter` returns `(0,false)` when absent or invalid and `(0,true)` only for a valid header whose computed remaining delay is zero; `Retryable` is the final status/type classification and is false for a zero value. `BodyTruncated` is false unless bytes were actually omitted by the fixed 1 MiB limit. `RawResponseBody` returns `nil` when no body was retained and otherwise a fresh defensive copy on every call. `Unwrap` returns the underlying cause or nil; configuration and validation errors do not wrap causes. Nil receivers for every accessor return the documented zero value, `Unwrap` returns nil, `Param` and `RawResponseBody` return nil, and `Error` returns the stable type phrase (`"gateway configuration error"`, `"gateway validation error"`, `"gateway transport error"`, `"gateway response error"`, or `"gateway response validation error"`) without panicking.

`ConfigurationError.Option()` has this closed value set: `WithBaseURL`, `WithHTTPClient`, `WithAPIKey`, `WithOIDCToken`, `WithOIDCTokenSource`, `WithTeam`, `WithHeaders`, `WithRetryPolicy`, or `credentials`. `ConfigurationError.Reason()` has this closed value set: `must not be empty or whitespace`, `must be an absolute http or https URL`, `must not contain userinfo, query, or fragment`, `must not be nil`, `contains protected header`, `invalid MaxAttempts`, `invalid InitialDelay`, `invalid MaxDelay`, `MaxDelay is less than InitialDelay`, `invalid Multiplier`, `invalid Jitter`, or `no credential configured`. URL parse failures, relative/opaque URLs, missing hosts, and non-HTTP(S) schemes map to `WithBaseURL`/`must be an absolute http or https URL`; userinfo/query/fragment maps to the other base-URL reason. Blank explicit credentials/team and blank base URL map to their option and `must not be empty or whitespace`. Nil HTTP clients and nil/typed-nil token sources map to their option and `must not be nil`. Any protected header maps to `WithHeaders`/`contains protected header`. Retry fields map to the named field reason, with the cross-field comparison using `MaxDelay is less than InitialDelay`. Exhausted credential resolution maps to `credentials`/`no credential configured`. No other pair is emitted.

`TransportError.Operation()` has this closed value set: `resolve OIDC token`, `encode request`, `create request`, `send request`, or `read response body`. A selected token source error or blank returned token maps to `resolve OIDC token`; JSON marshaling after successful validation maps to `encode request`; `http.NewRequestWithContext` failure maps to `create request`; and `http.Client.Do`, including context cancellation/deadline while sending or awaiting headers, maps to `send request`. The Chunk 04 raw helper captures response status and headers before reading the body and returns them together with any bounded-body read, overflow, or close error. Public `Client.Evaluate` maps such a failure for status 200 to `TransportError("read response body")`; for every non-200 status, Chunk 06 instead returns a `ResponseError` retaining that status and wrapping a `TransportError("read response body")` as its cause. The underlying source/context/HTTP/body error remains reachable through `Unwrap`; a blank source token wraps a private sentinel rather than a configuration error. No other operation string is emitted.

`Error()` and any `fmt.Formatter` implementation must never include raw response bytes, authorization values, caller-header values, state, provider options, or other credential material. The diagnostic Go documentation on both `RawResponseBody` methods must state that an upstream or intermediary may have echoed sensitive request state or provider options and that callers must sanitize before logging or persistence. Structured safe fields may appear in error strings. Tests must treat `%s`, `%v`, `%+v`, `%q`, nil receivers, and wrapped-error formatting as public disclosure surfaces.

Retry classification is the exact status-only predicate `status == 408 || status == 409 || status == 429 || (status >= 500 && status <= 599)`. Parsed or malformed `error.type` and `error.code` never add, remove, or override retryability. `ResponseError.Retryable()` returns exactly that predicate for its HTTP status, including when the body is empty, malformed, truncated, or unreadable; a nil/zero-value error returns false. `Evaluate` retries a `ResponseError` if and only if `Retryable()` is true, the resolved policy has another attempt, the response body was read successfully, and the context remains active. A non-200 `ResponseError` that wraps a body read/overflow/close `TransportError` is therefore never retried even when its preserved status is otherwise retryable. Thus all other statuses, including 400, 401, 402, 403, and 404, are non-retryable. Honor both Retry-After seconds and HTTP dates using the injected `now` hook, but Retry-After never makes a non-retryable status retryable. Default maximum attempts is one. A caller opting into retries accepts duplicate evaluation risk; context cancellation stops waits and attempts immediately.
## Assumptions and risks

1. **Experimental upstream protocol:** patch releases may change Evaluation Model V4. Pin fixture evidence to upstream versions and record protocol changes before adapting.
2. **No live request evidence yet:** source/tests establish the shape, but an authenticated live smoke is required before release candidate tagging.
3. **Authentication environment:** OIDC token refresh semantics vary. The token-source interface prevents freezing an expiring token into a long-lived client.
4. **Arbitrary JSON in Go:** `any` is ergonomic but needs strict recursive validation and deterministic error paths.
5. **Floating-point probabilities:** use the fixed upstream-compatible `1e-6` base tolerance plus declared half-unit-in-last-place errors exactly as specified above; boundary comparisons are inclusive and provider values are never renormalized.
6. **Error-body drift and sensitivity:** parse known fields and retain bounded raw bytes for explicit diagnostics, but never include them in error formatting because providers or intermediaries may echo sensitive request data.
7. **Model catalog drift:** accept arbitrary nonempty model IDs. Examples may use `typesafe-ai/jev`, but the API must not imply it is the only model.
8. **Retry billing/duplication:** evaluation is not known to be idempotent. Zero retries is the safe default.
9. **Repository publication:** local `origin` now matches the module repository at `git@github.com:EveGoodEvening/vercel-ai-go-sdk.git` for both fetch and push. Hosted repository existence/metadata, owner-selected license, hosted CI/provenance, and release history remain independently unverified release prerequisites; the matching local remote does not clear them.
10. **`x_search`:** direct xAI Responses and `@ai-sdk/xai` support it, but Gateway support is unconfirmed. Do not infer support from a model's generic `tools` capability.

## Dependency-ordered implementation checklist

### Chunk 01 — Module and public contract skeleton

**Commit boundary:** `chore: initialize evaluation SDK module`

- [x] Create `go.mod` with module `github.com/EveGoodEvening/vercel-ai-go-sdk` and `go 1.26`, meaning an exact minimum language/toolchain compatibility floor of Go 1.26.0, and document that floor in `README.md`. The initial support window is the maintained Go 1.26 and 1.27 families; CI is pinned to Go 1.26.8 and Go 1.27.1 as observed on 2026-09-20.
- [x] Create `doc.go` declaring package `gateway` and its evaluation-only scope.
- [x] Create `client.go` with `Client`, private `clientConfig`, `Option`, `NewClient`, option declarations, the exact exported `RetryPolicy`, its `WithRetryPolicy` validation/default resolution, and private `retryHooks`/`withRetryHooks`; options may validate/store configuration but must not perform network calls.
- [x] Create `evaluation.go` with every public request/result/question/answer/metadata declaration fixed above, including warning constants, pointer optionality, nested provider metadata, and successful-response copy/zero semantics.
- [x] Create `errors.go` with exactly `ConfigurationError`, `ValidationError`, `TransportError`, `ResponseError`, and `ResponseValidationError` plus every fixed accessor, nil-receiver zero behavior, safe `Error`, and specified `Unwrap` behavior, without HTTP envelope decoding yet. Do not add `ConfigError`, exported fields, constructors, aliases, or alternate accessors.
- [x] Create `README.md` with an explicit supported/non-goal matrix and experimental status; do not publish a usage example that cannot run yet.
- [x] Add license only after repository ownership selects it; until then add a release blocker in the README rather than guessing.

**Acceptance checks:**

- [x] `go test ./...` compiles the public skeleton without network access.
- [x] `go vet ./...` succeeds.
- [x] `go doc .` shows only the intended evaluation-first skeleton surface and no parity claim; it is not required to show `Client.Evaluate` before Chunk 06.
- [x] A package-external compile test locks the exported declarations and method signatures available in this skeleton commit for metadata, retry policy, options, and all five error types; `Client.Evaluate` is added and compile-locked in Chunk 06 once both its success and non-200 paths are complete. A package-internal test locks nil/zero accessor behavior, the closed `ConfigurationError` and `TransportError` accessor value sets, and confirms the retry hooks remain unexported.
- [x] Independent review after fixes was clean and confirms no `/v1` OpenAI client, language API, or tool executor was introduced.

### Chunk 02 — JSON-compatible input model and request validation

**Commit boundary:** `feat: validate evaluation requests`

- [x] Create `jsonvalue.go` implementing cycle-safe recursive JSON compatibility checks with the exact `$`/JSON-quoted-key/index path grammar, deterministic traversal, cycle-edge rule, and canonical reason vocabulary fixed above; include a distinct input validator that accepts only JSON-compatible strings, objects, or arrays.
- [x] Create `evaluation_validate.go` implementing model ID, state, required question instructions, question ID/criteria, `OptionalJSON` invariants, and provider-options validation; reject nil/non-object outer provider values before transport.
- [x] Add custom/private wire encoding in `evaluation_wire.go` for discriminated question JSON and the specified boolean criteria omission/empty-object/explicit-null behavior.
- [x] Add `evaluation_validate_test.go` covering all three valid question types; required string/object/array instructions; missing instructions and scalar/null/invalid nested instructions; nested state; arrays as one shared state; empty questions; invalid IDs; too-short criteria; `OptionalJSON{Set:false}` with a non-nil value; nil outer provider-option objects and invalid nested provider-option values; non-string map keys; channels/functions; cycles; NaN; and infinities. Assert `errors.As` to `*ValidationError` and exact canonical `Path`/`Reason` values, including root `$`, JSON-quoted escaping for punctuation/quotes/backslashes/control characters, map keys, array indexes, the first active-stack cycle edge, sorted-map determinism, non-string-key maps, and every canonical reason.
- [x] Add `evaluation_wire_test.go` asserting exact observable JSON bodies, including required `instructions`, nil versus empty `providerOptions`, omitted boolean criteria, an empty criteria object, true-only explicit null, false-only explicit null, both explicit null, and non-null values for both keys. Cover normalization of byte slices, `json.Number`, named primitive types, and value- and pointer-receiver custom JSON marshalers so encoder-specific behavior cannot change the validated wire shape.

**Acceptance checks:**

- [x] `gofmt` reports no formatting changes for the Chunk 02 Go files.
- [x] `go test ./...` succeeds with no network access.
- [x] `go test -race ./...` succeeds.
- [x] Independent post-fix review was clean.

### Chunk 03 — Authentication, configuration, and protected headers

**Commit boundary:** `feat: configure gateway authentication and headers`

- [x] Complete `client.go` options and deterministic environment resolution in the exact fixed order: explicit API key, environment API key, explicit OIDC token/source, environment OIDC.
- [x] Create `auth.go` implementing API-key/OIDC credential selection and the `TokenSource` request-time callback.
- [x] Create `headers.go` implementing required provider headers, team scope, caller headers, and protected-header rejection.
- [x] Add `auth_test.go` covering pure construction-time credential selection: explicit API key, environment API key, explicit OIDC token/source, environment OIDC, missing credentials, blank explicit/environment credentials, nil/typed-nil token sources, last-option-wins between explicit OIDC forms, and API-key precedence. Include separate cases that an environment API key selects over an explicit OIDC token and source without invoking the source. Assert every construction failure's exact `ConfigurationError.Option`/`Reason`. Request-time source invocation/errors and fallback observations belong only to Chunk 04.
- [x] Add `headers_test.go` covering the pure header builder and option ownership: exact protected-name set, case-insensitive rejection including nil/empty values, nil input, multi-value/order preservation, defensive copy at option application and per-request clone, team validation, and every exact `ConfigurationError.Option`/`Reason`. Exact outbound observation belongs only to Chunk 04.

**Acceptance checks:**

- [x] Env-scrubbed `go test ./...` succeeds hermetically using only construction-time and pure helper tests for this chunk.
- [x] Env-scrubbed `go test -race ./...` succeeds hermetically using only construction-time and pure helper tests for this chunk.
- [x] Independent post-fix review was clean.
- [x] Tests prove credential precedence, option acceptance/copy semantics, protected-header rejection, and the closed `ConfigurationError` value mapping without constructing or sending an HTTP request.

### Chunk 04 — Raw HTTP transport and evaluation request execution

**Commit boundary:** `feat: send evaluation model requests`

- [x] Create `transport.go` with a private raw evaluation execution helper that accepts an already validated/encoded request payload, builds a context-bound `POST {baseURL}/evaluation-model` request, resolves the selected credential for that attempt, applies caller and SDK-owned headers, executes through the injected/default `http.Client`, captures status and cloned headers before body consumption, closes the response body, and returns one internal raw-response outcome containing status, headers, the bounded bytes/prefix and truncation state, plus any bounded-body read/overflow/close error. Once headers exist, that error is carried in the outcome rather than returned as the helper's top-level error, so later policy can preserve HTTP status. The model travels only in `ai-model-id`. This helper owns transport, authentication-attempt, header, and raw request/response behavior; it must not decode or validate an `EvaluationResult`, choose the public error type for a received response, or expose a new public API.
- [x] Create `internal/httpx/body.go` with the fixed unexported 1 MiB response-body limit, limit-plus-one detection, status-neutral bounded bytes/prefix and truncation reporting, and preservation of read/overflow/close causes. HTTP status policy and public error construction remain outside `internal/httpx`.
- [x] Create `internal/testserver/server.go` for exact request capture in tests only.
- [x] Create a test-only hermetic transport guard used by ordinary tests: it must reject every destination whose resolved request URL host is not loopback (`localhost`, a loopback IP, or the active `httptest.Server` loopback address), fail closed on unparsable/empty hosts, and never delegate rejected requests. Ordinary tests must scrub `AI_GATEWAY_API_KEY`, `VERCEL_OIDC_TOKEN`, and the live-evaluation cost acknowledgement `AI_GATEWAY_LIVE_COST_ACK` before client construction.
- [x] Add `evaluation_transport_test.go` using `httptest.Server` and a recording transport to exercise the private raw execution helper directly and verify path, method, exact headers/body, all base-URL acceptance and normalization cases, injected HTTP client, context cancellation, bounded body reading, and body closure. For both status 200 and non-200 responses, assert that status and headers survive body read, overflow, and close failures in the internal outcome; do not assert a public `TransportError` or `ResponseError` for a received response in this chunk. This file owns the deferred observable transport checks from Chunk 03: selected token sources are called once per attempt; source errors/blank tokens produce `TransportError("resolve OIDC token")`; API-key selection never invokes or falls back to an OIDC source; caller header maps are not mutated; the exact protected/SDK-owned header set is emitted; and concurrent raw executions through one client, including a token source, are race-free. Request-validation ordering, nil-context behavior, successful result decoding, and public `Client.Evaluate` composition belong to Chunks 05–06.

**Acceptance checks:**

- [x] Env-scrubbed `go test ./...` and `go test -race ./...` succeed under the non-loopback-rejecting guard with `AI_GATEWAY_API_KEY`, `VERCEL_OIDC_TOKEN`, and `AI_GATEWAY_LIVE_COST_ACK` unset; no test calls the public `Client.Evaluate` method in this chunk.
- [x] Env-scrubbed focused `go test -run 'TestExecuteEvaluationRequest|TestHermeticTransport' ./...` succeeds.
- [x] Independent post-fix review was clean.
- [x] Targeted tests prove a loopback server remains usable while the default live Gateway host is rejected without a network attempt; request-time token-source failures occur before transport observation; and API-key selection performs no source call or fallback.
- [x] Targeted recording tests prove the exact outbound protected/required headers, caller-header non-mutation, encoded request body with no `model` field, and `ai-model-id` containing the arbitrary requested model string.
- [x] Targeted tests prove every fixed pre-response `TransportError.Operation` mapping owned by raw execution, observe `context.Canceled` through the `send request` error chain, prove received-response body failures preserve status/headers and their underlying cause in the internal outcome, and exercise concurrent raw executions/token-source access under the race detector.

### Chunk 05 — Response decoding and contract validation

**Commit boundary:** `feat: decode and validate evaluation results`

- [x] Add private response DTOs and discriminated answer decoding to `evaluation_wire.go`.
- [x] Complete `evaluation_validate.go` with exact answer-set/type checks, the fixed absent-versus-present probability semantics, choice maxima, score ranges/weighted means, finite probability checks, complete probability key sets, rounding validation, and the fixed `1e-6`/half-unit-in-last-place formulas above.
- [x] Add a private successful-response composition function in `evaluation.go`: given caller model ID, original validated questions, and a successfully read status-200 raw outcome from Chunk 04, strictly decode the bounded raw body, validate the decoded result against the original questions, and return the completed `EvaluationResult`. Keep request validation, transport invocation, status dispatch, and public error selection for the sole public `Client.Evaluate` composition point in Chunk 06; introduce no duplicate HTTP/auth/header logic.
- [x] Decode the exact metadata wire contract: preserve absent versus present-empty `Rounding`/`Usage`, require optional counts to be valid integers, validate warning discriminator-specific required/forbidden fields, preserve nil versus empty `ProviderMetadata`, clone response headers, retain bounded raw successful-response bytes, and set the caller's model ID in `ResponseMetadata`.
- [x] Normalize only an absent warnings key to a non-nil empty slice; reject explicit null, malformed warnings, forbidden warning fields, malformed metadata, and invalid integer/count forms rather than repairing them.
- [x] Add `evaluation_response_test.go` for the private status-200 response composition and contract: cover valid boolean/choice/score results, mixed questions, unknown provider-metadata preservation, every metadata absent/present-empty/present-zero case, all four warning variants and invalid field combinations, cloned headers/body, missing/extra answers, wrong types, invalid choices/scores/probabilities, ties, weighted means, rounding decimal validity and boundaries 0 and 15, malformed JSON, trailing JSON, duplicate keys, and unknown fields at every fixed DTO level. Assert Content-Type is ignored. Assert the exact `ResponseValidationError.Path`/`Reason` mapping for malformed JSON, answer coverage/types/probabilities, warnings, rounding, usage, and provider metadata.

**Acceptance checks:**

- [x] Env-scrubbed focused `go test -run 'TestComposeEvaluationResult' ./...` succeeds with private successful-response decoding and validation complete; the public `Client.Evaluate` method is not implemented or called yet.
- [x] Env-scrubbed `go test ./...` succeeds hermetically.
- [x] Env-scrubbed `go test -race ./...` succeeds hermetically.
- [x] Independent post-fix review was clean.
- [x] Tests prove a successfully read status-200 raw outcome composes strict decoding and response validation without duplicating transport behavior.
- [x] Tests prove every request question receives exactly one matching typed answer.
- [x] Tests prove the implementation uses the specified formulas and strict-`>` rejection comparisons, including the default `1e-6` tolerance when rounding metadata is absent.
- [x] Tests prove malformed provider output yields `ResponseValidationError` with the exact status/path/reason/ID accessors, bounded defensively copied raw body, truncation flag, nil-safe zero semantics, and safe formatting.
### Chunk 06 — Typed Gateway errors and response diagnostics

**Commit boundary:** `feat: expose typed gateway errors`

- [x] Create `response_error.go` to parse known Gateway error envelopes into the fixed private `ResponseError` representation while retaining unknown bounded raw bodies only behind `RawResponseBody`; it also owns construction of every non-200 `ResponseError`, including outcomes whose body could not be completely read.
- [x] Create `internal/httpx/retry_after.go` to parse delta-seconds and HTTP-date Retry-After values against the injected `now` hook and return presence separately from a zero duration.
- [x] Populate every fixed `ResponseError` accessor: status, message, type, code, param, generation ID, request ID, response ID, Retry-After presence/value, the exact status-only retry predicate (with type/code explicitly ignored), truncation, cause, and defensive raw-body copy.
- [x] Implement, document, and compile-lock the public `Client.Evaluate` method in `evaluation.go` as the sole composition point: validate the non-nil context, model ID, and complete request before any credential-source or transport work; encode the validated wire request; invoke Chunk 04's private raw helper; dispatch on the captured status; map a status-200 body read/overflow/close failure to `TransportError("read response body")`; pass a successfully read status-200 outcome to Chunk 05 decoding; and map every non-200 outcome to `ResponseError`. A non-200 body read/overflow/close failure must retain the positive status and headers, wrap a `TransportError("read response body")` reachable through `ResponseError.Unwrap`/`errors.Is`, retain only the bounded bytes actually captured, and remain ineligible for retry by `Evaluate`.
- [x] Handle empty bodies, invalid JSON, upstream provider-shaped bodies, oversized bodies, and body-read failures without hiding the HTTP status. Parsing is attempted only when the non-200 body was read successfully; a failed read produces no invented envelope fields, while the body-read cause remains reachable through `Unwrap`/`errors.Is`.
- [x] Add `response_error_test.go` covering public `Client.Evaluate` request ordering and status dispatch; the zero value and nil receiver; boundary statuses 407/408/409/410, 428/429/430, 499/500/599/600 plus 400, 401, 402, 403, and 404; known/unknown envelopes; retryable statuses carrying arbitrary known/unknown type/code values; non-retryable statuses carrying the same values; valid zero, malformed, negative, seconds, and HTTP-date Retry-After; proof that Retry-After does not alter classification; all IDs; missing fields; truncation; a fresh defensive copy per raw-body accessor call; and `errors.As`/`errors.Is`. Include response bodies echoing unique secret markers from request `state` and `providerOptions`. Assert nil context and every invalid request fail before credential/transport observation, only status 200 enters Chunk 05 decoding, status-200 body read/overflow/close failures are `TransportError("read response body")`, and non-200 equivalents are status-preserving `ResponseError` values unwrapping that `TransportError`.
**Acceptance checks:**

- [x] Env-scrubbed `go test ./...` and `go test -race ./...` succeed hermetically with the public `Client.Evaluate` method now complete and package-external compile coverage locking its fixed signature; env-scrubbed `go doc .` shows the documented method on `Client` for the first time.
- [x] Tests prove `ResponseError.Retryable()` is true exactly for 408, 409, 429, and 500–599 and false otherwise, independent of `error.type`, `error.code`, Retry-After, or body parseability; tests also prove that a non-200 body-read/overflow/close failure carries an internal read-failure cause that Chunk 07 can deterministically exclude from retries.
- [x] Secret markers and raw-body text are absent from `Error()`, `%s`, `%v`, `%+v`, `%q`, and wrapped-error output, while the accessor retains the bounded bytes; authorization and caller-header values are absent from the same surfaces.
- [x] Independent post-fix review was clean.

### Chunk 07 — Opt-in bounded retry policy

**Commit boundary:** `feat: add opt-in evaluation retries`

- [x] Create `retry.go` using the fixed `RetryPolicy` normalization, bounds, attempt-count units, exponential formula, symmetric jitter, `MaxDelay` caps, and Retry-After replacement semantics; use only private `retryHooks` for deterministic tests.
- [x] Retry only errors classified retryable and only when resolved `MaxAttempts` is greater than one; `RetryPolicy{}` and `MaxAttempts:1` each send exactly one attempt.
- [x] Never retry request validation, response validation, authentication/configuration, context cancellation, body-read/overflow/close failures, or non-retryable statuses. For a `ResponseError`, test the preserved internal body-read-failure cause before applying its status-only `Retryable()` result, so an unreadable 408/409/429/5xx response deterministically stops while a fully read response at the same status may retry.
- [x] Ensure request bodies are recreated for every attempt and token sources are consulted per attempt.
- [x] Add `retry_test.go` with private fake hooks covering every default and validation boundary, attempt counts 0/1/2/10 and rejection of negative/11, exact exponential delays, multiplier overflow-safe capping, deterministic jitter endpoints, Retry-After absent/zero/seconds/date/malformed/negative and cap behavior, cancellation during wait, credential refresh, exhaustion, and no retries after a valid HTTP 200 with invalid evaluation output. Add separate status-200 and non-200 body read/overflow/close cases; for non-200 include otherwise retryable 408, 409, 429, and 5xx statuses and assert one attempt, preserved `ResponseError.StatusCode`, and the wrapped `TransportError("read response body")`.

**Acceptance checks:**

- [x] Env-scrubbed `go test ./...` succeeds without real sleeping.
- [x] Env-scrubbed `go test -race ./...` succeeds without real sleeping.
- [x] Independent post-fix review was clean.
- [x] Tests prove both zero-value policy and `MaxAttempts:1` send exactly one request for a retryable 500, and all sleeps are observed through private hooks with no real sleeping.
- [x] Public docs state the exact retry field units/defaults/bounds and that opt-in retries may duplicate billable evaluation work.

### Historical Chunk 08 — Document the then-native `x_search` unsupported decision

**Commit boundary:** `docs: record x search gateway support decision`

This documentation-only historical chunk was completeable without credentials and did not create an x_search test or implementation. Its then-current unsupported decision is preserved as evidence and is superseded only for the later fieldless Gateway request declaration.

- [x] Create `docs/x-search.md` documenting confirmed direct-xAI behavior: `POST https://api.x.ai/v1/responses`, top-level `tools:[{type:"x_search", ...}]`, snake_case options, provider-executed `x_search_call`, and direct `@ai-sdk/xai` support.
- [x] Document that current `@ai-sdk/gateway@4.0.87` has no `xSearch` helper and public Gateway `/v1` docs do not establish native xAI `x_search` support.
- [x] Record the evidence-review date, cited upstream versions/sources, and the release decision: Gateway-native `x_search` is unsupported and all exported x_search APIs remain absent.
- [x] State that generic model tool capability is insufficient evidence and that only a future reviewed plan backed by a first-party Gateway wire contract and authenticated native-tool evidence may change the decision.

**Acceptance checks:**

- [x] `go test ./...` remains credential-free and contains no Gateway x_search probe.
- [x] Repository search and API review find no exported x_search API or unsupported implementation claim.
- [x] The README support matrix and `docs/x-search.md` consistently say Gateway-native x_search is unsupported/unconfirmed while distinguishing direct xAI facts.
- [x] Independent post-fix review was clean.

**Future blocked item — outside chunks 01–12 and not a release prerequisite:** `[!]` An authenticated Gateway-native x_search probe may be planned only after first-party documentation supplies the Gateway endpoint, protocol/request schema, native response evidence shape, and a suitable explicit model ID. Before execution it must additionally define credential variables, a dedicated cost-acknowledgement variable, sanitization rules, and success criteria that require native `x_search_call` evidence rather than generated text. Until every prerequisite exists, do not create a placeholder/skipping test, invent a request, or block the evaluation release. **Classification: blocked/deferred outside the release; first-party Gateway wire/model evidence, authorized credentials, and explicit cost authorization are all required.**

### Chunk 09 — Credentialed evaluation contract smoke

**Commit boundary:** `test: add gated evaluation contract smoke`

- [x] Create `internal/livecontract/evaluation_test.go` behind build tag `livecontract`. Require `AI_GATEWAY_API_KEY` or `VERCEL_OIDC_TOKEN` and require the exact cost acknowledgement `AI_GATEWAY_LIVE_COST_ACK=I_ACCEPT_LIVE_EVALUATION_COSTS`; any other value is unacknowledged.
- [x] Exercise the public `Client.Evaluate` API against the default `/v4/ai/evaluation-model` endpoint with `typesafe-ai/jev` and one request containing boolean, choice, and score questions over a shared state.
- [x] Assert contract properties rather than subjective answer wording: three matching typed answers; finite valid probabilities/scores; exact `ResponseMetadata.ModelID`; non-nil cloned headers and bounded body; non-negative returned rounding/usage values; warnings with known types and no fields forbidden by their discriminator; and non-nil provider metadata values. Required warning-field presence and wire shapes remain release-required hermetic decoder coverage because decoded empty strings do not distinguish absent from present-empty fields.
- [x] Do not send a live request merely to provoke an error. Typed error-envelope, status/type/code retry classification, ID, Retry-After, truncation, body-read, and safe-diagnostic behavior are release-required hermetic coverage in Chunks 06–07. A live error probe is outside this release and remains blocked until first-party documentation identifies a deterministic, non-secret request with a fixed status/envelope and acceptable billing/side-effect contract.
- [x] Skip with a precise prerequisite message when credentials are absent or `AI_GATEWAY_LIVE_COST_ACK` is not exactly `I_ACCEPT_LIVE_EVALUATION_COSTS`; never fall back to a mock in this build-tagged test.
- [!] Create `docs/evaluation-live-evidence.md` and record the execution date, model, upstream package versions, sanitized result shape, and any protocol drift before a release candidate. This Chunk 09 evidence file is the sole live-run record and must not depend on the Chunk 10 public guide existing. **Blocker:** the sanitized template, pinned versions, model, exact command, and sanitization checklist exist, but the execution date/result/drift fields remain `PENDING LIVE RUN`. Completing them requires a non-empty `AI_GATEWAY_API_KEY` or `VERCEL_OIDC_TOKEN`, the exact `AI_GATEWAY_LIVE_COST_ACK=I_ACCEPT_LIVE_EVALUATION_COSTS` sentinel, and authorized paid network execution; no live result or evidence was fabricated. **Classification: blocked; the authorized paid credentialed run and sanitized observed result are prerequisites.**

**Acceptance checks:**

- [x] `gofmt` completed for the Chunk 09 Go files.
- [x] Env-scrubbed ordinary `go test ./...` passed and remained entirely hermetic without compiling or executing the live contract package.
- [x] With credentials absent, the build-tagged live-contract test skipped exactly at the credential gate: `live Gateway evaluation contract requires a non-empty AI_GATEWAY_API_KEY or VERCEL_OIDC_TOKEN`.
- [x] Independent review was clean.
- [!] `env -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK AI_GATEWAY_LIVE_COST_ACK=I_ACCEPT_LIVE_EVALUATION_COSTS go test -tags=livecontract ./internal/livecontract -run TestGatewayEvaluationContract` passes with an authorized credential already present in the environment. **Blocked:** an actual credentialed run requires a non-empty `AI_GATEWAY_API_KEY` or `VERCEL_OIDC_TOKEN`, the exact `AI_GATEWAY_LIVE_COST_ACK=I_ACCEPT_LIVE_EVALUATION_COSTS` sentinel, the opposite public acknowledgement explicitly unset, and authorized paid network execution. **Classification: blocked.**
- [!] Captured credentialed-live output and CI artifacts contain no tokens, authorization headers, or unsanitized sensitive state/provider metadata. **Blocked:** no authorized credentialed live run or evidence exists to review. **Classification: blocked until the credentialed run and hosted artifacts exist.**

### Chunk 10 — Public documentation and runnable evaluation example

**Commit boundary:** `docs: document evaluation SDK usage`

- [x] Expand `README.md` with installation; the module's exact Go 1.26 minimum, maintained 1.26/1.27 support window, and Go 1.26.8/1.27.1 as planned/manual validation targets pending the pinned CI matrix in Chunk 11; experimental/version scope; exact endpoint distinction; all option acceptance/copy semantics; API-key/OIDC precedence and token-source per-attempt behavior; team/header/protected-header rules; nil-context behavior; status-200-only success and ignored response Content-Type; exact status-only retry predicate and type/code non-effect; retry policy units/defaults/bounds and duplicate-work warning; the five exact typed error names/accessor roles and closed `ConfigurationError`/`TransportError` values; feature matrix; and no-parity statement.
- [x] Create `examples/evaluate/main.go` using `typesafe-ai/jev`, one shared state, and boolean/choice/score questions with required instructions; read credentials only from environment and print typed answers plus safe structured metadata/error accessors without secrets or raw bodies.
- [x] Create `docs/evaluation.md` with all request/answer invariants; the exact `ValidationError` and `ResponseValidationError` path grammars/reason vocabularies and field mappings; malformed/trailing/duplicate/unknown JSON behavior at every DTO level; status and Content-Type classification; absent/null/empty choice and score probability semantics; exact rounding/usage/warning/provider/response metadata optionality and wire semantics; every option's acceptance, normalization, defensive-copy, protected-header, credential/team/token-source behavior; exact `ConfigurationError.Option`/`Reason` and `TransportError.Operation` mappings; the exact status-only retry predicate and type/code non-effect; retry defaults/bounds/formula; all error accessor and zero-value semantics; provider options; validation failures; distinction from `/v1/evaluate`; and the sensitive nature of both raw-response diagnostic accessors and successful `ResponseMetadata.Body`. Examples and logging guidance must use safe structured error fields rather than raw bodies. Link it to the Chunk 09 `docs/evaluation-live-evidence.md` record, and update that record only for factual drift observed by the live run.
- [x] Cross-link `docs/x-search.md` and explicitly distinguish direct xAI support from Gateway support.
- [x] Ensure all exported identifiers have useful Go documentation, including exact `OptionalJSON`, metadata, `RetryPolicy`, and nil/zero error semantics and the warning on both `RawResponseBody` methods.

**Acceptance checks:**

- [x] `gofmt` reports no formatting changes for the Chunk 10 Go files.
- [x] Env-scrubbed `go test ./...` succeeds, compiles the example, and performs no live network call.
- [x] Env-scrubbed `go vet ./...` succeeds.
- [x] `go doc .` is sufficient to construct and call an evaluation client and independently determine every option acceptance/copy rule, response-decoding rule, metadata field, retry default/bound/unit, and error accessor/value/zero behavior.
- [x] The README support matrix and linked public documentation are complete enough for a reviewer to identify every supported and unsupported modality without reading source.
- [x] Env-scrubbed `go run ./examples/evaluate` reaches the documented missing-credentials `*ConfigurationError` path and exits before any network request.
- [x] Independent post-fix review was clean after the Go-version wording and acceptance-gate accounting corrections.

### Chunk 11 — CI and release readiness

**Commit boundary:** `ci: add module verification and release gates`
- [x] Create `.github/workflows/ci.yml` with the exact pinned matrix Go 1.26.8 and Go 1.27.1, running ordinary tests only after explicitly unsetting `AI_GATEWAY_API_KEY`, `VERCEL_OIDC_TOKEN`, and `AI_GATEWAY_LIVE_COST_ACK`. Go 1.27.1 is the primary job and exclusively runs `go test -race ./...` and `go vet ./...`; both matrix versions run `go test ./...` and the example compile check. Updating either pin requires a reviewed plan/evidence update; floating `stable`, `oldstable`, `1.26.x`, or `1.27.x` selectors are not allowed in the initial release workflow. The ordinary suite's mandatory transport guard must reject non-loopback destinations; CI fails if the default Gateway regression request reaches DNS/network instead of that guard.
- [x] Create `.github/workflows/live-contract.yml` as manual/scheduled only and never for untrusted pull requests. Supply one authorized credential from CI secrets and set the job environment to the exact sentinel `AI_GATEWAY_LIVE_COST_ACK=I_ACCEPT_LIVE_EVALUATION_COSTS`; do not accept a repository, workflow-dispatch, or caller-provided acknowledgement value. This is the sole CI path allowed to contact non-loopback services.
- [x] Add a dependency policy: standard library preferred; every new dependency requires rationale and license review.
- [!] Confirm repository owner-selected `LICENSE` exists before any public tag. **Exact blocker:** repository ownership has not selected a license or supplied committed license text; no choice may be inferred locally.
- [x] Document v0 release and migration policy in `README.md`: begin at `v0.x`; do not promise v1 compatibility while evaluation remains experimental; every exported breaking change, including during v0, requires a changelog entry, a release-note migration section naming removed/changed APIs and caller actions, and an appropriate version increment.
- [x] Document bad-release policy: published tags are immutable and must never be moved, deleted as a rollback, or reused. Correct a defective release with a new patch version containing an appropriate `retract` directive for the bad version/range and rationale; mark the hosting release as affected, publish corrected release notes, and issue a security advisory when applicable.
- [!] Complete all pre-tag release checks in this plan. The locally doable hermetic/test/race/vet/docs/examples/API/consumer/changelog/migration/static secret-payload checks are complete and local `origin` matches the module path. **Exact remaining blockers:** protected hosted CI/environment evidence; both owner-authorized paid live contracts; owner-selected license; independently verified hosted repository metadata; provenance, permissions, hosting authorization, and publication review; immutable tag; and post-tag direct-VCS/proxy/checksum evidence.
- [x] Update local `origin` to the module repository identity. Current orchestrator evidence records both fetch and push as `git@github.com:EveGoodEvening/vercel-ai-go-sdk.git`. Hosted repository existence and metadata are not inferred from that local configuration and remain a separate blocked release prerequisite; no candidate tag was created or published.

**Acceptance checks:**

- [x] With `AI_GATEWAY_API_KEY`, `VERCEL_OIDC_TOKEN`, and `AI_GATEWAY_LIVE_COST_ACK` explicitly unset and `GOPROXY=off`, `go test ./...`, `go test -race ./...`, and `go vet ./...` passed; the ordinary suite's non-loopback guard remained in force. This is local hermetic evidence, not an actual hosted CI run.
- [x] The regression test proves the default Gateway host cannot be contacted by ordinary tests; loopback `httptest.Server` traffic still works.
- [x] Static review confirmed the live-contract workflow cannot expose secrets to forked pull requests and is the only workflow permitted external network access; no hosted workflow run is claimed.
- [x] `scripts/verify-local-consumer.sh` is executable and passed from a temporary clean consumer module outside the repository using a local `replace`; it imported `gateway`, constructed an evaluation call, initialized every exported request/metadata/retry struct, read every public error accessor through `errors.As`, compiled, and found no `ConfigError`, exported retry hooks, or alternate compatibility aliases.
- [!] Release verification remains blocked on owner-selected license, independently verified hosted repository metadata, owner-authorized paid public-generation and provider-evaluation evidence, protected hosted CI/environment execution, hosting provenance/permissions/publication review, and hosting authorization. Local `origin` and all currently doable local checks are complete; no external result is claimed.
- [x] Release documentation contains the immutable-tag, retraction/patch, advisory, and v0 breaking-change migration rules.
- [x] Independent Chunk 11 review was clean.

### Final offline reconciliation before Chunk 12

- Final split-review findings were fixed and independently re-reviewed clean: redirects are disabled; exact integral JSON-number parsing handles huge balanced exponents without false rejection; and the live smoke no longer imposes warning-field or provider-name constraints beyond the exported decoded-value contract.
- `gofmt` completed on every final-fix Go file.
- From the committed clean checkout, with `AI_GATEWAY_API_KEY`, `VERCEL_OIDC_TOKEN`, and `AI_GATEWAY_LIVE_COST_ACK` unset and `GOPROXY=off`, `go test ./...`, `go test -race -count=1 ./...`, `go vet ./...`, `go test -run TestEvaluate ./...`, `go test ./examples/evaluate`, `go doc -all .`, and `./scripts/verify-local-consumer.sh` passed. The example executable reached the expected missing-credentials `*ConfigurationError` path before network access.
- The API snapshot and local-consumer audit matched the supported root-package surface exactly: the metadata declarations, `RetryPolicy`, four warning constants, and five error types/accessors are present; no `ConfigError`, exported retry hook, exported field on an error/client, compatibility alias, x_search API, or unsupported modality API is exposed.
- The current shell has no `gopls` executable, so no new LSP run was available; the immediately preceding workspace Go LSP diagnostic remained `No issues found`.
- README, `docs/evaluation.md`, `docs/x-search.md`, package documentation, and the runnable example were re-reviewed against the implemented behavior. Their evaluation-only scope, unsupported `/v1`/language/streaming/full-parity/Gateway-native-x_search statements, README feature matrix, and experimental/v0 status are accurate.
- The bad-release procedure covers a new patch, an appropriate `retract` directive and rationale, hosting-release affected marking, corrected notes, and a security advisory when applicable. The draft release-note content in `CHANGELOG.md` now states supported evaluation behavior, explicit `/v1`/language/streaming/full-parity non-goals, experimental/v0 compatibility risk, Gateway-native x_search's unsupported/unconfirmed status, the absence of credentialed live or publication evidence, and the initial-release Migration status.
- With all three environment variables unset, the build-tagged live-contract command passed by skipping at the credential gate with exactly `live Gateway evaluation contract requires a non-empty AI_GATEWAY_API_KEY or VERCEL_OIDC_TOKEN`. This is not a credentialed live success and supplies no live evidence.

### Chunk 12 — Final integration verification and release candidate

**Commit boundary:** `chore: verify evaluation SDK release candidate`
- [x] Audit every exported identifier against the supported-surface section and an API snapshot: require the exact metadata declarations, `RetryPolicy`, warning constants, and five error types/accessors; delete accidental abstractions, aliases (including `ConfigError`), exported hooks, fields, and unsupported promises. The `go doc -all .` snapshot and clean external-consumer AST/compile audit matched the supported surface exactly and found none of the forbidden exports.
- [x] Re-checked the pinned upstream evaluation packages and current first-party Gateway documentation on 2026-09-21. The npm `latest` records remain `ai@7.0.107`, `@ai-sdk/gateway@4.0.87`, `@ai-sdk/provider@4.0.17`, and `@ai-sdk/provider-utils@5.0.45`; the Evaluation page remains `last_updated: 2026-09-16`, the Responses/Chat/tool-calling pages remain `last_updated: 2026-09-08`, and the Go download feed still identifies Go 1.27.1 and Go 1.26.8 as the maintained release pins. No drift requiring a code or fixture change was found. This current continuation re-check supersedes the earlier offline deferral.
- [x] Run all hermetic verification commands below from a clean checkout with `AI_GATEWAY_API_KEY`, `VERCEL_OIDC_TOKEN`, and `AI_GATEWAY_LIVE_COST_ACK` unset. The complete local inventory passed with `GOPROXY=off`: ordinary, race, vet, focused evaluation, example-package, public `go doc`, and clean external-consumer checks; the example executable reached its expected local missing-credentials failure. `gopls` was unavailable in the current shell.
- [!] Credentialed provider-evaluation live contract against `typesafe-ai/jev` is **NOT RUN**. Exact blocker: owner-authorized paid network execution, exactly one protected credential, the sole exact `AI_GATEWAY_LIVE_COST_ACK=I_ACCEPT_LIVE_EVALUATION_COSTS` acknowledgement, approved pinned model/environment, sanitized retained evidence, and independent review. Hermetic typed-failure evidence is complete and no live error-provoking request is required.
- [x] Confirm the documentation-only x_search decision remains complete and unsupported; the future blocked probe is outside this release and is not executed. Repository/API review found no x_search implementation or export, and README plus `docs/x-search.md` consistently distinguish confirmed direct-xAI facts from unconfirmed Gateway-native support.
- [x] Review README/docs/example against observed behavior and update only factual claims. The review was clean: supported behavior, option/error/metadata semantics, unsupported surfaces, live-evidence status, feature matrix, experimental/v0 status, and the example's safe output all match the implementation and local observations.
- [!] **Superseded before execution by the approved continuation:** no evaluation-only candidate/tag or release was created, and that obsolete sequence must not be executed. The single continuation release sequence in Chunk 24 remains externally blocked on authorized live evidence, owner-selected license, protected hosted repository/CI and publication configuration, provenance review, hosting authorization, immutable tag creation, direct-VCS/proxy/checksum verification, and publication. Its locally doable upstream, API, documentation, consumer, and static secret/payload checks are accounted independently below.

**Acceptance checks:**

- [x] Hermetic suite and race checks pass with `AI_GATEWAY_API_KEY`, `VERCEL_OIDC_TOKEN`, and `AI_GATEWAY_LIVE_COST_ACK` unset and non-loopback access denied. The clean-checkout `GOPROXY=off` ordinary and uncached race runs passed under the mandatory transport guard.
- [!] Evaluation live-contract smoke is **NOT RUN**. Exact blocker: owner-authorized paid execution with exactly one production credential, the exact `AI_GATEWAY_LIVE_COST_ACK=I_ACCEPT_LIVE_EVALUATION_COSTS` sentinel, an approved protected environment/model, sanitized retained evidence, and independent review.
- [!] **Superseded before execution:** Chunk 12 produced no prerelease tag, VCS/proxy evidence, or evaluation-only release. The deferred continuation candidate is blocked on the Chunk 24 external tag/publication dependencies; this historical item is not locally actionable.
- [x] No docs or exported API claim OpenAI `/v1`, language/streaming, full AI SDK parity, or Gateway native x_search support. Repository text and API-snapshot review found only explicit non-support/non-parity statements and direct-xAI facts clearly separated from Gateway support.

## Verification command inventory

Run these only in the implementation chunks that name them; this planning-only change does not execute them.
```sh
env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go test ./...
env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go test -race ./...
env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go vet ./...
env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go doc .
env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go test -run TestEvaluate ./...
env -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK AI_GATEWAY_LIVE_COST_ACK=I_ACCEPT_LIVE_EVALUATION_COSTS go test -tags=livecontract ./internal/livecontract -run TestGatewayEvaluationContract
```

`AI_GATEWAY_API_KEY` and `VERCEL_OIDC_TOKEN` are the fixed production credential variables. `AI_GATEWAY_LIVE_COST_ACK` is the provider-evaluation live acknowledgement with sole sentinel `I_ACCEPT_LIVE_EVALUATION_COSTS`; `AI_GATEWAY_PUBLIC_LIVE_COST_ACK` is the public-API live acknowledgement with sole sentinel `I_ACCEPT_LIVE_PUBLIC_API_COSTS`. Every ordinary command and CI job must explicitly unset both credentials and both acknowledgements. A live command requires exactly one authorized credential already present plus only its named sentinel assignment. For pre-tag consumer verification in Chunk 11, create a temporary module outside the repository and use a local `replace`. The evaluation-only Chunk 12 tag/proxy/finalization sequence was superseded before execution; after Chunk 24 creates and pushes the single continuation prerelease tag, repeat from a fresh temporary module without `replace`: resolve direct VCS, resolve through `GOPROXY=proxy.golang.org`, compile, and record sanitized provenance/checksum evidence.

## Release checklist
- [!] **Historical release-check state before the authoritative evidence continuation below:** Chunks 01–23 required by the continuation were complete with Chunk 22 Hermetic `[x]`; Chunk 22 Live and Chunk 24 overall remained incomplete on the two owner-authorized paid live contracts and external release gates. At that time public Evaluate and every search category were absent. Current search disposition is stated only in the authoritative table below.
- [x] Current upstream version/protocol evidence is re-checked and cited: the 2026-09-21 first-party review found the pinned AI SDK package versions, Gateway page dates/contracts, search evidence gaps, and maintained Go pins unchanged.
- [!] The complete local hermetic, race, vet, example, docs, live-gate compile/fail-closed, and external-consumer inventory is successful with credentials/acknowledgements unset and non-loopback traffic denied. Exact remaining blocker: successful execution and retained evidence from the protected hosted CI environment; local evidence does not substitute for hosted CI.
- [!] Authorized paid provider-evaluation and public-generation success contracts are **NOT RUN**. Exact blocker: owner authorization, exactly one protected credential per isolated job, approved pinned models, paid network execution, sanitized retained evidence, and independent review.
- [x] **Historical documentation state at Chunk 24 closure:** README/support documentation described provider Evaluate, public Responses, and public Chat and listed public `/v1/evaluate` plus all search categories as blocked/absent. The later authoritative continuation supersedes only the Chat request-support disposition; it does not retroactively change this completed documentation record.
- [!] License selection is blocked on the repository owner's explicit license choice and committed license text; none may be inferred locally.
- [!] Hosted repository existence and metadata verification are blocked on owner/hosting access to confirm visibility, default branch, protection/release settings, and publication configuration. Local `origin` is complete and matches `git@github.com:EveGoodEvening/vercel-ai-go-sdk.git`, but that is not hosted verification.
- [!] Local pre-tag API, clean-consumer, changelog/release-note, migration, and static secret/payload checks are complete. Remaining pre-tag work is blocked on protected hosted CI/environment evidence, owner license, hosted metadata/provenance/publication review, authorized live contracts, and hosting authorization.
- [!] The evaluation-only prerelease tag was never created and is superseded. Creation of the single immutable continuation candidate, direct-VCS and public-proxy import, checksum capture, provenance attestation, promotion, and publication are deferred until every external pre-tag blocker clears; no tag may be created, moved, reused, or published now.
- [x] Release notes list supported evaluation behavior, explicit non-goals, experimental risk, x_search unsupported status, and any required v0 migration steps. `CHANGELOG.md` now covers the evaluation question types and typed surfaces; explicit `/v1`, language/streaming, and full-parity non-goals; experimental v0 compatibility risk; Gateway-native x_search's unsupported/unconfirmed status; no implied live or publication evidence; and the module-path migration from `github.com/EveGoodEvening/vercel-ai-gateway-go-sdk` to `github.com/EveGoodEvening/vercel-ai-go-sdk`, including the required `go.mod` and import updates.
- [x] The bad-release procedure documents a new patch with `retract`, hosting-release marking, corrected notes, and an advisory when applicable. `docs/releasing.md` and README contain every required correction step and preserve immutable failed tags.
- [x] Current repository source, documentation, examples, tests, workflows, and retained local evidence received static secret/payload review: diagnostics and examples avoid credentials, authorization headers, raw diagnostic bodies, prompts/state, generated content, tool data, provider values, and unsanitized live payloads. No live/hosted/tag/proxy artifacts exist yet; their future sanitization attestation remains an external pre-publication gate rather than a defect in current local artifacts.

## Approved continuation — public generation and evaluation APIs

The following continuation is a newly approved goal. It does **not** rewrite, reopen, or erase Chunks 01–12, their completed history, or their external release blockers. The initial evaluation-only statements above remain the historical contract for the work already completed. For Chunks 13 onward, this section is authoritative where it broadens that earlier scope.

### Continuation decisions and boundaries

1. **Use public `/v1` generation surfaces, not internal `/v4/ai/language-model`.** Implement both `POST /v1/responses` and `POST /v1/chat/completions` as separate, explicitly named Go APIs. `/v1/responses` is the preferred feature-rich surface for new callers; `/v1/chat/completions` exists for established OpenAI Chat Completions compatibility. Do not implement `/v4/ai/language-model`: it is the AI SDK provider adapter protocol rather than the stable public REST API, and the pinned provider declaration/fixture disagreement over stream delta fields (`delta` versus `textDelta`) makes a Go wire contract unsafe to infer. Reconsidering `/v4/ai/language-model` requires a separate approved plan plus matching first-party schema/fixture and authenticated wire evidence.
2. **Keep public `/v1/evaluate` as an evidence-gated additive surface.** <https://vercel.com/docs/ai-gateway/modalities/evaluation> (`last_updated: 2026-09-16`, inspected 2026-09-21) establishes `POST https://ai-gateway.vercel.sh/v1/evaluate` and its request shape, but not enough public success presence/nullability/extension semantics for a deterministic Go result decoder. The existing `Client.Evaluate` remains the Evaluation Model V4 provider-protocol method at `/v4/ai/evaluation-model`; the public method stays absent until the complete gate below clears and must never share or silently translate provider-protocol headers/body semantics. This does not block generation.
3. **Model each implemented wire surface independently.** Responses, Chat Completions, provider-protocol Evaluate, and public Evaluate if later unblocked have separate request/response/stream DTOs, validation, and tests. Shared code is limited to transport mechanics, bounded reading, authentication, safe error capture, and SSE framing where the wire contract is actually common. There is no catch-all `Generate`, generic untyped tool bag, or automatic tool execution.
4. **Historical search decision for Chunks 13–24; superseded for current status by the 2026-09-21 search-request continuation below.** The first-party pages then used for those chunks did not establish Responses built-in search or Chat server-search request contracts, so neither entered the required continuation chain. That evidence record remains traceable, but it is not a current blocker for the exact implemented request declarations: fixed low-context Responses `{"type":"web_search","search_context_size":"low"}` and fieldless Gateway `{"type":"x_search"}` are supported through the opt-in wrapper. Configurable options, `web_search_preview` and other forms, typed outputs/events, wider model compatibility, direct-xAI, and unrelated surfaces remain blocked.
5. **Historical Gateway-native xAI `x_search` rule for Chunks 13–24; superseded for the fieldless request declaration only.** At that time direct xAI evidence was insufficient for Gateway, so no `x_search` request type, helper, response item, or support claim could ship without first-party Gateway evidence and an authorized live probe showing a native `x_search_call` item. The later evidence and implementation clear only fieldless Gateway `{"type":"x_search"}` through the opt-in wrapper; configurable options, typed outputs/events, wider model compatibility, direct-xAI, and unrelated surfaces remain blocked. Generic web search, model tool capability, or generated prose mentioning X is still not proof of any broader contract.
6. **Compatibility and package shape.** Keep module `github.com/EveGoodEvening/vercel-ai-go-sdk` and root package `gateway`. Preserve every existing exported identifier and existing `WithBaseURL` behavior for provider-protocol evaluation. Add explicit surface configuration (`WithPublicBaseURL`) defaulting to `https://ai-gateway.vercel.sh/v1`; do not reinterpret `WithBaseURL`. The existing credential precedence, `WithHTTPClient`, team/header ownership, token refresh, typed error safety, retry defaults, and context behavior apply unless a surface section below states a stricter rule.
7. **Retries and resource ownership.** Generation and public evaluation default to one attempt. Non-stream retries may use the existing opt-in status policy only before a successful response is returned; streaming requests are never automatically replayed after response headers or any event bytes are received. Every stream is caller-closed, context-cancellable, single-consumer, and bounded by fixed event/line/error-body limits. The SDK never buffers an unbounded stream and never starts hidden goroutines that can outlive `Close` or context cancellation.

### Continuation dependency and review rules

- Required Chunks 13, 15–16, 18–19, 21, and 23–24 execute sequentially, one chunk at a time; no later required chunk starts until its stated predecessor/gate is complete. Evidence-gated Chunk 14 and the evidence-gated search items are outside this dependency chain and may remain `[!]`. Chunk 22 has separately status-bearing Hermetic and Live gates: Chunk 23 may begin only after Chunk 22 Hermetic is `[x]`; Chunk 24 additionally requires Chunk 22 Live `[x]` for every surface actually implemented and claimed. Existing Chunks 09/11/12 external release blockers do not prevent implementation of this separately approved continuation, but **no release may be tagged** until both the preserved blockers and the continuation release gates are satisfied.
- **The exact first dependency-ready implementation work is Chunk 13.** It has no external credential, hosting, license, or live-network prerequisite. Start with its public-base/auth/header compatibility changes and hermetic tests; after Chunk 13, proceed to Chunk 15 while public Evaluate remains gated unless its evidence prerequisite has independently cleared.
- Each required chunk is one reviewable, verifiable, committable change with the listed Conventional Commit subject. Mark a required chunk `[x]` only after all tasks, exact commands, and independent review accounting in that chunk are complete. For Chunk 22, apply that rule independently to its named Hermetic and Live status checklists. Review accounting must record the reviewer, findings, fixes, and clean re-review in the commit/PR record; “self-reviewed” without findings accounting is insufficient.
- Owned-file lists are the expected mutation boundary. Tests may use existing `internal/testserver`; changing files outside the list requires updating this plan before implementation. Documentation and release chunks intentionally own more documentation/gate files.
- All ordinary commands must run with credentials and both live acknowledgements (`AI_GATEWAY_LIVE_COST_ACK` and `AI_GATEWAY_PUBLIC_LIVE_COST_ACK`) unset. No implementation chunk may perform a paid/live request unless it is explicitly a live-contract item and has the named authorization sentinel.

### Chunk 13 — Public base URL, authentication, and API compatibility foundation

**Depends on:** preserved Chunks 01–08 implementation; no unresolved continuation dependency.

**Owned files:** `client.go`, `errors.go`, `headers.go`, `client_public_test.go`, `contract_external_test.go`, `.github/workflows/ci.yml` (six-file exception to the 3–5 target-path preference: this earliest sentinel-owning chunk must atomically scrub the newly introduced public live acknowledgement from ordinary hosted CI; deferring the workflow edit would leave Chunks 13–23 unable to satisfy the hermetic-CI invariant).

**Commit boundary:** `feat: add public gateway api configuration`

- [x] Add `WithPublicBaseURL(string) Option`, defaulting to `https://ai-gateway.vercel.sh/v1`, with the same strict URL acceptance and trailing-slash normalization rules as `WithBaseURL`. Preserve `WithBaseURL` exclusively for `/v4/ai` provider evaluation.
- [x] Define private endpoint construction that appends exact surface paths without `path.Join` cleaning or accidental escaped-path changes. Reject query, fragment, userinfo, opaque, relative, blank, and non-HTTP(S) values at construction.
- [x] Preserve credential precedence and request-time OIDC source behavior for both base URLs. Define public `/v1` owned headers as `Authorization`, `Content-Type`, and optional `X-Vercel-Ai-Gateway-Team`; provider-protocol version/spec/model headers remain exclusive to `/v4` evaluation.
- [x] Extend protected-header validation so caller headers cannot override any SDK-owned header on either surface, without changing existing option semantics or exported error type names.
- [x] Compile-lock the additive API and prove all existing evaluation construction and `Evaluate` call sites remain source-compatible. Add `WithPublicBaseURL` to the closed, documented `ConfigurationError.Option()` value set in `errors.go`; retain the existing `ConfigurationError.Reason()` vocabulary unchanged.
- [x] Update every ordinary command in `.github/workflows/ci.yml` to explicitly unset `AI_GATEWAY_API_KEY`, `VERCEL_OIDC_TOKEN`, `AI_GATEWAY_LIVE_COST_ACK`, and `AI_GATEWAY_PUBLIC_LIVE_COST_ACK`; no ordinary or pull-request-triggered job may inherit either live acknowledgement.

**Acceptance commands:**

```sh
env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go test -run 'Test(NewClient|WithPublicBaseURL|PublicHeaders|ExistingEvaluationCompatibility)' ./...
env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go test ./...
env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go vet ./...
```

**Recorded verification evidence:**

- [x] `env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go test -run 'Test(NewClient|WithPublicBaseURL|PublicHeaders|ExistingEvaluationCompatibility)' ./...` passed across all packages.
- [x] `env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go test ./...` passed.
- [x] `env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go vet ./...` passed.
- [x] `gofmt` completed for the implemented files.
- [x] Workspace Go diagnostics reported no issues.
- [x] `git diff --check` passed.

**Review/blocker accounting:**

- [x] Independent review found that `WithPublicBaseURL` accepted a URL ending in an empty literal `#` fragment delimiter. The implementation now rejects the literal delimiter while preserving escaped `%23` paths and unchanged `WithBaseURL` behavior.
- [x] Post-fix verification passed: focused fragment tests; `env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go test ./...`; standalone post-fix `env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go vet ./...`; workspace Go diagnostics; and `git diff --check`.
- [x] Commit-subject correction complete at HEAD `bb7ca24`: `feat: add public gateway api configuration`.
- [x] First independent re-review confirmed the fragment fix preserves escaped `%23` and `WithBaseURL` behavior and found no remaining implementation issue.
- [x] Second independent re-review confirmed the implementation clean; its accounting note requested standalone post-fix `go vet ./...` evidence, now resolved by the passing env-scrubbed command recorded above.
- [x] Clean independent re-review after the fragment fix and commit-subject correction.
- [x] Independent review confirms `WithBaseURL` was not repurposed, no existing export was removed/renamed, `/v1` requests receive no `/v4` protocol headers, every ordinary hosted-CI command scrubs both credentials and both live acknowledgements, and no network is used.
- [x] **Block only if** first-party documentation changes the public base origin/path or authentication contract before implementation; record the exact source/version and decision required. No current blocker is known.

### Evidence-gated Chunk 14 — Public `/v1/evaluate`

**Prospective dependency after unblocking:** Chunk 13 `[x]`. This item is outside the required generation chain; Chunk 15 depends directly on Chunk 13.

**Prospective owned files:** `public_evaluation.go`, `public_evaluation_wire.go`, `public_evaluation_test.go`, `response_error.go`, `contract_external_test.go`.

**Prospective commit boundary:** `feat: add public gateway evaluation api`

- [!] Do not add `Client.EvaluatePublic` or public success DTOs/decoder yet. The page at <https://vercel.com/docs/ai-gateway/modalities/evaluation> (`last_updated: 2026-09-16`) establishes `POST https://ai-gateway.vercel.sh/v1/evaluate`, its request example, and a success-field inventory, but the currently pinned first-party public-HTTP evidence does not establish the requiredness/nullability of `model`, `answers`, `usage`, or `providerMetadata`, whether additional top-level fields are permitted, or whether unknown fixed-object fields must be rejected or retained. A stable Go result cannot be chosen without guessing presence semantics or a forward-compatibility policy. This gate does not block Responses or Chat text generation.
- [!] **Evidence required to unblock the whole success decoder:** a versioned first-party Vercel public-HTTP OpenAPI/schema, released Gateway source/test, or exact public `/v1/evaluate` fixture contract that states, for every top-level success member, presence/requiredness, nullability, wire type, and unknown-field policy. It must also state answer-object requiredness, whether choice/score `probabilities` may be absent or null, usage count integer/nonnegative/null rules, and whether `rounding`, `warnings`, `response`, `totalTokens`, or other top-level members exist. Authorized sanitized live fixtures may corroborate but cannot alone prove absence, optionality, nullability, or extension policy.
- [!] **Deterministic decoder rule after evidence arrives:** represent every proven optional or nullable member with an explicit private presence/null wrapper and map it to an equally unambiguous public pointer/presence type; reject duplicate keys and trailing JSON; apply the evidence's exact unknown-field rule separately at the top level, fixed answer/usage objects, and extension objects. Only `providerMetadata` may retain unknown nested bounded JSON when first-party evidence explicitly designates it as an extension point. Never infer public-HTTP presence rules from Evaluation Model V4 declarations.
- [!] **Request contract retained for future implementation:** send `POST {publicBaseURL}/evaluate`; body `model` is a required nonempty string, `state` is a required JSON-compatible string/object/array, `questions` is a required nonempty object with the documented boolean/choice/score shapes, and `providerOptions` is optional. The shown `providerOptions.gateway.zeroDataRetention` boolean and `providerOptions.gateway.only` string array are the only proven option fields. Fixed request objects reject unknown fields; no generic option bag or provider-protocol header/body translation is allowed.
- [!] The cited page establishes no public `/v1/evaluate` JSON error envelope. After unblocking, non-success responses may expose only already-proven HTTP status, headers, Retry-After, and bounded defensive raw bytes through `ResponseError`; typed body fields remain individually blocked until a versioned first-party `/v1/evaluate` error contract defines them.
- [!] **Limits owned by this item when unblocked:** enforce before network/deep allocation the shared 64-level JSON nesting ceiling, 1 MiB maximum individual string/byte value, 10,000-member maximum for every map/object or array, 10,000-question maximum, and 1 MiB buffered success/error-body ceiling. Limit-plus-one cases must return stable typed validation/response errors. These limits are resource policy, not wire-schema claims.
- [!] Unblocking review must map every request/result field and presence/null/unknown-field decision to the qualifying first-party evidence, prove existing `/v4/ai/evaluation-model` behavior unchanged, and run focused hermetic/race/full ordinary commands with credentials and both live acknowledgements unset.

### Chunk 15 — Responses non-stream text generation

**Depends on:** Chunk 13 `[x]`; the public `/v1/evaluate` item may remain `[!]`.

**Owned files:** `responses.go`, `responses_wire.go`, `responses_validate.go`, `responses_test.go`, `contract_external_test.go`.

**Commit boundary:** `feat: add responses text generation`

- [x] Add a distinctly named `Client.CreateResponse` API targeting `POST {publicBaseURL}/responses`; required wire fields are `model` (nonempty `provider/model` string) and `input` (string or typed input-item array); the SDK owns `stream:false`.
- [x] Implement this complete first-party Responses request inventory from <https://vercel.com/docs/ai-gateway/sdks-and-apis/responses> (`last_updated: 2026-09-08`, inspected 2026-09-21): `max_output_tokens` (integer), `temperature` (number `[0,2]`), `top_p` (number `[0,1]`), `presence_penalty` and `frequency_penalty` (numbers; `[!]` exact bounds are not stated on that page and must be obtained from a versioned Vercel schema before enforcing any range), `instructions` (string), function-only `tools` (array with `type:"function"`, name, optional description, JSON Schema parameters, optional strict), `tool_choice` (`auto|required|none` or documented specific-function object), `parallel_tool_calls` (boolean), `allowed_tools` (array of tool names), `reasoning.effort` (`none|minimal|low|medium|high|xhigh|max`) and optional documented summary values, documented `text` structured-output forms, `truncation` (`auto|disabled`), `previous_response_id`, `store`, constrained string-pair `metadata`, `caching` (`auto`), `cache_anchor_items`, `cache_ttl` (`5m|1h` with caching), and `prompt_cache_key`. The exact request DTO/presence rules come from that page and its linked first-party pages, each carrying `last_updated: 2026-09-08`; omitted option pointers omit keys, and explicit JSON null is never emitted unless a cited page explicitly documents null.
- [x] **Exact exported success representation:** `type ResponseResult struct { rawJSON []byte }` with `func (r *ResponseResult) RawJSON() []byte`; the method returns nil on a nil receiver and otherwise a fresh non-nil defensive copy of the complete status-200 JSON value. A successful decode requires a non-null top-level JSON object and stores its original bounded bytes unchanged. Do not export guessed `ID`, `Object`, `Model`, timestamps, status, `OutputText`, output items, incomplete/error details, usage, or metadata fields in this chunk. The current first-party Responses overview/text/tool/structured-output/reasoning pages (all `last_updated: 2026-09-08`, inspected 2026-09-21) show SDK property access and selected item examples but publish no complete non-stream HTTP success fixture/schema establishing those fields' requiredness, nullability, nesting, or extension policy. In particular, `response.output_text` is documented only as an OpenAI-SDK convenience access in <https://vercel.com/docs/ai-gateway/sdks-and-apis/responses/text-generation>; it is therefore explicitly surfaced only through `RawJSON()` and remains `[!]` as a typed Go field until a versioned first-party Vercel HTTP schema/fixture defines its wire location and presence/null behavior.
- [x] **Exact decoder policy:** accept exactly one JSON value followed only by JSON whitespace; reject malformed JSON, trailing values, duplicate object keys at every depth, a top-level non-object, depth over 64, any string/byte value over 1 MiB, or any array/object over 10,000 members. Within the accepted top-level object and all nested objects, preserve unknown keys and unsupported output-item variants byte-for-byte only as part of `RawJSON`; do not interpret, validate, discard, normalize, or claim support for them. This deliberate forward-compatible raw envelope is capped by the existing 1 MiB successful-body limit, is defensively copied on construction and every accessor/copy boundary, and must never be included in `Error()`/formatting. JSON null remains JSON null and absence remains absence in the retained bytes; no public typed optionality is invented. These acceptance rules are SDK resource/forward-compatibility policy, not a claim that Vercel guarantees unknown fields or variants.
- [x] Typed Responses success fields may be added only field-by-field after a versioned/current first-party Vercel page, OpenAPI/schema, released Gateway source/test, or exact public HTTP fixture states that field's wire path, type, requiredness, nullability, nested shape, and variant discriminator. A newly proven optional or nullable field must use an explicit public presence representation (pointer when absent versus present-non-null is sufficient; otherwise a presence/null wrapper); fixed typed DTO levels reject unknown fields unless the same evidence designates an extension point. Unsupported discriminated variants must continue to survive only in bounded `RawJSON`, never be coerced into a known variant. This gate applies independently to ID/object/model/timestamps/status, text/refusal/reasoning/function-call items, incomplete/error details, usage, and metadata, so one missing field does not block the raw-result chunk.
- [x] Apply strict request validation before credentials/network and bounded success/error bodies. Context cancellation must interrupt send/read; only status 200 is successful unless first-party documentation proves another exact success status. Retain unknown provider extension objects only in explicitly documented request metadata/provider-option fields. Gateway routing fields not enumerated on the Responses page are `[!]` for this surface until a versioned first-party Vercel Responses page/schema defines their exact wire names and value contracts.
- [x] Enforce in this chunk, before network or excessive allocation: maximum JSON nesting depth 64 for input items, schemas, metadata, provider options, and success JSON; maximum 1 MiB per string/byte value; maximum 10,000 members per array/object; maximum 10,000 input items and tools; and the existing 1 MiB buffered success/error-body ceiling. Limit-plus-one cases return stable typed validation/response errors. Do not defer these surface bounds to Chunk 21.

**Acceptance commands:**

```sh
env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go test -run 'Test(CreateResponse|ResponsesRequest|ResponsesResult)' ./...
env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go test -race -run 'Test(CreateResponse|ResponsesRequest|ResponsesResult)' ./...
env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go test ./...
```

**Recorded post-fix verification evidence:**

- [x] `env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go test -run 'Test(CreateResponse|ResponsesRequest|ResponsesResult)' ./...` passed.
- [x] `env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go test -race -run 'Test(CreateResponse|ResponsesRequest|ResponsesResult)' ./...` passed.
- [x] `env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go test ./...` passed.
- [x] `env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go vet ./...` passed.
- [x] `gofmt` completed for the implemented files.
- [x] Workspace Go diagnostics reported no issues.
- [x] `git diff --check` passed.

**Review/blocker accounting:**

- [x] Review fix implemented and verified: request validation now enforces the required 1 MiB byte ceilings on request string fields before credentials/network.
- [x] Review fix implemented and verified: documented character-count limits now count Unicode characters rather than bytes, preserving valid multibyte UTF-8 metadata and prompt-cache values.
- [x] Review fix implemented and verified: request JSON validation now detects pointer cycles and applies depth accounting across pointer traversal, returning a stable typed error instead of hanging.
- [x] Review fix implemented and verified: successful response validation now rejects invalid UTF-8 inside JSON strings.
- [x] First independent re-review confirmed all four review fixes and found no remaining implementation issue.
- [x] Second independent re-review confirmed all four review fixes and found no remaining implementation issue.
- [x] Third independent re-review confirmed all four review fixes and found no remaining implementation issue.
- [x] Clean independent re-review result recorded after the fixes and post-fix verification.
- [x] Independent review mapped every interpreted request field and the intentionally raw-only success surface to the cited current first-party `/v1/responses` pages, verified duplicate/trailing/limit behavior and defensive copies, and rejected speculative OpenAI success fields not documented by Gateway.
- [x] **Block a specific typed success field, not the whole chunk,** when Gateway documentation omits its wire semantics; keep that field available only through bounded `RawJSON` and mark it `[!]` with the exact missing evidence rather than guessing. No current blocker prevents completion of the raw-result chunk.

### Chunk 16 — Responses streaming and resource lifecycle

**Depends on:** Chunk 15 `[x]`.

**Owned files:** `responses_stream.go`, `sse.go`, `responses_stream_test.go`, `internal/httpx/body.go`, `contract_external_test.go`.

**Commit boundary:** `feat: stream responses text generation`

- [x] Add `Client.StreamResponse` returning a caller-owned stream with `Next`, typed `Event`, `Err`, and idempotent `Close` semantics; do not use an exposed channel or background producer goroutine.
- [x] Parse HTTP SSE incrementally, accepting CRLF/LF framing and multi-line `data` fields as required by SSE. The only currently proven typed payload is `response.output_text.delta` from <https://vercel.com/docs/ai-gateway/sdks-and-apis/responses> (`last_updated: 2026-09-08`); expose its documented text delta plus the SSE event name/ID. For every other event type, expose only a bounded raw event containing its discriminator and defensive-copy JSON bytes, without interpreting completion, failure, refusal, tool/output-item, usage, or error semantics.
- [x] Set `stream:true`; require status 200 before exposing a stream. Enforce in this chunk a 64 KiB SSE-line ceiling, 1 MiB assembled-event ceiling, 1 MiB retained terminal-diagnostic ceiling, 10,000-event maximum, 64-level JSON nesting ceiling, 1 MiB maximum decoded string/byte value, and 10,000 members per decoded array/object. Oversize or over-count input closes the body and returns a typed transport/stream error before excessive allocation.
- [x] Cancellation or `Close` must promptly unblock a blocked read, close the body exactly once, release the connection, and cause subsequent `Next` to terminate deterministically. Because current first-party Vercel evidence does not define a Responses terminal event, clean HTTP EOF after complete SSE framing is successful transport termination; the SDK must not require or synthesize an application terminal event. Malformed/truncated framing remains an error. No retry occurs after headers or stream bytes.
- [x] Preserve event order and raw IDs/types needed by callers; the SDK accumulates no complete transcript and executes no tools. Completion, failure, refusal, tool/output-item, usage, and error event DTOs and terminal semantics are individually `[!]` until an exact versioned first-party Vercel page/OpenAPI/released source/test defines the discriminator, complete payload, field presence/nullability, unknown-field policy, and termination effect.

**Acceptance commands:**

```sh
env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go test -run 'Test(StreamResponse|SSE)' ./...
env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go test -race -run 'Test(StreamResponse|SSE)' ./...
env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go test ./...
```

**Recorded implementation verification evidence:**

- [x] `env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go test -run 'Test(StreamResponse|SSE)' ./...` passed.
- [x] `env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go test -race -run 'Test(StreamResponse|SSE)' ./...` passed.
- [x] `env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go test ./...` passed.
- [x] `env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go vet ./...` passed.
- [x] `gofmt` completed for the implemented files.
- [x] Workspace Go diagnostics reported no issues.
- [x] `git diff --check` passed.

**Review/blocker accounting:**

- [x] Independent review explicitly checked blocked-read cancellation, idempotent and exactly-once close, clean-EOF behavior, malformed/truncated framing, limit-plus-one behavior, allocation bounds, raw-event defensive copies, absence of goroutine leaks/replay, and terminal reservation/publication across slow or erroring body closure plus concurrent `Close`, cancellation, and `Next` races.
- [x] Review fix implemented and verified: SSE `id` state now persists as the last-event ID across subsequent dispatched events that omit `id`, with focused framing coverage.
- [x] Review fix implemented and verified: a dispatched SSE event with no explicit `event` field now exposes the standard default event name `message`, with focused coverage.
- [x] Review fix implemented and verified: every decoded event JSON object now requires a present, non-null, nonempty string `type` discriminator before typed-delta decoding or raw fallback, with focused coverage.
- [x] Review fix implemented and verified: explicit `Close` clears a previously exposed `Event` even when context cancellation won the shared `sync.Once`, with focused lifecycle and race coverage.
- [x] Review fix implemented and verified: concurrent `Close` and successful `Next` event publication are serialized so `Next` cannot publish an event after closure, with a deterministic lifecycle regression and race coverage.
- [x] Review fix implemented and verified: cancellation observed after event decode but before publication synchronizes terminal-state transition with publication so a decoded event cannot be exposed after cancellation. The first deterministic regression ordering failed because it sampled `Err` before terminal state; test synchronization was corrected to await the terminal transition, after which focused, race, full, vet, formatting, diff-check, and diagnostics verification passed.
- [x] Review fix implemented and verified: terminal state is published atomically only after body closure and final `Event`/`Err` stabilization, preventing observers from seeing closed state before terminal accessors are stable, with deterministic slow/blocking body-close concurrency regressions and race coverage.
- [x] Clean independent re-review confirmed all seven review fixes and found no remaining implementation issue.
- [x] **Block every event interpretation beyond `response.output_text.delta`** until the exact first-party evidence gate above clears; bounded raw preservation is implemented and is not a support claim for any blocked event schema.

### Evidence-gated continuation item — Responses built-in web search

**Prospective commit boundary after unblocking:** `feat: add responses web search tool`

- [!] Do not implement a Responses `web_search` tool, `search_context_size`, search-call output, or citation/annotation schema now. **Blocker:** <https://vercel.com/docs/ai-gateway/sdks-and-apis/responses> and <https://vercel.com/docs/ai-gateway/sdks-and-apis/responses/tool-calling> (`last_updated: 2026-09-08`) document function tools only and provide no Gateway Responses built-in-search request or response/event wire contract. OpenAI-native documentation alone is not Vercel Gateway evidence.
- [!] **Exact parameter/output inventory requiring evidence:** request tool discriminator; every request field with type, requiredness, enum/bounds (including whether `search_context_size` exists and its allowed values); compatible Gateway model IDs; non-stream search-call/result item discriminators and fields; streaming event types/deltas; citation/annotation URL, title, and offset semantics; refusal/error shapes; and coexistence/precedence with function tools and Gateway routing. Each field must be cited to an exact versioned first-party Vercel URL/schema/source test before it enters the API; unproven fields remain individually blocked and must not be represented by a generic map.
- [!] **Evidence required to unblock:** either (1) a versioned first-party Vercel Gateway Responses page, OpenAPI/schema, or released Gateway source/test defining the exact request plus non-stream and streaming result shapes, or (2) the same request schema plus an owner-authorized paid live fixture through `/v1/responses` that returns the claimed structured items/events in a sanitized capture. Once unblocked, use the five-file scope `responses_tools.go`, `responses_wire.go`, `responses_tools_test.go`, `responses_stream_test.go`, and `contract_external_test.go`; make the tool compile-time Responses-only; and run focused hermetic/race/full ordinary commands with both live acknowledgements unset. This item is outside the required sequential chain and does not block Chunk 18.

### Chunk 18 — Chat Completions non-stream compatibility

**Depends on:** Chunk 16 `[x]`; the Responses built-in-search item may remain `[!]`.

**Owned files:** `chat.go`, `chat_wire.go`, `chat_validate.go`, `chat_test.go`, `contract_external_test.go`.

**Commit boundary:** `feat: add chat completions compatibility`

- [x] Add `Client.CreateChatCompletion` targeting `POST {publicBaseURL}/chat/completions`; required wire fields are `model` (nonempty string) and `messages` (nonempty typed array); the SDK owns `stream:false`.
- [x] Implement this complete first-party Chat request inventory from <https://vercel.com/docs/ai-gateway/sdks-and-apis/openai-chat-completions/chat-completions> and its linked tool/advanced pages (`last_updated: 2026-09-08`, inspected 2026-09-21): `messages` with documented roles and string or typed `text`, `image_url`, and `file` content parts; `temperature` (`[0,2]`), `max_tokens` (integer), `top_p` (`[0,1]`), `frequency_penalty` and `presence_penalty` (`[-2,2]`), `stop` (string or string array), `safety_identifier` (nonempty opaque string; Gateway truncates after 64 characters), function-only `tools` (`type:"function"` with name, optional description, JSON Schema parameters), `tool_choice` (`auto|none` or documented specific-function object), and `response_format` exactly as documented. Explicitly excluded/blocked Chat request fields are any OpenAI-compatible field not named above and provider-specific nested options other than enumerated Gateway routing fields; no generic bag is accepted.
- [x] **Exact exported success representation from the two first-party response fixtures:** use `type JSONField[T any] struct { Present bool; Null bool; Value T }` for every documented field because the fixtures show wire types and nullable `content` but do not explicitly state requiredness for any success member. Invariant: absent is `{Present:false, Null:false, zero Value}`; explicit null is `{Present:true, Null:true, zero Value}`; a non-null value is `{Present:true, Null:false, Value:v}`; any other combination is impossible from decoding. Export `type ChatCompletionResult struct { ID JSONField[string]; Object JSONField[string]; Created JSONField[int64]; Model JSONField[string]; Choices JSONField[[]ChatChoice]; Usage JSONField[ChatUsage]; rawJSON []byte }` with `func (r *ChatCompletionResult) RawJSON() []byte`; the method returns nil on a nil receiver and otherwise a fresh non-nil defensive copy of the complete bounded status-200 JSON value. Export `type ChatChoice struct { Index JSONField[int]; Message JSONField[ChatAssistantMessage]; FinishReason JSONField[string] }`; `type ChatAssistantMessage struct { Role JSONField[string]; Content JSONField[string]; ToolCalls JSONField[[]ChatToolCall] }`; `type ChatToolCall struct { ID JSONField[string]; Type JSONField[string]; Function JSONField[ChatFunctionCall] }`; `type ChatFunctionCall struct { Name JSONField[string]; Arguments JSONField[string] }`; `type ChatUsage struct { PromptTokens JSONField[int64]; CompletionTokens JSONField[int64]; TotalTokens JSONField[int64] }`. Evidence is the ordinary “Response format” fixture in <https://vercel.com/docs/ai-gateway/sdks-and-apis/openai-chat-completions/chat-completions> and the “Tool call response format” fixture in <https://vercel.com/docs/ai-gateway/sdks-and-apis/openai-chat-completions/tool-calling>, both `last_updated: 2026-09-08`, inspected 2026-09-21.
- [x] **Field-by-field presence/null policy:** no documented Chat success member is locally required solely because it appears in an example fixture; absence and explicit null are preserved distinctly by `JSONField` for every field above. When present and non-null, `id`, `object`, `model`, `finish_reason`, message `role`/`content`, tool-call `id`/`type`, function `name`/`arguments` are strings; `created`, indexes, and usage counts are integers representable by their Go types. When present and non-null, `choices` and `tool_calls` are arrays (with `[]` retained as non-null empty slices), while `usage`, message, function, and choice elements are objects. The two fixtures establish that `content` may be a string or null and that `tool_calls` may be absent or an array; the generic presence form additionally preserves explicit null without claiming Vercel guarantees it. Validate `object == "chat.completion"`, message `role == "assistant"`, and tool-call `type == "function"` only when each discriminator is present and non-null because those exact discriminator values are stated by the named response formats. Do not enforce sign/range constraints on `created`, indexes, or usage counts, a usage-sum equation, a minimum choice count, nonempty strings, or a closed `finish_reason` enum because the cited Vercel pages state none; preserve every representable integer/string. A present null object/array/string/integer field is retained as null, not rejected, unless future first-party schema evidence expressly forbids null for that field.
- [x] **Exact decoder/forward-compatibility policy:** accept one JSON value plus trailing whitespace only; reject malformed JSON, trailing values, duplicate keys at every depth, a null/non-object top level, depth over 64, any string/byte value over 1 MiB, or any array/object over 10,000 members. Decode the documented fields above with their exact presence/null/type rules. Because the cited examples are not an exhaustive schema, do not infer that additional members are invalid: accept unknown fields at the top level and within choice, message, tool-call, function, and usage objects after applying the same recursive bounds and duplicate-key checks, preserve them byte-for-byte only through `RawJSON`, and do not interpret, normalize, or expose them as typed support. Unknown-field rejection at any fixed Chat DTO level may replace this policy only when a versioned/current first-party Vercel schema, released source/test, or equally exhaustive contract explicitly closes that object's field set; evidence must be assessed separately for each object level. An unrecognized present non-null `object`, role, or tool-call `type` is a stable response-validation error because the named response formats state those discriminator values; an unrecognized `finish_reason` remains a string. New typed response members or message/tool variants remain individually `[!]` until exact first-party evidence defines their path, wire type, requiredness, nullability, nesting, and discriminator; accepting them as bounded raw JSON does not claim typed support.
- [x] Keep Chat message/tool/response types distinct from Responses input/output types even where names resemble one another. The SDK exposes ordinary function calls but never executes them; `arguments` remains the exact JSON-encoded string shown by first-party evidence and is not parsed automatically. No lossy cross-surface converter is part of the public API. Apply the same context, body-bound, authentication, safe error, and opt-in non-stream retry rules established earlier.
- [x] Enforce in this chunk, before network or excessive allocation: maximum JSON nesting depth 64 for messages, content parts, schemas, routing data, and success JSON; maximum 1 MiB per string/byte value; maximum 10,000 members per array/object; maximum 10,000 messages, content parts, tools, choices, and tool calls; and the existing 1 MiB buffered success/error-body ceiling. Limit-plus-one cases return stable typed validation/response errors. Do not defer these surface bounds to Chunk 21.

**Acceptance commands:**

```sh
env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go test -run 'Test(CreateChatCompletion|ChatRequest|ChatResult)' ./...
env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go test -race -run 'Test(CreateChatCompletion|ChatRequest|ChatResult)' ./...
env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go test ./...
```

**Recorded evidence:**

- [x] Initial post-fix focused tests failed because the new assertions expected non-canonical validation path strings and rejected the legitimate `read response body` transport operation label; the test expectations were corrected to the established contracts before final verification.
- [x] Post-correction `gofmt` and `git diff --check` passed.
- [x] `env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go test -run 'Test(CreateChatCompletion|ChatRequest|ChatResult)' ./...` passed.
- [x] `env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go test -race -run 'Test(CreateChatCompletion|ChatRequest|ChatResult)' ./...` passed.
- [x] `env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go test ./...` passed.
- [x] `go vet ./...` passed.
- [x] Workspace diagnostics reported no issues.

**Review/blocker accounting:**

- [x] Independent review checks every result field and presence/null/unknown/duplicate rule against the two cited Vercel fixtures, verifies optional `tool_calls` and nullable `content` remain distinct, and rejects accidental Responses/provider-protocol fields.
- [x] **Block a specific compatibility field or variant** when Gateway does not document it; compatibility with another vendor alone is not evidence, and unsupported variants fail stably rather than being discarded or normalized.
- [x] Review fix implemented and verified: `providerOptions.gateway.providerTimeouts.byok` values are validated before network against the documented inclusive `1,000..789,000` millisecond range.
- [x] Review fix implemented and verified: function-tool `parameters` and `response_format.json_schema.schema` require JSON Schema objects rather than bounded scalar or array JSON.
- [x] Review fix implemented and verified: a non-nil `ProviderTimeouts` with nil `BYOK` no longer encodes `providerTimeouts.byok:null`; the documented wire value remains an object map.
- [x] Review fix implemented and verified: an oversized HTTP 200 success body retains the established response-validation error classification rather than being surfaced as a transport/read error.
- [x] Clean independent re-review confirms all four findings are fixed and the focused, race, full, formatting, diff, and diagnostics evidence is current.
- [x] Final split review found that request JSON depth counted Go interface/pointer representation as JSON nesting, rejecting valid values below the documented 64-container boundary. The validator now increments JSON depth only for maps/arrays, retains a separate bounded indirection guard and cycle detection, and focused coverage proves 64 JSON container levels are accepted while 65 are rejected.
- [x] The final depth fix and boundary regression were included in the final orchestrator verification described after Chunk 23; Chunk 18 remains accepted on hermetic evidence, without changing any live or evidence-gated field status.

### Chunk 19 — Chat Completions streaming

**Depends on:** Chunk 18 `[x]`.

**Owned files:** `chat_stream.go`, `sse.go`, `chat_stream_test.go`, `internal/httpx/body.go`, `contract_external_test.go`.

**Commit boundary:** `feat: stream chat completions`

- [x] Add `Client.StreamChatCompletion` using the shared bounded SSE framer. Type only the first-party-proven `chat.completion.chunk` text path (`choices[].delta.content`) and terminal `data: [DONE]` from <https://vercel.com/docs/ai-gateway/sdks-and-apis/openai-chat-completions/chat-completions> (`last_updated: 2026-09-08`). Preserve the chunk discriminator and bounded defensive-copy JSON for untyped fields; do not claim or assemble refusal fragments, tool-call fragments, streamed usage, finish-reason deltas, or any other undocumented field.
- [x] Enforce in this chunk the 64 KiB line, 1 MiB assembled-event, 1 MiB terminal-diagnostic, 10,000-event, 64-level JSON-depth, 1 MiB per decoded string/byte value, and 10,000 members per decoded array/object limits, plus caller-close, cancellation, no-background-goroutine, and no-post-header-retry rules.
- [x] Treat `[DONE]` as required successful termination and EOF before it as a stream error. Ignore SSE comments/keepalives without losing cancellation responsiveness. Preserve event and choice order; the SDK executes and assembles no tool calls.
- [!] Refusal fragments, tool-call fragment fields/assembly, streamed usage, finish-reason deltas, and any other additional chunk field are individually `[!]` until an exact versioned first-party Vercel Chat page/OpenAPI/released source/test defines their wire names, types, presence/nullability, ordering/assembly rules, unknown-field policy, and termination effect. OpenAI documentation alone is insufficient.

**Acceptance commands:**

```sh
env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go test -run 'Test(StreamChatCompletion|SSE)' ./...
env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go test -race -run 'Test(StreamChatCompletion|SSE)' ./...
env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go test ./...
```

**Recorded implementation verification evidence:**

- [x] Implementation commit recorded as `d9629d5`; review-fix commit recorded as `8dad38f`.
- [x] `gofmt` completed for the implemented files.
- [x] `git diff --check` passed.
- [x] `env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go test -run 'Test(StreamChatCompletion|SSE)' ./...` passed.
- [x] `env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go test -race -run 'Test(StreamChatCompletion|SSE)' ./...` passed.
- [x] `env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go test ./...` passed.
- [x] `go vet ./...` passed.
- [x] Workspace diagnostics reported no issues.

**Review/blocker accounting:**

- [x] Independent review checks `[DONE]`, multiple documented text choices, cancellation, close/body ownership, originating limits, bounded raw-field preservation, and no event-schema sharing with Responses beyond framing.
- [x] Review fix implemented and verified: removed unsupported typed `choices[].index`; the field remains available only through bounded defensive-copy raw chunk JSON unless exact first-party evidence clears it.
- [x] Review fix implemented and verified: replaced the nondeterministic process-wide goroutine-count regression with deterministic stream-owned lifecycle proof.
- [x] Three clean independent re-reviews confirm both review fixes and the current focused, race, full, vet, formatting, diff-check, and diagnostics evidence.
- [x] **Block every undocumented chunk-field interpretation** behind the exact first-party evidence gate above; retaining bounded raw chunk bytes is not a typed support claim.

### Historical evidence-gated continuation item — Chat Gateway server search tools (superseded for current request disposition)

**Historical prospective commit boundary:** `feat: add chat gateway search tools`

- [!] **Historical `vercel:exa_search` gate:** this former all-or-nothing request/output gate is superseded. Current request-only disposition and exact API are in Chunk 25 below; typed outputs remain blocked independently.
- [!] **Historical `vercel:parallel_search` gate:** superseded for request serialization only; typed outputs remain blocked independently.
- [!] **Historical `vercel:perplexity_search` gate:** superseded for request serialization only; typed outputs remain blocked independently.
- [!] **Historical `vercel:tako_search` gate:** superseded for request serialization only; typed outputs remain blocked independently.
- [!] **Historical blocker at Chunk 24 closure:** the Chat tool-calling page used then defined only ordinary `type:"function"` tools. This statement is retained as history, but its current-status effect is superseded by the later first-party Gateway web-search page cited in the authoritative continuation: the four request declarations are now plan-ready; typed outputs remain blocked.
- [!] **Historical prospective ownership, not current authority:** the authoritative current Chunk 25 below uses one explicit seven-file atomic implementation-and-documentation boundary.

### Blocked continuation item — Gateway-native xAI `x_search`

- [!] Do not implement Gateway-native `x_search`. **Stage 1 — schema evidence:** first obtain a versioned first-party Vercel Gateway document, OpenAPI/schema, or released `@ai-sdk/gateway` source/test that shows the exact `/v1/responses` request JSON, parameter names/limits, compatible Gateway model identifier, and native `x_search_call` response/event item. Direct `api.x.ai/v1/responses`, `@ai-sdk/xai`, generic/provider-native web-search availability, generated prose, or another provider's schema cannot clear this stage.
- [!] **Stage 2 — prospective probe implementation, permitted only after Stage 1 clears:** add a minimal build-tagged paid probe and its fail-closed sentinel/workflow support in `internal/livecontract/x_search_test.go`, `internal/livecontract/live_test.go` if shared helpers require change, `.github/workflows/live-contract.yml`, and `docs/evaluation-live-evidence.md`. The probe may encode only the Stage-1-proven schema/model, must require exactly one authorized Gateway credential and sole sentinel `AI_GATEWAY_X_SEARCH_LIVE_COST_ACK=I_ACCEPT_LIVE_X_SEARCH_COSTS`, reject missing/extra live acknowledgements, and retain no query/result/headers/body/credential. Adding this probe is evidence acquisition, not feature implementation or a support claim.
- [!] **Stage 3 — paid live-result gate:** an owner-authorized protected-environment run of `env -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK AI_GATEWAY_X_SEARCH_LIVE_COST_ACK=I_ACCEPT_LIVE_X_SEARCH_COSTS go test -tags=livecontract ./internal/livecontract -run TestGatewayNativeXSearchContract` must return a native structured `x_search_call`. Record only schema/source version, approved model, structured item types, sanitized IDs, status, and date in the owned evidence file. Only after Stages 1–3 pass may a separate implementation chunk be planned; the live probe itself never ships an exported x_search API.

### Chunk 21 — Unified error, cancellation, and resource-limit audit

**Depends on:** Chunks 15–16 and 18–19 `[x]`; evidence-gated public Evaluate and search items may remain `[!]`.

**Owned files (justified twelve-file exception):** `errors.go`, `response_error.go`, `transport.go`, `sse.go`, `resource_limits_test.go`, `chat.go`, `responses.go`, `chat_stream.go`, `responses_stream.go`, `evaluation_response.go`, `evaluation_response_test.go`, `evaluation_validate_test.go`. The four public generation files are included because every buffered and streaming call path must share pre-send cancellation and cancellation-aware error-body lifecycle behavior. The Evaluation response decoder and its focused tests are included because the all-surfaces allocation audit found that its response-tree parsing did not enforce the shared depth, collection-member, string-value, or object-key bounds. The Evaluation validation tests are included because certification found that caller-controlled validation paths could make `ValidationError.Error()` disclose input content and grow without a diagnostic bound.

**Commit boundary:** `fix: harden public api transport lifecycle`

- [x] Audit every implemented surface for typed `errors.As` behavior, nil receiver safety, response/request IDs, Retry-After, bounded defensive raw bodies, and formatting that never reveals request bodies, prompts, tool arguments/results, headers, credentials, provider options, or streamed payloads.
- [x] Define exported stream error behavior without weakening existing five evaluation error contracts. The existing typed transport error contract represents stream-read failures honestly without exposing wrapped payload text, so no new exported stream error type was required.
- [x] Prove cancellation before send, while waiting for headers, during buffered body read, while blocked between SSE events, and during `Close`. Prove bodies close once and no retry/replay occurs after response receipt.
- [x] Migrate every buffered public call site in `chat.go` and `responses.go` to the shared cancellation-aware, exactly-once body helper. Acceptance requires Chat and Responses, as well as the already-covered buffered surfaces, to unblock body reads on context cancellation, retain cancellation in the returned error chain, close each response body exactly once under read/cancel/close races, and perform no retry or replay after response receipt.
- [x] Audit—not introduce or postpone—the exact depth, item, string, collection, buffered-body, SSE-line, event, diagnostic, and event-count limits already owned by Chunks 15–16 and 18–19 (and by public Evaluate if separately unblocked). Centralize constants only when doing so preserves each originating chunk's committed behavior and tests. Verify every allocation path fails before exceeding its bound and uses stable typed reasons.
- [x] Fuzz or table-test SSE framing and strict JSON decoders with bounded corpora; tests must remain deterministic, hermetic, and fast.

**Audit findings and implemented fixes:**

- [x] All four public generation paths now apply an explicit pre-send context guard, so an already-canceled context terminates before transport setup/send; deterministic coverage verifies zero transport calls.
- [x] The Chat Completions and Responses streaming non-200 paths now use the shared cancellation-aware, exactly-once body helper; deterministic coverage verifies cancellation unblocks the read, cancellation remains in the error chain, the body closes once, and no retry/replay occurs after response receipt.
- [x] Evaluation success response-tree parsing and the best-effort response-ID parser now enforce the shared maximum nesting-depth, array/object-member, string-value, and object-key-size bounds before excessive allocation; focused exact-boundary and limit-plus-one coverage verifies stable rejection.
- [x] Certification found that `ResponseValidationError.Error()` formatted the full attacker-controlled `Path()`, allowing an exact-limit unknown Evaluation response key to disclose response content and create an approximately 1 MiB diagnostic. The verified fix preserves the full stable machine-readable `Path()` while `Error()` emits a stable bounded redaction; an exact-limit unknown top-level key containing a secret marker proves the formatted error neither exposes the marker nor grows with attacker-controlled path content.
- [x] Certification found that `ValidationError.Error()` formatted its full caller-controlled `Path()`, allowing request input content to be disclosed and producing an unbounded diagnostic. The verified fix preserves the complete stable machine-readable `Path()` accessor while `Error()` uses stable bounded/redacted formatting; oversized secret-marker question and provider-key regressions prove the diagnostic neither exposes the markers nor grows with caller-controlled path content, and validation still performs zero network dispatch.
- [x] Chunk 21 is accepted after complete independent certification and clean re-review; its dependency for Chunk 22 is satisfied. Unrelated global release blockers and evidence-gated items remain governed by their own tracker rows.
- [x] Final split review found that limit-plus-one successful Evaluation and Responses bodies were classified as `TransportError` because the generic body error was checked before truncation. Both buffered paths now classify status-200 overflow as `ResponseValidationError` at `$` with reason `response body exceeds 1 MiB`, `BodyTruncated()==true`, the bounded retained prefix, and the overflow/read/close cause; focused public regressions cover both surfaces.
- [x] The first final full and race runs after this source fix failed only because `retry_test.go` still expected the obsolete status-200 `TransportError` classification. The stale expectation was corrected to the new typed response-validation contract while preserving one-attempt/no-retry and the unchanged read/close and non-200 overflow behavior. Subsequent full and race reruns passed as recorded after Chunk 23.

**Acceptance commands:**

```sh
env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go test -run 'Test(ResourceLimits|Cancellation|ErrorFormatting|BodyClosure)' ./...
env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go test -race -run 'Test(ResourceLimits|Cancellation|BodyClosure)' ./...
env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go test ./...
```

**Recorded implementation verification evidence:**

- [x] Shared cancellation-aware, exactly-once body reads now cover Evaluation, Chat Completions, and Responses across buffered and streaming non-200 paths, and all four public generation paths reject pre-canceled contexts before transport use.
- [x] Non-200 responses preserve status, structured error fields, generation/request/response IDs, parsed Retry-After, retry classification, truncation state, bounded defensive raw bytes, and safe formatting that omits caller and upstream payloads.
- [x] Shared SSE framing retains the 64 KiB line and 1 MiB assembled-event limits while explicitly capping retained event allocation at 1 MiB; deterministic limit/format/body-closure/retry coverage includes exact-boundary and limit-plus-one behavior, fragmented reads, cancellation, safe diagnostics, and no post-response replay.
- [x] Evaluation response-tree decoding, including best-effort response-ID extraction, enforces shared depth, array/object-member, string-value, and object-key-size bounds with exact-boundary and limit-plus-one coverage.
- [x] Post-all-fix `gofmt` completed for the implemented files and `git diff --check` passed on the current workspace.
- [x] Post-all-fix `env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go test -run 'Test(ResourceLimits|Cancellation|ErrorFormatting|BodyClosure)' ./...` passed on the current workspace.
- [x] Post-all-fix `env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go test -race -run 'Test(ResourceLimits|Cancellation|BodyClosure)' ./...` passed on the current workspace.
- [x] Post-all-fix `env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go test ./...` passed on the current workspace.
- [x] Post-all-fix `env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go vet ./...` passed on the current workspace.
- [x] Post-all-fix workspace Go diagnostics reported no issues.
- [x] Post-commit verification at HEAD `3ebb25f`, after implementation commits `131edb6` and `026b09a`: `env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go test -run 'Test(ResourceLimits|Cancellation|ErrorFormatting|BodyClosure)' ./...` passed.
- [x] Post-commit verification at HEAD `3ebb25f`: `env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go test -race -run 'Test(ResourceLimits|Cancellation|BodyClosure)' ./...` passed.
- [x] Post-commit verification at HEAD `3ebb25f`: `env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go test ./...` passed.
- [x] Post-commit verification at HEAD `3ebb25f`: `env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go vet ./...` passed.
- [x] Post-commit verification at HEAD `3ebb25f`: workspace Go diagnostics reported no issues.
- [x] Post-formatter-fix `gofmt` completed for the implemented files and `git diff --check` passed.
- [x] Post-formatter-fix exact focused tests, including the exact-limit secret-marker regression, passed.
- [x] Post-formatter-fix focused race tests passed.
- [x] Post-formatter-fix full tests passed.
- [x] Post-formatter-fix `go vet ./...` passed.
- [x] Post-formatter-fix workspace Go diagnostics reported no issues.
- [x] Post-commit verification at HEAD `37fa6de`: `env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go test -run 'Test(ResourceLimits|Cancellation|ErrorFormatting|BodyClosure)' ./...` passed.
- [x] Post-commit verification at HEAD `37fa6de`: `env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go test -race -run 'Test(ResourceLimits|Cancellation|BodyClosure)' ./...` passed.
- [x] Post-commit verification at HEAD `37fa6de`: `env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go test ./...` passed.
- [x] Post-commit verification at HEAD `37fa6de`: `env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go vet ./...` passed.
- [x] Post-commit verification at HEAD `37fa6de`: workspace Go diagnostics reported no issues.
- [x] Post-`ValidationError`-fix `gofmt` completed for the implemented files and `git diff --check` passed.
- [x] Post-`ValidationError`-fix focused tests, including oversized secret-marker question/provider-key regressions and zero network dispatch, passed.
- [x] Post-`ValidationError`-fix focused race tests passed.
- [x] Post-`ValidationError`-fix full tests passed.
- [x] Post-`ValidationError`-fix `go vet ./...` passed.
- [x] Post-`ValidationError`-fix workspace Go diagnostics reported no issues.
- [x] Post-commit verification at HEAD `705d359`: `env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go test -run 'Test(ResourceLimits|Cancellation|ErrorFormatting|BodyClosure)' ./...` passed.
- [x] Post-commit verification at HEAD `705d359`: `env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go test -race -run 'Test(ResourceLimits|Cancellation|BodyClosure)' ./...` passed.
- [x] Post-commit verification at HEAD `705d359`: `env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go test ./...` passed.
- [x] Post-commit verification at HEAD `705d359`: `env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go vet ./...` passed.
- [x] Post-commit verification at HEAD `705d359`: workspace Go diagnostics reported no issues.
- [x] Full finding-to-fix commit chain: lifecycle/cancellation/SSE/error hardening in `131edb6`, Evaluation response allocation bounds in `026b09a`, combined verification at HEAD `3ebb25f`, bounded/redacted `ResponseValidationError` formatting verified at HEAD `37fa6de`, and bounded/redacted `ValidationError` formatting plus final twelve-file verification at HEAD `705d359`.

**Review/blocker accounting:**

- [x] Independent security/resource review recorded every limit, allocation path, close owner, retry boundary, formatter surface, and finding/fix/re-review outcome.
- [x] Complete independent clean re-review confirmed the cancellation fixes, Evaluation response-tree allocation fix, both bounded/redacted validation formatter fixes, and the complete twelve-file audit with no remaining implementation issue.
- [x] Chunk-specific release blocker cleared: no stream goroutine/body leak, unbounded line/event/body or validation-diagnostic allocation, cancellation-blind read, or formatted caller/upstream payload disclosure remains. Unrelated global release blockers and evidence-gated items are unchanged.

### Chunk 22 — Hermetic contract fixtures and gated live coverage

**Depends on:** Chunk 21 `[x]`.

**Owned files:** `internal/livecontract/public_test.go`, `internal/livecontract/evaluation_test.go`, `internal/testserver/server.go`, `hermetic_transport_test.go`, `.github/workflows/live-contract.yml`, `docs/evaluation-live-evidence.md`.

**Commit boundary:** `test: add public api contract gates`

**Hermetic status (independently trackable):**

- [x] **Historical Chunk 22 Hermetic scope:** fixtures covered Responses non-stream/stream and Chat non-stream/stream; public Evaluate and every search schema were excluded because none had cleared at that time. `TestMain` scrubbed both credentials and both live acknowledgements, installed a loopback-only default transport, and targeted coverage proved all four default public generation paths fail closed on non-loopback destinations. The later Chat request-only plan does not alter this completed historical evidence.
- [x] Static inspection of the six owned-file changes confirmed manual/scheduled-only protected-environment workflow configuration, single-credential selection, secret boundaries, isolated acknowledgements, and sanitized-evidence rules. Review found that the public live prerequisite gate used `t.Skip`, which could report a credential-free invocation as a passing package; all three public acknowledgement/credential rejection paths were corrected to `t.Fatalf`, and the fail-closed correction is verified.
- [x] Post-correction orchestrator evidence records gofmt/diff-check success, workspace diagnostics with no issues, exact env-scrubbed `go test ./...` success, and exact env-scrubbed `go test -race ./...` success with `AI_GATEWAY_API_KEY`, `VERCEL_OIDC_TOKEN`, `AI_GATEWAY_LIVE_COST_ACK`, and `AI_GATEWAY_PUBLIC_LIVE_COST_ACK` unset. The credential-free targeted command `env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go test -tags=livecontract ./internal/livecontract -run '^TestGateway(Responses|Chat)Contract$' -count=1` failed at the exact public acknowledgement gates before network access. This verifies fail-closed behavior only and is **NOT A LIVE RUN** or live success.
- [x] Independent Hermetic acceptance is complete. Three independent re-reviews returned **CLEAN** after the public live-gate finding was corrected; review accounting covers the six-file implementation, workflow/secret boundaries, exact acknowledgement and credential gates, fixture/non-loopback behavior, command evidence, and the finding-to-fix clean re-review. Implementation commit `b01c240` and fail-closed correction commit `c0892d1` form the recorded change chain. Chunk 22 Hermetic status is `[x]`, satisfying the dependency for Chunk 23; paid Live status remains separately `[!]` and conveys no success claim.
- [x] Final split review found the paid public live gate too weak to support its intended sanitized evidence: it accepted arbitrary buffered raw JSON or any stream item without proving pinned-model identity, buffered Chat finish reasons, buffered Responses output-item structure, or a terminal Responses completion event. The build-tagged test gate now requires exact pinned model identity wherever exposed, nonblank buffered Chat finish reasons, nonempty buffered Responses output whose test-only minimal structural decode finds a nonblank `type` discriminator on every item, and a terminal `response.completed` event whose payload exposes the pinned model. This assertion does not add typed SDK output items: the public buffered Responses contract remains raw-only through `ResponseResult.RawJSON`. The evidence template remains sanitized and retains structural facts only.
- [x] This is a static/hermetic gate correction only. No paid live command ran, no live success is claimed, and every Chunk 22 Live row below remains `[!]` pending owner-authorized protected execution, retained sanitized evidence, and independent Live review.

**Live status (independently trackable release gate):**

- [!] Public-generation paid live success smoke: **NOT RUN**. Build-tagged tests exist for the already implemented Responses non-stream/stream and Chat non-stream/stream text paths, but execution remains blocked on explicit owner authorization for paid network use, exactly one authorized production credential, approval of the pinned `openai/gpt-5-nano` model, an approved protected environment, and sanitized retained evidence. At this historical Chunk 22 gate, Public Evaluate and Responses/Gateway-native search were inapplicable because their gates had not cleared. Later request-only support does not retroactively add those declarations to this live gate; any future corroboration must be planned separately.
- [!] Provider-evaluation paid live success smoke: **NOT RUN**. The preserved build-tagged contract remains blocked on explicit owner authorization for paid network use, exactly one authorized production credential, approval of the pinned `typesafe-ai/jev` model, an approved protected environment, and sanitized retained evidence. The credential-free invocation failed at the exact acknowledgement gate before network and is fail-closed proof, not live success.
- [!] Paid live workflow execution and evidence review: **NOT RUN**. Static workflow code is manual/scheduled rather than pull-request-triggered; each protected job requires its sole exact acknowledgement, rejects the opposite acknowledgement at any value, and requires exactly one nonblank credential. Both build-tagged gates implement the same verified fail-closed policy and pass only the selected credential to the client. Credential-free gate failures occurred before network and are not paid-live evidence. Completion remains blocked on owner authorization, one authorized credential, approved pinned models, protected-environment execution, sanitized evidence, and independent Live review accounting.
- [!] Mark this Live status `[x]` only after every applicable paid command passes with sanitized evidence and independent review accounting. **NOT RUN:** no paid command has executed. Chunk 24 and release remain blocked until then; Hermetic completion does not imply a live pass.

### Chunk 23 — Documentation, examples, migration, and public API audit

**Depends on:** Chunk 22 Hermetic status `[x]`; Chunk 22 Live may remain explicitly `[!]`, but release and Chunk 24 remain blocked.

**Owned files:** `README.md`, `doc.go`, `docs/evaluation.md`, `docs/generation.md`, `examples/evaluate/main.go`, `examples/generate/main.go`, `scripts/verify-local-consumer.sh`, `CHANGELOG.md`, `client.go` (documentation/API-comment chunk exception: nine files).

**Commit boundary:** `docs: document public generation apis`

- [x] Update the support matrix to distinguish `/v4/ai/evaluation-model`, `/v1/evaluate`, `/v1/responses`, `/v1/chat/completions`, unsupported `/v4/ai/language-model`, and all three evidence-gated search categories: Responses built-in search, Chat Gateway server search tools, and Gateway-native `x_search`. Do not list a blocked search feature as supported.
- [x] Document authentication/base URL compatibility, the continued meaning of `WithBaseURL`, new `WithPublicBaseURL`, retries, cancellation, stream ownership/limits, errors, privacy warnings, tool non-execution, the complete surface-specific parameter inventories/blockers above, and separate Hermetic versus Live evidence status.
- [x] Add runnable credentialed examples for non-stream and streaming Responses and Chat without logging raw prompts, tool arguments/results, response bodies, or secrets. Add a distinct `Client.EvaluatePublic` example only if that evidence-gated item has cleared and been implemented; otherwise document `/v1/evaluate` as blocked on its public success-schema gate. Existing provider-protocol `Client.Evaluate` remains distinct and unchanged.
- [x] Extend the external-consumer audit to instantiate every implemented exported request/result/function-tool/stream/error type and method. Include `EvaluatePublic` only if its gate cleared. Fail on accidental generic generation/tool APIs, `/v4` language-model APIs, blocked public-evaluation/search exports, compatibility aliases, or removed existing exports.
- [x] Add a migration section stating the change is additive, existing `Evaluate`/`WithBaseURL` behavior is unchanged, and callers choose public surfaces explicitly. Document no automatic conversion between Responses and Chat types.

**Acceptance commands:**

```sh
env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go test ./...
env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go test ./examples/...
env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go doc -all .
env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK ./scripts/verify-local-consumer.sh
```

- [x] Final post-fix implementation verification evidence: gofmt/diff-check passed, and workspace diagnostics reported no issues across the nine owned files.
- [x] After the shared `TokenSource` and `RetryPolicy` multi-surface comment corrections, the exact env-scrubbed `go test ./...` command passed; the exact env-scrubbed `go test ./examples/...` command passed; and the exact env-scrubbed `go doc -all .` command completed.
- [x] After those shared-comment corrections, the exact env-scrubbed `./scripts/verify-local-consumer.sh` command passed. The audit also retains the earlier corrections for legitimate exported `Path` and `MarshalJSON` symbols, internal receiver noise, and the `RetryAfter` arity.
- [!] Paid Live status remains **NOT RUN**. No Chunk 23 documentation, example, hermetic test, documentation command, or external-consumer result substitutes for Chunk 22 Live evidence.

- [x] The initial review findings were fixed and verified: the support matrix now distinguishes the required endpoint and search categories; the generation example no longer logs unsanitized identifiers; and the external-consumer method audit uses receiver-qualified method identity rather than method name alone.
- [x] The further review finding that the evaluation example and README could emit sensitive evaluation output was fixed by sanitizing that output, and the fix is verified.
- [x] The further review finding that the supposedly safe `docs/evaluation.md` logging snippet emitted the raw `RequestID` was fixed with a presence boolean, and the fix is verified.
- [x] The further review finding of raw model-output printing in `docs/generation.md` was fixed with sanitized structural output, and the correction is verified.
- [x] Clean certification found contradictory Unreleased support and non-goal claims in `CHANGELOG.md`. The contradiction was corrected within the expanded documentation scope, and the generation-support correction is verified.
- [x] Closure certification found a stale exported `Client` comment in `client.go` that still described the client as evaluation-only. The comment was corrected within the expanded nine-file documentation/API-comment scope, and the fix is verified.
- [x] Closure certification also found raw `ResponseValidationError.Path()` logging in both runnable examples. Both examples now report only path presence, and the privacy-safe corrections are verified.
- [x] A further closure review found stale evaluation-only comments on exported `TokenSource` and `RetryPolicy`. The multi-surface documentation corrections are implemented and verified.
- [x] The complete eight-commit implementation/finding/fix chain is recorded at HEAD `eddc0d5a3a70f1dca1f29e65dbe7bf467404a16b`: `4fc85054db18493f7530e57d795f15848eda923b` (`docs: document public generation apis`), `c83ba7fc503add9e1b94443a40d60a5a9cc81420` (`fix: correct public api documentation`), `64dbe9f82fa7b86cedc3d079cf386426cf78aa28` (`fix: sanitize sdk examples`), `e14e2a95b7c9c83960e54be6b5299b67214ba2da` (`fix: sanitize evaluation documentation`), `7ed90a6e96971940c09c129fbea48a7482d23b61` (`fix: sanitize generation documentation`), `05c8f874cbd0d4940cfcba4e884b4a0d40230a48` (`docs: update generation changelog`), `d456e269cabf884581711027c931b2538f98af45` (`fix: align public client documentation`), and `eddc0d5a3a70f1dca1f29e65dbe7bf467404a16b` (`docs: clarify shared client contracts`). Two independent definitive certifications returned **CLEAN** over the nine-file scope, the complete finding-to-fix chain, the final exact evidence, and the blocked Live boundary.

**Review/blocker accounting:**

- [x] Independent documentation/API review maps every support claim and runnable example—including the distinct blocked `EvaluatePublic` path—to an implemented hermetic contract; two definitive certifications returned **CLEAN**, and paid-live claims remain separately labeled pending until Chunk 22 Live evidence exists.
- [x] **Block completion** for undocumented exports, examples that can make accidental live calls without explicit credentials, stale evaluation-only claims presented as current scope, conflation of the two evaluation protocols, or any blocked-search support implication. Both definitive certifications found none remaining after the recorded fixes.

### Final split-review reconciliation before Chunk 24

- [x] Five-way final review was recorded. One review of Chunks 13/15/16 found no actionable defect and confirmed the distinct provider/public base URLs, raw-only Responses evidence boundaries, stream lifecycle contracts, and accurate paid-live blockers. The other reviews reported four Important findings: Chat request JSON depth counted Go indirections as JSON levels; successful oversized Evaluation/Responses bodies used the wrong typed error; the public paid-live gate did not prove enough pinned-model/terminal/finish-reason structure; and tracker/release material still asserted a disproven local-origin blocker and an evaluation-only release boundary.
- [x] All four findings were fixed and committed in final split-review fix commit `449aed2`. Chunk 18 owns the exact 64/65 JSON-container correction; Chunk 21 owns status-200 overflow classification and regressions; Chunk 22 Hermetic owns the strengthened build-tagged public-live assertions while Chunk 22 Live remains `[!]`; Chunk 23 documentation/release guidance now covers both public-generation and provider-evaluation surfaces and records the matching local origin without inferring hosted state.
- [x] Orchestrator evidence after the fixes records `gofmt` and diff-check success, targeted final-fix test success, examples/API-audit/livecontract compile success, and workspace diagnostics with no issues. `go vet ./...` passed before the later test-only retry expectation correction; that correction changed no production code.
- [x] The first env-scrubbed full `go test ./...` and `go test -race ./...` runs failed because `retry_test.go` retained the old status-200 overflow expectation. After correcting that stale test to require `ResponseValidationError` and the bounded truncation contract, both complete env-scrubbed commands were rerun successfully. These are hermetic results only.
- [x] Local repository configuration is no longer a rename blocker: orchestrator evidence records `origin` fetch and push as `git@github.com:EveGoodEvening/vercel-ai-go-sdk.git`. This does not verify hosted repository existence/metadata, license, hosted CI, provenance, authorization, immutable tags, direct-VCS/proxy publication, checksums, or paid live contracts; those release gates remain open.
- [!] **Historical closure statement, superseded for current planning by the 2026-09-21 search-request continuation below.** At Chunk 24 closure, public `POST /v1/evaluate`, Responses built-in search, every Chat Gateway server-search identifier, and Gateway-native `x_search` were absent and evidence-gated. The later continuation now supports the four Chat request declarations plus fixed low-context Responses `web_search` and fieldless Gateway `x_search` through their opt-in wrappers. Public Evaluate; configurable search options; `web_search_preview` and other forms; typed outputs/events; wider model compatibility; direct-xAI; and unrelated surfaces remain blocked.
- [!] No paid public-generation or provider-evaluation live run occurred. Chunk 24 and release remain incomplete and blocked on every applicable Live and external publication gate.
- [x] Cleanup commit/current HEAD at the substantive closure review, `ee41722`, contains the final rereview corrections after `449aed2`: removal of the stale uncommitted-workspace statement, removal of the unsupported typed buffered-Responses implication while preserving `ResponseResult.RawJSON` as the SDK contract and test-only minimal structural decoding, and `docs/x-search.md` continuation-release wording that covers provider Evaluation plus buffered/streaming Responses and Chat. Closure review `final-closure-review-1` independently verified all three substantive corrections and returned **CLEAN**. Closure review `final-closure-review-2` confirmed the substantive blocker boundaries and found only tracker accounting/local-checkbox closure missing; accounting fix commit `3545fac` plus this closure evidence records that finding as fixed. Every local/doable continuation item is `[x]`, and every incomplete item is `[!]` with its exact external or explicitly deferred blocker; no paid-live, hosted, owner, tag, provenance, proxy, checksum, or publication gate is cleared.

### Chunk 24 — Continuation release verification

**Depends on:** required Chunks 13, 15–16, 18–19, 21, and 23 `[x]`; Chunk 22 Hermetic and Live statuses `[x]` for every implemented surface; and the still-relevant external prerequisites preserved from Chunks 09/11/12—current upstream evidence, authorized sanitized evaluation live evidence, owner-selected license, configured repository/remote metadata, hosted CI, provenance, clean-consumer verification, checksum, secret/payload audit, and hosting authorization—satisfied. Chunk 12's never-executed evaluation-only tag/proxy/finalization sequence is explicitly superseded and is not a dependency. Evidence-gated public Evaluate and search items may remain `[!]` only when the corresponding features remain absent and explicitly unsupported.

**Owned files:** `planning/IMPLEMENTATION_PLAN.md`, `CHANGELOG.md`, `docs/releasing.md`, `docs/evaluation-live-evidence.md`, `.github/workflows/ci.yml`, `.github/workflows/live-contract.yml`.

**Commit boundary:** `chore: verify public api release candidate`

- [x] **Historical Chunk 24 recheck completed on 2026-09-21; superseded for request support by the later search-request continuation.** Its then-current conclusion that every search gate remained preserved was later narrowed first for four Chat request declarations and then for fixed low-context Responses `web_search` and fieldless Gateway `x_search` through opt-in wrappers. No completed Chunk 24 implementation claim is changed; configurable options, preview/other forms, typed outputs/events, wider model compatibility, direct-xAI, and unrelated surfaces remain blocked.
- [!] The complete local hermetic/race/vet/docs/examples/external-consumer inventory is successful, including the strengthened build-tagged live gates compiling and failing closed without credentials. Exact remaining blocker: protected hosted CI has not run and its environment/network-denial evidence is unavailable; local checks do not establish hosted CI success.
- [!] Authorized sanitized live evidence is **NOT RUN / PENDING LIVE RUN** for the already implemented general public-generation and provider-Evaluation contracts. Public Evaluate remains excluded. The later search-request continuation separately records the evidence supporting fixed low-context Responses `web_search` and fieldless Gateway `x_search` request declarations through the opt-in wrapper; that narrow support does not clear the general paid-live gate or authorize configurable options, preview/other forms, typed outputs/events, wider model compatibility, direct-xAI, or unrelated surfaces.
- [x] **Historical Chunk 24 exported-API/backward-compatibility audit.** Existing provider `Evaluate`/`WithBaseURL`, module/package identity, five error types, and documented evaluation types remained source-compatible; generation was additive through explicit Responses/Chat methods and `WithPublicBaseURL`. At that closure the API/local-consumer evidence found no compatibility aliases, generic tool bags, automatic execution, `/v4/ai/language-model`, public `/v1/evaluate`, direct-xAI, or search exports then blocked. The later opt-in Responses search wrapper and its fixed low-context `web_search` and fieldless Gateway `x_search` declarations supersede only that historical absence.
- [x] Completed the changelog and release-note audit. `CHANGELOG.md`, README, generation/evaluation/search documentation, and release guidance cover the implemented endpoints, buffered/streaming lifecycle and resource contracts, experimental v0 risk, module-path migration, separate hermetic/live status, raw-only Responses boundary, and independent public-Evaluate/search blockers without claiming hosted/live/publication success.
- [!] Immutable continuation tag creation, direct-VCS/public-proxy import, fetched-module compilation, checksum/provenance capture, promotion, and publication are deferred. Exact blockers: owner-selected license; independently verified hosted repository metadata and protected release environment; authorized live contracts; hosted CI; provenance/publication/least-privilege review; hosting authorization; and post-tag direct-VCS/proxy/checksum evidence. No tag or release may be created or claimed before all blockers clear.

**Acceptance commands:**
Run every ordinary command below. For each paid smoke, run exactly one of its two Bash credential variants; the guard rejects zero or multiple nonblank protected-environment credentials before `go test` starts.


```sh
env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK GOPROXY=off go test ./...
env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK GOPROXY=off go test -race -count=1 ./...
env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK GOPROXY=off go vet ./...
env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK GOPROXY=off go test ./examples/...
env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go doc -all .
env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK ./scripts/verify-local-consumer.sh
# Generation live smoke — API-key variant
[[ ${AI_GATEWAY_API_KEY:-} =~ [^[:space:]] && ! ${VERCEL_OIDC_TOKEN:-} =~ [^[:space:]] ]] && env -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK AI_GATEWAY_API_KEY="$AI_GATEWAY_API_KEY" AI_GATEWAY_PUBLIC_LIVE_COST_ACK=I_ACCEPT_LIVE_PUBLIC_API_COSTS go test -tags=livecontract ./internal/livecontract -run 'TestGateway(Responses|Chat)Contract'
# Generation live smoke — OIDC variant
[[ ${VERCEL_OIDC_TOKEN:-} =~ [^[:space:]] && ! ${AI_GATEWAY_API_KEY:-} =~ [^[:space:]] ]] && env -u AI_GATEWAY_API_KEY -u AI_GATEWAY_LIVE_COST_ACK VERCEL_OIDC_TOKEN="$VERCEL_OIDC_TOKEN" AI_GATEWAY_PUBLIC_LIVE_COST_ACK=I_ACCEPT_LIVE_PUBLIC_API_COSTS go test -tags=livecontract ./internal/livecontract -run 'TestGateway(Responses|Chat)Contract'
# Provider-evaluation live smoke — API-key variant
[[ ${AI_GATEWAY_API_KEY:-} =~ [^[:space:]] && ! ${VERCEL_OIDC_TOKEN:-} =~ [^[:space:]] ]] && env -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK AI_GATEWAY_API_KEY="$AI_GATEWAY_API_KEY" AI_GATEWAY_LIVE_COST_ACK=I_ACCEPT_LIVE_EVALUATION_COSTS go test -tags=livecontract ./internal/livecontract -run TestGatewayEvaluationContract
# Provider-evaluation live smoke — OIDC variant
[[ ${VERCEL_OIDC_TOKEN:-} =~ [^[:space:]] && ! ${AI_GATEWAY_API_KEY:-} =~ [^[:space:]] ]] && env -u AI_GATEWAY_API_KEY -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK VERCEL_OIDC_TOKEN="$VERCEL_OIDC_TOKEN" AI_GATEWAY_LIVE_COST_ACK=I_ACCEPT_LIVE_EVALUATION_COSTS go test -tags=livecontract ./internal/livecontract -run TestGatewayEvaluationContract
```

**Review/blocker accounting:**

- [x] Two-part final closure review and its accounting fix are recorded. `final-closure-review-1` returned **CLEAN** after independently verifying every substantive final correction at cleanup commit/current HEAD `ee41722`. `final-closure-review-2` independently confirmed the preserved external blockers and identified only missing tracker accounting/local-checkbox closure; commit `3545fac` and this closure evidence fix that finding. Every local/doable continuation item is checked `[x]`, while every incomplete item is `[!]` with its exact external or deferred blocker. These reviews and accounting changes do not establish paid live, hosted CI, hosted metadata, license, provenance, tag, direct-VCS/proxy, checksum, or publication success.
- [!] **Release remains blocked.** Exact external dependencies: owner-authorized paid generation and provider-evaluation evidence; protected hosted CI/environment evidence; owner-selected license; hosted repository metadata, provenance, permissions, and publication review; hosting authorization; immutable tag; direct-VCS and public-proxy verification; fetched-module checksum/provenance capture; and final publication. Current local API/docs/consumer/static secret-payload work is complete and is not a blocker.

## Continuation release checklist

- [!] Required Chunks 13, 15–16, 18–19, 21, and 23 plus Chunk 22 Hermetic are `[x]`; Chunk 22 Live and Chunk 24 overall remain blocked solely on the exact external release dependencies recorded above. Public Evaluate remains absent. The later continuations implement the four Chat request declarations plus fixed low-context Responses `web_search` and fieldless Gateway `x_search` through opt-in wrappers; configurable options, preview/other forms, typed outputs/events, wider model compatibility, direct-xAI, and unrelated surfaces remain blocked.
- [x] Existing `Client.Evaluate`, `WithBaseURL`, exported evaluation types/errors, and module/package identity remain source-compatible; additive public generation uses `WithPublicBaseURL` and explicit surface methods.
- [x] `/v1/responses` and `/v1/chat/completions` have hermetically proven buffered and streaming text paths with their originating bounds, cancellation, caller-owned closure, no hidden stream goroutine, no post-header replay, and no automatic tool execution. Local full/race/vet/docs/examples/external-consumer and live-gate compile/fail-closed evidence is recorded; hosted and paid-live success remain separate blockers.
- [x] `/v1/evaluate` remains absent and explicitly evidence-gated until first-party plus authorized-live success presence/nullability/unknown-field semantics are sufficient for a deterministic public contract. Any future API must be distinct, and `/v4/ai/evaluation-model` remains unchanged.
- [x] **Historical Chunk 24 API state:** Responses built-in search, every Chat Gateway server-search identifier, and Gateway-native `x_search` were absent from exports/support claims at closure. The later authoritative continuations now support the four Chat request declarations plus fixed low-context Responses `web_search` and fieldless Gateway `x_search` through opt-in wrappers. Outputs/streams/citations/errors remain raw-only where already available; configurable options, preview/other forms, typed outputs/events, wider model compatibility, direct-xAI, and unrelated surfaces remain blocked.
- [x] `/v4/ai/language-model` and direct xAI remain absent from the exported API and support claims. Gateway fieldless `x_search` is now narrowly supported only through the opt-in Responses wrapper; no direct-xAI client or broader `x_search` option/output/event/model contract is implied.
- [x] Current static secret/payload attestation is complete: error formatting, examples, docs, tests, workflows, and retained local artifacts do not expose credentials, authorization headers, prompts/state, provider/tool values, raw diagnostic bodies, streamed/generated content, or unsanitized live payloads. Future live/hosted/tag/proxy artifacts require their own pre-publication sanitization review.
- [!] Publication is deferred until the following exact external gates complete: protected hosted CI; both authorized sanitized paid live contracts; owner-selected license; hosted repository metadata/provenance/permissions/publication review; hosting authorization; immutable continuation tag; direct-VCS and public-proxy fetched-module verification; checksum/provenance capture; and final promotion/publication. Current upstream/version, API/migration/docs, local hermetic/race/vet/doc/examples/external-consumer, and current static secret/payload audits are `[x]`.

## 2026-09-21 evidence continuation — authoritative current disposition

This section records the authoritative disposition as of the Chat search continuation and is retained as historical evidence. It superseded earlier completed-chunk statements without reopening completed Chunks 01–23 or relabeling externally blocked Chunk 24 complete. The later **2026-09-21 search-request continuation — superseding current authority** further supersedes this section for fixed low-context Responses `web_search` and fieldless Gateway `x_search` request support only; its narrower remaining blockers are current.

### Surface disposition

| Surface | Status at this historical stage | Decision at this historical stage |
| --- | --- | --- |
| Public `POST /v1/evaluate` | [!] blocked | Keep `Client.EvaluatePublic` and public DTOs absent. The endpoint/request example and one observed 400 shape do not establish an exhaustive strict success/error contract. |
| Responses `web_search` / `web_search_preview` | [!] historical blocked evidence gate, later narrowly superseded | At this stage no request type was authorized. The later search-request continuation clears only fixed `{"type":"web_search","search_context_size":"low"}` through the opt-in wrapper; `web_search_preview`, other forms/options, typed outputs/events, and wider model compatibility remain blocked. |
| Chat `vercel:exa_search` / `vercel:parallel_search` / `vercel:perplexity_search` / `vercel:tako_search` | [x] request-only implementation accepted and complete | The first-party Vercel Gateway web-search page directly names all four Chat identifiers and their snake-case request config shapes. The implementation, review corrections, and public documentation span the preserved four-commit substantive boundary recorded below. The authoritative nil semantics are implemented, verified, and publicly documented: a nil `ChatExaSubpageTarget` interface omits the option; value-form `ChatExaSubpageTargetStrings(nil)` and a non-nil pointer to a nil slice emit `[]`; a typed-nil pointer rejects before credentials or network. Focused Chat/API tests, the full suite, focused race/full race, vet, examples, docs, external-consumer verification, diff-check, and workspace diagnostics all passed with credentials and acknowledgements unset. Typed Chat outputs/events remain blocked. |
| Gateway-native xAI `x_search` | [!] historical blocked gate, later narrowly superseded | At this stage model capability plus direct `@ai-sdk/xai` schemas did not prove the Gateway contract. The later continuation clears only fieldless `{"type":"x_search"}` through the opt-in wrapper; configurable options, typed outputs/events, wider model compatibility, direct-xAI, and unrelated surfaces remain blocked. |

### Gate A — public `POST /v1/evaluate` remains blocked

- [!] The official Evaluation page (`last_updated: 2026-09-16`) proves the endpoint, request members (`model`, `state`, `questions`, optional `providerOptions`), BYOK availability, and one representative success. A credential-free probe observed one HTTP 400 form, `{error:{message:string,param:null,type:"invalid_request_error"}}`. Released Evaluation Model V4 code supplies useful semantics only for the different `/v4/ai/evaluation-model` protocol.
- [!] Missing first-party public-HTTP contract: exhaustive success properties/required list; nullability; choice/score probability presence; whether `rounding`, `warnings`, or `usage.totalTokens` exist; numeric constraints; additional-property and duplicate-key behavior; and complete error/status variants. Therefore no strict public decoder can be designed without inference.
- [!] Unblock only with a first-party public `/v1/evaluate` schema/server artifact or documentation amendment resolving every item above. Live fixtures may corroborate documented variants but cannot prove exhaustiveness, absence, optionality, or unknown-field policy. Once cleared, plan a new implementation chunk; do not reuse the provider-protocol decoder.

### Historical Gate B — Responses built-in search request support was blocked here; later narrowly superseded

**Historical prospective commit boundary after the then-missing evidence cleared:** `feat: add responses web search requests`

- [!] **Historical evidence state:** first-party Gateway evidence then established `POST /v1/responses` and generic tool support but did not explicitly establish either `web_search` form or its request fields. The later search-request continuation supplies new evidence and clears only fixed `{"type":"web_search","search_context_size":"low"}` through the opt-in wrapper.
- [!] OpenAI documentation and released `@ai-sdk/openai@4.0.71` remain direct-OpenAI evidence only and authorize no broader Vercel Gateway contract.
- [!] No typed output or event projection is authorized. `ResponseResult.RawJSON()` remains the only buffered output contract; `ResponseOutputTextDeltaEvent` plus `RawResponseEvent` remain the streaming contract. Do not add typed search actions, citations, refusals, terminal/error event variants, or requiredness/nullability/unknown-variant rules from direct-provider evidence.
- [!] The later request evidence does not clear `web_search_preview`, omission or other values of `search_context_size`, configurable options, tool-choice/allowed-tools behavior, broader routing/model compatibility, or any other form.
- [!] Evidence required before any typed output/event work is separately planned remains first-party Gateway schema/source/test or owner-authorized Gateway fixtures that establish each exact discriminator, field type, requiredness, nullability, unknown-field/unknown-variant behavior, stream event ordering/termination effect, and raw preservation rule. Request evidence alone does not clear output evidence.
- [x] The later continuation implemented the exact fixed low-context request through an opt-in wrapper and preserved source compatibility; this historical prospective ownership statement no longer controls current dependency readiness.

### Chunk 25 — Chat Gateway server-search request declarations

**Depends on:** completed Chat buffered/streaming infrastructure only. Chunk 24 is locally complete but remains `[!]` on the named external release/publication gates; under the global sequence exception, those gates did not block this independently approved feature chunk, and Chunk 25 completed without relabeling Chunk 24 complete. This chunk had no semantic dependency on the then-blocked Responses built-in-search gate; the later search-request continuation independently cleared only fixed low-context Responses `web_search` through its opt-in wrapper.

**Owned files (exact, nine-file exception):** `chat.go`, `chat_stream.go`, `chat_wire.go`, `chat_validate.go`, `chat_test.go`, `contract_external_test.go`, `docs/generation.md`, `doc.go`, `planning/IMPLEMENTATION_PLAN.md`. This is an explicit exception to the 3–5 target-path preference because the two exported server-tool entry points, their shared request encoding/validation, compile-locked contract, hermetic proof, public support documentation, factual package-overview support statement, and truthful tracker accounting must land in the same reviewable commit under the global completion rule; documentation and tracker state may not trail the API in separately accepted chunks.

**Four-commit substantive review boundary:** `29af060` (`feat: add chat gateway search tools`) contains `chat.go`, `chat_stream.go`, `chat_test.go`, `chat_validate.go`, `chat_wire.go`, `contract_external_test.go`, `doc.go`, `docs/generation.md`, and `planning/IMPLEMENTATION_PLAN.md`; `ef840df` (`fix: correct chat search request handling`) contains `chat_test.go`, `chat_validate.go`, `chat_wire.go`, `contract_external_test.go`, and `planning/IMPLEMENTATION_PLAN.md`; `8bd93e9` (`fix: preserve chat search slice presence`) contains `chat_test.go`, `chat_validate.go`, `chat_wire.go`, and `planning/IMPLEMENTATION_PLAN.md`; and `8978cf6` (`docs: clarify chat search presence semantics`) contains exactly `docs/generation.md` and `planning/IMPLEMENTATION_PLAN.md`, supplying the public nil-union documentation and tracker correction. Two independent post-amend re-reviews of HEAD `8978cf6` returned **CLEAN**. The final tracker-only closure boundary is identified self-relatively by intended Conventional Commit subject `docs: close chat search chunk` and contains exactly `planning/IMPLEMENTATION_PLAN.md`. Its final hash is intentionally not embedded because this plan is part of that same commit and embedding its final hash would be self-referential; this tracker-only closure does not alter the preserved four-commit substantive review boundary.

The first-party page <https://vercel.com/docs/ai-gateway/models-and-providers/web-search> (`last_updated: 2026-09-08`, inspected 2026-09-21) directly names the four Chat Completions tool identifiers, the `{type,config}` request shape, snake-case config members, universal Gateway-model compatibility, hidden server execution, synthesized final answer behavior, coexistence with ordinary functions, name-collision rule, and `tool_choice` modes `auto` and `required`. It does **not** define typed raw results, streamed search lifecycle, structured citations/offsets, provider failures/refusals, or an exact `provider_metadata.gateway.gatewayToolCalls` schema. Released `@ai-sdk/gateway` helper schemas may corroborate member types but are not treated as Chat output evidence or as proof of REST rejection semantics.

#### Exact exported Go API

The existing exported `ChatCompletionRequest`, its field set, `ChatTool` function-tool struct, `ChatCompletionRequest.Tools []ChatTool`, and existing `CreateChatCompletion`/`StreamChatCompletion` method signatures remain exactly unchanged, including compatibility with external unkeyed `ChatCompletionRequest` literals. Server tools use a distinct wrapper request and two distinct methods, so no field is added to the existing request. Optional server-tool members use pointers: nil means omitted; a non-nil pointer emits the member even when the pointed scalar is its zero value. Optional slices use pointers so nil pointer means omitted, pointer-to-nil slice emits `[]`, and pointer-to-empty non-nil slice also emits `[]`; request JSON never emits `null`. Config is always emitted as an object. Required strings are ordinary strings and must be nonempty. No generic map or extension bag is added.
```go
// ChatCompletionRequest remains byte-for-byte the existing exported declaration.
type ChatCompletionRequest struct {
    Model            string
    Messages         []ChatMessage
    Temperature      *float64
    MaxTokens        *int
    TopP             *float64
    FrequencyPenalty *float64
    PresencePenalty  *float64
    Stop             ChatStop
    SafetyIdentifier *string
    Tools            []ChatTool
    ToolChoice       ChatToolChoice
    ResponseFormat   ChatResponseFormat
    Models           []string
    ProviderOptions  *ChatProviderOptions
    Provider         *ChatProvider
}

// ChatServerToolsRequest adds typed server tools without changing ChatCompletionRequest.
type ChatServerToolsRequest struct {
    Request     ChatCompletionRequest
    ServerTools []ChatServerTool
}

func (c *Client) CreateChatCompletionWithServerTools(ctx context.Context, request ChatServerToolsRequest) (*ChatCompletionResult, error)
func (c *Client) StreamChatCompletionWithServerTools(ctx context.Context, request ChatServerToolsRequest) (*ChatCompletionStream, error)

type ChatServerTool interface{ chatServerTool() }

// Existing function tool; declaration, source use, and wire shape remain unchanged.
type ChatTool struct {
    Name        string
    Description *string
    Parameters  any
}

type ChatExaSearchTool struct{ Config ChatExaSearchConfig }
type ChatParallelSearchTool struct{ Config ChatParallelSearchConfig }
type ChatPerplexitySearchTool struct{ Config ChatPerplexitySearchConfig }
type ChatTakoSearchTool struct{ Config ChatTakoSearchConfig }
func (ChatExaSearchTool) chatServerTool()
func (ChatParallelSearchTool) chatServerTool()
func (ChatPerplexitySearchTool) chatServerTool()
func (ChatTakoSearchTool) chatServerTool()

type ChatToolChoiceMode string
const (
    ChatToolChoiceAuto     ChatToolChoiceMode = "auto"
    ChatToolChoiceNone     ChatToolChoiceMode = "none"
    ChatToolChoiceRequired ChatToolChoiceMode = "required"
)

type ChatExaSearchType string
const (
    ChatExaSearchAuto    ChatExaSearchType = "auto"
    ChatExaSearchFast    ChatExaSearchType = "fast"
    ChatExaSearchInstant ChatExaSearchType = "instant"
)
type ChatExaCategory string
const (
    ChatExaCategoryCompany         ChatExaCategory = "company"
    ChatExaCategoryPeople          ChatExaCategory = "people"
    ChatExaCategoryResearchPaper   ChatExaCategory = "research paper"
    ChatExaCategoryNews            ChatExaCategory = "news"
    ChatExaCategoryPersonalSite    ChatExaCategory = "personal site"
    ChatExaCategoryFinancialReport ChatExaCategory = "financial report"
)
type ChatExaVerbosity string
const (
    ChatExaVerbosityCompact  ChatExaVerbosity = "compact"
    ChatExaVerbosityStandard ChatExaVerbosity = "standard"
    ChatExaVerbosityFull     ChatExaVerbosity = "full"
)
type ChatExaSection string
const (
    ChatExaSectionHeader     ChatExaSection = "header"
    ChatExaSectionNavigation ChatExaSection = "navigation"
    ChatExaSectionBanner     ChatExaSection = "banner"
    ChatExaSectionBody       ChatExaSection = "body"
    ChatExaSectionSidebar    ChatExaSection = "sidebar"
    ChatExaSectionFooter     ChatExaSection = "footer"
    ChatExaSectionMetadata   ChatExaSection = "metadata"
)
type ChatExaText interface{ chatExaText() }
type ChatExaTextEnabled bool
type ChatExaTextOptions struct {
    MaxCharacters   *int
    IncludeHTMLTags *bool
    Verbosity       *ChatExaVerbosity
    IncludeSections *[]ChatExaSection
    ExcludeSections *[]ChatExaSection
}
func (ChatExaTextEnabled) chatExaText()
func (ChatExaTextOptions) chatExaText()
type ChatExaHighlights interface{ chatExaHighlights() }
type ChatExaHighlightsEnabled bool
type ChatExaHighlightsOptions struct {
    Query         *string
    MaxCharacters *int
}
func (ChatExaHighlightsEnabled) chatExaHighlights()
func (ChatExaHighlightsOptions) chatExaHighlights()
type ChatExaExtras struct {
    Links      *int
    ImageLinks *int
}
type ChatExaSubpageTarget interface{ chatExaSubpageTarget() }
type ChatExaSubpageTargetString string
type ChatExaSubpageTargetStrings []string
func (ChatExaSubpageTargetString) chatExaSubpageTarget()
func (ChatExaSubpageTargetStrings) chatExaSubpageTarget()
type ChatExaContents struct {
    Text             ChatExaText
    Highlights       ChatExaHighlights
    MaxAgeHours      *int
    LivecrawlTimeout *int
    Subpages         *int
    SubpageTarget    ChatExaSubpageTarget
    Extras           *ChatExaExtras
}
type ChatExaSearchConfig struct {
    Query              string
    Type               *ChatExaSearchType
    NumResults         *int
    Category           *ChatExaCategory
    UserLocation       *string
    IncludeDomains     *[]string
    ExcludeDomains     *[]string
    StartPublishedDate *string
    EndPublishedDate   *string
    Contents           *ChatExaContents
}

type ChatParallelMode string
const (
    ChatParallelModeOneShot ChatParallelMode = "one-shot"
    ChatParallelModeAgentic ChatParallelMode = "agentic"
)
type ChatParallelSourcePolicy struct {
    IncludeDomains *[]string
    ExcludeDomains *[]string
    AfterDate      *string
}
type ChatParallelExcerpts struct {
    MaxCharsPerResult *int
    MaxCharsTotal     *int
}
type ChatParallelFetchPolicy struct{ MaxAgeSeconds *int }
type ChatParallelSearchConfig struct {
    Objective     string
    SearchQueries *[]string
    Mode          *ChatParallelMode
    MaxResults    *int
    SourcePolicy  *ChatParallelSourcePolicy
    Excerpts      *ChatParallelExcerpts
    FetchPolicy   *ChatParallelFetchPolicy
}

type ChatPerplexityQuery interface{ chatPerplexityQuery() }
type ChatPerplexityQueryString string
type ChatPerplexityQueryStrings []string
func (ChatPerplexityQueryString) chatPerplexityQuery()
func (ChatPerplexityQueryStrings) chatPerplexityQuery()
type ChatPerplexityRecency string
const (
    ChatPerplexityRecencyDay   ChatPerplexityRecency = "day"
    ChatPerplexityRecencyWeek  ChatPerplexityRecency = "week"
    ChatPerplexityRecencyMonth ChatPerplexityRecency = "month"
    ChatPerplexityRecencyYear  ChatPerplexityRecency = "year"
)
type ChatPerplexitySearchConfig struct {
    Query                   ChatPerplexityQuery
    MaxResults              *int
    MaxTokensPerPage        *int
    MaxTokens               *int
    Country                 *string
    SearchDomainFilter      *[]string
    SearchLanguageFilter    *[]string
    SearchAfterDate         *string
    SearchBeforeDate        *string
    LastUpdatedAfterFilter  *string
    LastUpdatedBeforeFilter *string
    SearchRecencyFilter     *ChatPerplexityRecency
}

type ChatTakoEffort string
const (
    ChatTakoEffortDeep    ChatTakoEffort = "deep"
    ChatTakoEffortFast    ChatTakoEffort = "fast"
    ChatTakoEffortInstant ChatTakoEffort = "instant"
)
type ChatTakoDataMode string
const (
    ChatTakoDataModeInline ChatTakoDataMode = "inline"
    ChatTakoDataModeURL    ChatTakoDataMode = "url"
)
type ChatTakoContentFormat string
const (
    ChatTakoContentFormatCardJSON    ChatTakoContentFormat = "card_json"
    ChatTakoContentFormatCSV         ChatTakoContentFormat = "csv"
    ChatTakoContentFormatJSONCompact ChatTakoContentFormat = "json_compact"
    ChatTakoContentFormatJSONRecords ChatTakoContentFormat = "json_records"
)
type ChatTakoWebCategory string
const (
    ChatTakoWebCategoryFinance ChatTakoWebCategory = "finance"
    ChatTakoWebCategoryNews    ChatTakoWebCategory = "news"
    ChatTakoWebCategorySports  ChatTakoWebCategory = "sports"
)
type ChatTakoDataSource struct {
    Count           *int
    IncludeContents *bool
    Mode            *ChatTakoDataMode
    ContentFormat   *ChatTakoContentFormat
    MaxRows         *int
    NodeIDs         *[]string
    Strict          *bool
}
type ChatTakoWebSource struct {
    Count                  *int
    IncludeContents        *bool
    Category               *ChatTakoWebCategory
    IncludeDomains         *[]string
    ExcludeDomains         *[]string
    SnippetMaxChars        *int
    Highlights             *bool
    ArticleContentMaxChars *int
    PublishedAfter         *string
    PublishedBefore        *string
}
type ChatTakoSources struct {
    Data *ChatTakoDataSource
    Web  *ChatTakoWebSource
}
type ChatTakoLocation struct {
    Latitude  float64
    Longitude float64
}
type ChatTakoOutputSettings struct{ ImageDarkMode *bool; ForceRefresh *bool }
type ChatTakoSearchConfig struct {
    Query          string
    Effort         *ChatTakoEffort
    Sources        *ChatTakoSources
    Location       *ChatTakoLocation
    CountryCode    *string
    Locale         *string
    Timezone       *string
    OutputSettings *ChatTakoOutputSettings
    IncludeRelated *int
}
```

`ChatCompletionRequest` and `ChatCompletionRequest.Tools []ChatTool` remain unchanged. `ChatServerToolsRequest.ServerTools []ChatServerTool` is the sole server-tool request addition, and the existing methods continue accepting only `ChatCompletionRequest`; callers opt in through `CreateChatCompletionWithServerTools` or `StreamChatCompletionWithServerTools`. No alias, generic tool bag, output type, or cross-surface converter is added. The two new methods must reuse the existing private Chat validation, encoding, transport/retry, response-decoding, and stream-lifecycle paths rather than duplicate the client; the wrapper contributes only its server-tool slice. Wire encoding produces one `tools` JSON array: `Request.Tools` entries first in their caller order, followed by `ServerTools` entries in their caller order. If both slices are nil or empty, preserve the existing omission behavior; otherwise emit exactly one array, including when only one slice is populated. Validation and the 10,000-tool limit apply to the combined count, and an overflow is reported deterministically at the first entry beyond the limit in this same ordinary-then-server order.

#### Request encoding and validation contract

- [x] Encode ordinary `ChatTool` exactly as today, then encode each server tool as `{type:"vercel:<identifier>",config:{...}}` in the deterministic combined ordering above, using the exact snake-case member names represented above. Omit nil optionals; never emit JSON null; preserve explicit false, zero, and empty arrays through non-nil pointers. Reject nil/typed-nil `ChatServerTool`, `ChatExaText`, `ChatExaHighlights`, and `ChatPerplexityQuery` implementations before credential or network work. For `ChatExaSubpageTarget`, a nil interface omits the option, value-form `ChatExaSubpageTargetStrings(nil)` and a non-nil pointer to a nil slice are present supported variants that emit `[]`, and a typed-nil pointer implementation is invalid before credential or network work. Non-nil pointers to other exported value-receiver server-tool or nested union variants serialize identically to corresponding values. Validation and exact regression coverage now verify these normalization semantics; clean independent re-review remains tracked separately below.
- [x] Validate only proven local invariants: required `query`/`objective`; the closed enum values above; valid documented union variants, including non-nil pointers to value-receiver variants; the existing 1 MiB string, 64-container-depth, 10,000-member/item, finite-number, cycle, and request-body bounds; a client-function/server-tool name collision only when the corresponding server tool is present in `ServerTools`; and the explicitly documented Tako rule that `strict:true` requires present nonempty `node_ids`. An ordinary function named `exa_search`, `parallel_search`, `perplexity_search`, or `tako_search` is valid when its corresponding server tool is absent. Do **not** infer hostname/date/ISO normalization, default insertion, unknown-field forwarding, model-catalog restrictions, or server rejection from prospective optional combinations.
- [x] Add the exact exported constant declared above: `ChatToolChoiceRequired ChatToolChoiceMode = "required"`. Named `ChatSpecificToolChoice{Name: ...}` remains legal for an ordinary function, including one with a reserved-looking name when the corresponding server tool is absent. Reject a named choice only when the corresponding server tool is present in the same request and the name attempts to select that server tool, whether by its full identifier or corresponding function name (`exa_search`, `parallel_search`, `perplexity_search`, `tako_search`); never encode a named-function choice as a way to select a present server tool. Do not reject other named function choices merely because any server tool is present. Preserve existing `auto` and `none`; add `required` without changing their encoding.
- [x] Preserve `ChatCompletionResult`, `ChatCompletionChunk`, `RawJSON`, streamed raw chunk bytes, `ResponseError`, and all existing metadata behavior byte-for-byte and type-for-type. Do not decode `gatewayToolCalls`, costs, raw results, citations, offsets, lifecycle deltas, refusals, or server-tool-specific errors. No field is added to buffered or streaming output types.

#### Independent identifier acceptance checklist

| Identifier | Minimum exact JSON | Option/omission fixtures | Collision and tool-choice fixtures | Zero-dispatch invalid fixtures | Output unchanged |
| --- | --- | --- | --- | --- | --- |
| `vercel:exa_search` | [x] | [x] every declared option group/union/enum and nil-vs-explicit-zero/false/empty presence is covered; a nil union interface omits the option, both value-form `ChatExaSubpageTargetStrings(nil)` and pointer-to-nil emit `[]`, typed-nil pointers fail before dispatch, and exact complete-wire plus value/pointer parity are verified | [x] ordinary `exa_search` alone allowed; with Exa server tool present reject collision and matching named choice; allow named different function | [x] nil-interface omission, nil value/pointer acceptance, typed-nil rejection, and other invalid fixtures reject or normalize before dispatch as specified | [x] buffered/stream raw compatibility; no new projection |
| `vercel:parallel_search` | [x] | [x] every declared policy/object/enum and presence case; exact complete wire fixture and pointer/value parity verified | [x] ordinary `parallel_search` alone allowed; with Parallel server tool present reject collision and matching named choice; allow named different function | [x] empty objective, bad enum, typed-nil and non-nil pointer tool coverage, generic bounds | [x] buffered/stream raw compatibility; no new projection |
| `vercel:perplexity_search` | [x] | [x] string and string-array query plus every declared option/enum/presence case; exact complete wire fixture and pointer/value parity verified | [x] ordinary `perplexity_search` alone allowed; with Perplexity server tool present reject collision and matching named choice; allow named different function | [x] nil/empty query union, bad enum/union, typed-nil and non-nil pointer union coverage, generic bounds | [x] buffered/stream raw compatibility; no new projection |
| `vercel:tako_search` | [x] | [x] every declared source/location/output/enum and presence case; exact complete wire fixture and pointer/value parity verified | [x] ordinary `tako_search` alone allowed; with Tako server tool present reject collision and matching named choice; allow named different function | [x] empty query, bad enum, `strict:true` without node IDs, nonfinite location, typed-nil and non-nil pointer tool coverage, generic bounds | [x] buffered/stream raw compatibility; no new projection |

- [x] Cross-identifier tests cover both new methods; multiple server tools together; exact `Request.Tools`-then-`ServerTools` wire ordering; ordinary-only, server-only, and both-empty omission behavior; the combined 10,000-tool boundary and limit-plus-one path; `auto`, `required`, preserved `none` with and without server tools; named choice of a different ordinary function; each reserved-looking ordinary function and named choice accepted when its corresponding server tool is absent; rejection only when the corresponding server tool is present and creates a collision or is targeted by name; duplicate server-tool declarations serialized in caller order rather than deduplicated; and exact preservation of all existing function-tool fixtures, `[]ChatTool` assignments, and unkeyed `ChatCompletionRequest` compatibility.
- [x] `contract_external_test.go` compile-locks every declaration and constant above, including an external positional composite literal containing all existing `ChatCompletionRequest` fields, unchanged `Tools []ChatTool`, `ChatServerToolsRequest`, both new method signatures, and exact `ChatToolChoiceRequired` type/value; verifies each private-marker implementation through wrapper request construction; compile-locks unchanged existing methods and result/chunk/raw APIs; and asserts the exact ordered exported `ChatCompletionRequest` field names and types so reordering among nilable fields cannot pass unnoticed. It does not assert any server-search output field.

**Focused verification commands (implementation time only):**

```sh
env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go test -run 'Test(CreateChatCompletion|ChatRequest|ChatGatewaySearch|ExternalContract)' ./...
env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go test -race -run 'Test(CreateChatCompletion|ChatRequest|ChatGatewaySearch)' ./...
env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go test ./...
env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go doc -all .
```

**Recorded pre-review verification evidence:**

- [x] `env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go test -run 'Test(CreateChatCompletion|ChatRequest|ChatGatewaySearch|ExternalContract)' ./...` passed.
- [x] `env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go test -race -run 'Test(CreateChatCompletion|ChatRequest|ChatGatewaySearch)' ./...` passed.
- [x] `env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go test ./...` passed.
- [x] `env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go doc -all .` rendered successfully. Its package overview exposed the stale statement that all search is unsupported; correcting that factual package support statement adds `doc.go` to this chunk's atomic owned-path exception.
- [x] Workspace Go diagnostics reported no issues.

**Recorded post-review-fix verification evidence:**

- [x] `env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go test -run 'Test(CreateChatCompletion|ChatRequest|ChatGatewaySearch|ExternalContract)' ./...` passed with the pointer-union, complete-wire-fixture, and exact external request-contract fixes.
- [x] `env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go test ./...` passed.
- [x] `env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go test -race -run 'Test(CreateChatCompletion|ChatRequest|ChatGatewaySearch)' ./...` passed.
- [x] Workspace Go LSP diagnostics reported `No issues found`.

**Recorded `CHAT-NIL-SUBPAGE-POINTER` fix verification evidence:**

- [x] `env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go test -run 'Test(CreateChatCompletion|ChatRequest|ChatGatewaySearch|ExternalContract)' ./...` passed after the nil subpage-target pointer normalization fix and its exact JSON `[]` regression coverage.
- [x] `env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go test ./...` passed.
- [x] `env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go test -race -run 'Test(CreateChatCompletion|ChatRequest|ChatGatewaySearch)' ./...` passed.
- [x] Workspace Go LSP diagnostics reported `No issues found` after the fix.

**Recorded nil value-form validation-fix verification evidence:**

- [x] Independent semantics review established that a nil `ChatExaSubpageTarget` interface omits the option; value-form `ChatExaSubpageTargetStrings(nil)` and a non-nil pointer to a nil slice are present supported variants and encode `subpage_target:[]`; a typed-nil pointer implementation is invalid before credential or network work. Validation was corrected to accept and normalize both supported nil-slice forms while preserving nil-interface omission and typed-nil rejection.
- [x] `env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go test -run 'Test(CreateChatCompletion|ChatRequest|ChatGatewaySearch|ExternalContract)' ./...` passed after the validation fix.
- [x] `env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go test ./...` passed after the validation fix.
- [x] `env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go test -race -run 'Test(CreateChatCompletion|ChatRequest|ChatGatewaySearch)' ./...` passed after the validation fix.
- [x] Workspace Go LSP diagnostics reported `No issues found` after accepting and normalizing the nil value and pointer forms.

**Documentation tasks in the same atomic chunk:**

- [x] Update `docs/generation.md` Chat request inventory with all four identifiers, exact `{type,config}` shapes, exported Go declarations, the opt-in `ChatServerToolsRequest` and its two methods, the fact that `ChatCompletionRequest` and existing method signatures are unchanged, deterministic `Request.Tools`-then-`ServerTools` wire ordering, nil/zero/false/empty presence semantics, required fields, enums, conditional collision behavior, and the proven named-tool-choice rule. Include migration examples for buffered and streaming calls and state that existing keyed and unkeyed `ChatCompletionRequest` literals keep compiling because its field set is unchanged. State that server execution is hidden and any inline citations are model-generated text rather than a typed citation contract.
- [x] Keep the Responses section explicitly unchanged and blocked for built-in search. State that Chat support does not imply Responses support and that neither OpenAI nor direct-provider SDK evidence clears the Gateway Responses gate.
- [x] State prominently that buffered/streamed outputs, raw accessors, metadata, errors, privacy/resource limits, and live status are unchanged; no raw search results, lifecycle events, structured citations, refusals, provider errors, `gatewayToolCalls`, or cost shape is typed. Correct the `doc.go` package overview so it no longer says all search is unsupported while preserving the blocked Responses and Gateway-native `x_search` statements.
- [x] Add an independent support table for all four identifiers with request implementation documented as complete, typed output `[!]`, and live corroboration `not part of implementation acceptance`; final accepted-support `[x]` status is recorded after closure of the whole Chunk 25 substantive review boundary.

**Completion/review boundary:** implementation, documentation, and pre-review verification landed in `29af060` (`feat: add chat gateway search tools`) across `chat.go`, `chat_stream.go`, `chat_test.go`, `chat_validate.go`, `chat_wire.go`, `contract_external_test.go`, `doc.go`, `docs/generation.md`, and `planning/IMPLEMENTATION_PLAN.md`. Review fixes landed in `ef840df` (`fix: correct chat search request handling`) across `chat_test.go`, `chat_validate.go`, `chat_wire.go`, `contract_external_test.go`, and `planning/IMPLEMENTATION_PLAN.md`. The nil value-form validation and slice-presence correction landed in `8bd93e9` (`fix: preserve chat search slice presence`) across `chat_test.go`, `chat_validate.go`, `chat_wire.go`, and `planning/IMPLEMENTATION_PLAN.md`. Public nil-union documentation and amended accounting landed in `8978cf6` (`docs: clarify chat search presence semantics`) across exactly `docs/generation.md` and `planning/IMPLEMENTATION_PLAN.md`. These four commits remain the substantive review boundary. Two independent post-amend re-reviews of HEAD `8978cf6` returned **CLEAN**. Final post-commit verification was completed at HEAD `1c2ac97`; final tracker acceptance and closure are recorded by the tracker-only boundary identified self-relatively by intended Conventional Commit subject `docs: close evidence continuation`, containing exactly `planning/IMPLEMENTATION_PLAN.md`. Its final hash is intentionally omitted because embedding it in the plan contained by that same commit would be self-referential.

- [x] Pointer/value nil semantics correction is implemented and verified: a nil `ChatExaSubpageTarget` interface omits the option; value-form `ChatExaSubpageTargetStrings(nil)` and a non-nil pointer to a nil slice are present supported variants and emit `[]`; typed-nil pointers remain pre-network errors.
- [x] Finding/fix verification: the all-options fixture compares the complete decoded `tools` wire value, including every declared field and nil-versus-zero/false/empty behavior, rather than selected fragments.
- [x] Finding/fix verification: the external contract asserts the exact ordered exported `ChatCompletionRequest` field names and types while retaining the external unkeyed-literal compile check.
- [x] Durable boundary accounting preserves the four-commit substantive chain: `29af060` (`feat: add chat gateway search tools`) is the nine-file implementation commit; `ef840df` (`fix: correct chat search request handling`) is the five-file review-fix commit; `8bd93e9` (`fix: preserve chat search slice presence`) is the four-file nil-semantics fix commit containing `chat_test.go`, `chat_validate.go`, `chat_wire.go`, and `planning/IMPLEMENTATION_PLAN.md`; and `8978cf6` (`docs: clarify chat search presence semantics`) contains exactly `docs/generation.md` and `planning/IMPLEMENTATION_PLAN.md`, supplying public nil-union documentation and amended tracker accounting. The final tracker-only closure boundary is identified by intended subject `docs: close evidence continuation` and contains exactly `planning/IMPLEMENTATION_PLAN.md`. Its hash is deliberately not recorded because a plan cannot embed the final hash of the commit that contains that plan without creating a self-reference; this one-file closure boundary does not become a fifth substantive commit.
- [x] Value-form nil-slice finding fixed, committed, and verified: both `ChatExaSubpageTargetStrings(nil)` and pointer-to-nil `*ChatExaSubpageTargetStrings` emit `subpage_target:[]`, while a nil interface omits the option and a typed-nil pointer rejects before dispatch. Focused Chat/API, full-suite, focused-race, and workspace LSP evidence passed.

- [x] First independent review and finding/fix accounting complete with the successful post-fix evidence recorded above.
- [x] Acceptance review found the code/API clean, and the public nil-union documentation plus corrected four-commit substantive boundary accounting are present at HEAD `8978cf6`.
- [x] Clean independent post-amend re-review complete: both reviewers returned **CLEAN** at HEAD `8978cf6`, with no actionable code, API, documentation, verification, or accounting finding.
- [x] Final Chunk 25 acceptance and four-commit substantive review-boundary closure complete after the two clean post-amend reviews; the one-file tracker-only closure boundary is recorded separately and self-relatively above.

**Final split review accounting:**

- [x] Final split reviews 1–3 found no actionable code/API, test, security/privacy, or secret-handling issue; those clean findings remain closed and are not reopened by documentation/accounting corrections.
- [x] **Historical Chunk 25 review resolution:** final split review 4's repository-documentation finding was resolved. At that stage the documentation fix set distinguished the implemented four Chat request declarations from typed Chat output blockers, optional live corroboration, and the then-blocked Responses built-in-search and Gateway-native `x_search` request gates. The later search-request continuation supersedes only those two request-gate statuses; its narrower blockers remain current.
- [x] Final split review 4's tracker-accounting finding is resolved. The two blocked Chunk 09 live acceptance rows now use the legend-consistent `[!]` marker, the continuation release checklist identifies the four Chat request declarations as implemented and accepted in Chunk 25 while preserving Chunk 24's external blockers, and this rereview's only tracker finding—the stale `[!]` status on these already corrected documentation/accounting rows—was fixed by marking both resolved findings `[x]`.
- [x] The final external-consumer split-review finding is fixed and verified, not erased. The first final gate run correctly rejected the newly supported API because `scripts/verify-local-consumer.sh` still had the pre-Chunk-25 exported-type allowlist and consumer compile exercise. The corrected strict gate now enforces bidirectional expected/export equality for types, functions, methods, and values: it continues to reject unexpected exports and additionally records encountered allowed declarations, reports sorted stale or missing expected entries, and exits nonzero after the scan. The external consumer now directly compile-references all four exported union interfaces: `ChatExaText`, `ChatExaHighlights`, `ChatExaSubpageTarget`, and `ChatPerplexityQuery`. With `AI_GATEWAY_API_KEY`, `VERCEL_OIDC_TOKEN`, `AI_GATEWAY_LIVE_COST_ACK`, and `AI_GATEWAY_PUBLIC_LIVE_COST_ACK` unset, the corrected strict `./scripts/verify-local-consumer.sh` gate reran successfully; `git diff --check` also passed. That scoped proof preceded, and is now supplemented by, the completed post-commit full-repository verification recorded below.
- [x] **Historical Chunk 25 closure verification:** full-repository verification completed at HEAD `1c2ac97` with `AI_GATEWAY_API_KEY`, `VERCEL_OIDC_TOKEN`, `AI_GATEWAY_LIVE_COST_ACK`, and `AI_GATEWAY_PUBLIC_LIVE_COST_ACK` unset. `go test ./...`, `go test -race -count=1 ./...`, `go vet ./...`, both example packages, `go doc -all .`, and `./scripts/verify-local-consumer.sh` passed; workspace Go LSP diagnostics reported `No issues found`. At that closure, public `/v1/evaluate`, Responses built-in search, typed Chat search outputs, Gateway-native `x_search`, optional Chat paid corroboration, and Chunk 24/release gates remained `[!]`. The later search-request continuation supersedes only fixed low-context Responses `web_search` and fieldless Gateway `x_search` request support; all other named blockers remain.

### Optional future Chat live corroboration — not an implementation chunk

- [!] No live test, workflow, evidence file, sentinel, model, or protected job is owned by Chunk 25; therefore no live run or live-status row is part of its acceptance. Hermetic request serialization is the complete implementation proof for this chunk.
- [!] If the owner later requests paid corroboration, first add a separate planned evidence chunk naming exact ownership for a build-tagged test under `internal/livecontract`, `.github/workflows/live-contract.yml`, and the retained evidence document; define a fail-closed sentinel, approved model, exact sanitized record, and four independent identifier cases. Evidence for one identifier must not clear another. Until that plan exists, do not append ad hoc live work to Chunk 25.

### Gate C — typed Chat search outputs remain blocked

- [!] The first-party Chat page promises the synthesized final answer and says Gateway executes the search internally; it does not publish exact buffered `gatewayToolCalls`/cost metadata schema, stream placement, search lifecycle events, structured citations/offsets, refusal behavior, or provider-error mapping. Released helper output unions are not Chat wire outputs.
- [!] Unblock each typed field/event only with identifier-specific first-party Chat REST schema/source/test or an approved fixture contract establishing exact discriminator/path/type/presence/nullability/cardinality/unknown-field and streaming termination semantics. Keep existing raw boundaries unchanged until then.

### Historical Gate D — Gateway-native xAI `x_search` request support was blocked here; later narrowly superseded

- [!] **Historical evidence state:** first-party Gateway evidence then showed Responses compatibility and X Search capability, while direct `@ai-sdk/xai@5.0.4` separately described xAI's own options and `x_search_call` schemas; released `@ai-sdk/gateway@4.0.87` had no helper or native-tool schema.
- [!] At that stage no Vercel Gateway document or authorized probe established fieldless `{"type":"x_search"}`. The later search-request continuation records the authorized Gateway probe and clears only that fieldless request declaration through the opt-in wrapper.
- [!] Configurable options, typed outputs/events, wider model compatibility, direct-xAI support, and unrelated surfaces remain blocked; direct-provider schemas and generic model capability still cannot authorize them.

### Historical completion accounting before the search-request continuation

- [!] Public `/v1/evaluate`: blocked on an exhaustive first-party strict public-HTTP contract covering success/error properties, requiredness, presence/nullability, numeric constraints, duplicate/additional-key behavior, and status/error variants; no exported method or DTO is authorized.
- [!] **Historical completion state, later superseded for one request form:** Responses built-in search was blocked on Gateway-specific request, output, and event evidence. The later continuation clears and implements only fixed low-context `{"type":"web_search","search_context_size":"low"}` through the opt-in wrapper; preview/other forms, configurable options, typed outputs/events, and wider model compatibility remain blocked.
- [x] Chat server search request support: Chunk 25 is complete. It adds a source-compatible opt-in wrapper and two methods without changing `ChatCompletionRequest` or existing method signatures, plus hermetic proof, documentation, and tracker accounting across the preserved four-commit substantive review boundary: nine-file implementation commit `29af060` (`feat: add chat gateway search tools`); five-file review-fix commit `ef840df` (`fix: correct chat search request handling`); four-file nil-semantics fix commit `8bd93e9` (`fix: preserve chat search slice presence`) containing `chat_test.go`, `chat_validate.go`, `chat_wire.go`, and `planning/IMPLEMENTATION_PLAN.md`; and two-file documentation/accounting commit `8978cf6` (`docs: clarify chat search presence semantics`) containing exactly `docs/generation.md` and `planning/IMPLEMENTATION_PLAN.md`. Two independent post-amend re-reviews returned **CLEAN** at `8978cf6`, and final repository verification passed at HEAD `1c2ac97`. Final acceptance is recorded in the tracker-only boundary identified self-relatively by intended subject `docs: close evidence continuation`, containing exactly `planning/IMPLEMENTATION_PLAN.md`; no self-hash is embedded because that would be self-referential.
- [!] Typed Chat search outputs: blocked on identifier-specific first-party output and event schemas defining exact discriminators, paths, types, requiredness, nullability, cardinality, unknown-field/variant behavior, ordering, and stream termination for each of `vercel:exa_search`, `vercel:parallel_search`, `vercel:perplexity_search`, and `vercel:tako_search`.
- [!] Optional Chat paid corroboration: outside implementation acceptance and not locally actionable; no live test, workflow, evidence file, sentinel, approved model, protected job, or owner authorization is owned by Chunk 25.
- [!] **Historical completion state, later superseded for the fieldless request:** Gateway-native xAI `x_search` was blocked before probe and implementation. The later continuation clears and implements only fieldless Gateway `{"type":"x_search"}` through the opt-in wrapper; configurable options, typed outputs/events, wider model compatibility, direct-xAI, and unrelated surfaces remain blocked.
- [x] Historical Chunks 01–23 and every locally doable continuation item are complete. [!] Chunk 24 and release remain blocked, unchanged, on protected hosted CI/environment evidence, both owner-authorized paid live contracts, owner-selected license, hosted repository metadata/provenance/permissions/publication review, hosting authorization, immutable continuation tagging, direct-VCS/public-proxy/checksum evidence, and final promotion/publication. No locally doable requested implementation remains, and no release gate is marked closed.

## 2026-09-21 search-request continuation — superseding current authority

This section supersedes every earlier current-status statement in this plan that says Responses built-in search or Gateway-native `x_search` is wholly unimplemented, wholly unsupported, or wholly evidence-blocked. Earlier rows remain preserved history and must not be deleted or rewritten as if their evidence had existed when they closed. In particular, the historical continuation at lines 814–820, historical `x_search` stages at lines 919–923, Gate B at lines 1155–1164, Gate D at lines 1531–1535, and the corresponding completion-accounting rows are superseded for **request support only**. They remain authoritative blockers for every typed search output, citation, annotation, search-call lifecycle event, and streaming interpretation not expressly cleared below. This continuation does not reopen Chunks 01–25, alter Chunk 24's external release blockers, or broaden the SDK into direct xAI client support or generic OpenAI SDK parity.

### Evidence reconciliation and exact cleared boundary

- [x] **Responses `web_search` request gate is narrowly cleared.** The current first-party Vercel page <https://vercel.com/docs/ai-gateway/models-and-providers/web-search> (page front matter `last_updated: 2026-09-08`; freshly inspected 2026-09-21) now gives an exact public `POST https://ai-gateway.vercel.sh/v1/responses` example using model `openai/gpt-5.4-mini` and tool `{"type":"web_search","search_context_size":"low"}`. Because the mutable page retained its prior date while its reviewed content changed, record the canonical URL, inspection date, and exact request rather than claiming a reliable publication/change date or immutable upstream version.
- [x] **Owner-authorized `web_search` live corroboration is positive, with one retained-artifact sanitation exception.** On 2026-09-21, a minimal authenticated Gateway probe sent only the documented tool form above. Sanitized structural result: HTTP 200; top-level `object:"response"`; response status `incomplete` under the deliberately low output-token cap; output item discriminators included `web_search_call` and `message`; no API error. The credential, Authorization value, generated prose, complete body, IDs, and raw response were not retained. However, `agent://search-evidence-1` retains the exact probe input text in `sanitized_request.body.input` for both the `web_search` and `x_search` probes. That agent report is therefore non-publishable internal orchestration evidence, not a repository evidence record. No prompt text from it may be copied, quoted, paraphrased, or otherwise transferred into repository evidence. This proves current request acceptance/execution, but it does not define a typed `web_search_call`, citation, annotation, result, error, or event contract.
- [x] **Gateway-native `x_search` request discriminator is narrowly cleared for SpaceXAI Responses.** On 2026-09-21, an owner-authorized minimal authenticated probe sent exactly `{"type":"x_search"}` to public Gateway `/v1/responses` with `spacexai/grok-4.6` and general `tool_choice:"required"`. Sanitized structural result: HTTP 200; top-level `object:"response"`; response status `completed`; output item discriminators included a completed `x_search_call`, reasoning items, and a completed message; no API error. Subject to the retained-prompt exception above, this supersedes the former Gate D claim that no authorized Gateway discriminator probe existed and clears only the fieldless request declaration.
- [!] **Configurable `x_search` options remain blocked.** Direct `@ai-sdk/xai@5.0.4` source and <https://docs.x.ai/developers/tools/x-search> provide candidate direct-xAI vocabulary only; they do not prove the public Gateway request contract. Do not expose `allowed_x_handles`, `excluded_x_handles`, `from_date`, `to_date`, `enable_image_understanding`, `enable_video_understanding`, or any validation, limits, mutual-exclusion rule, date rule, null/presence behavior, or compatibility promise for them. Unblock an exact field or inseparable option family only with Gateway-specific first-party documentation/schema/source/test, or an owner-authorized sanitized Gateway probe that exercises that exact field, presence form, accepted and rejected boundary, and relevant model route. Evidence and invariants from different direct-xAI contract versions must remain separate and must never be blended into a Gateway contract.
- [!] Direct-xAI handle limits and invariants are specifically non-authoritative for Gateway: pinned SDK source says 10 handles and does not encode mutual exclusion, while current xAI prose says 20 and forbids combining allowed and excluded lists. Neither version authorizes a Gateway limit or local invariant, and fail-closed local invention is not a substitute for Gateway evidence.
- [!] `web_search_preview`; omission of `search_context_size`; values other than exact `"low"`; external-web-access flags; filters/domains; approximate location; search-specific tool-choice or `allowed_tools` selectors; fallback/routing promises; and current/preview coexistence remain blocked until exact first-party Gateway request evidence clears each field or behavior.
- [!] Typed buffered outputs remain blocked. `ResponseResult.RawJSON()` remains the complete buffered output boundary. The structural live discriminators prove execution only; they do not establish complete fields, requiredness, nullability, unknown-field/variant behavior, citations, annotations, actions, source lists, refusals, provider errors, usage/cost, or retention semantics.
- [!] Typed streaming search lifecycle remains blocked. `ResponseOutputTextDeltaEvent` and `RawResponseEvent` remain the complete streaming boundary. Do not add `response.web_search_call.*`, `response.x_search_call.*`, citation/source events, ordering rules, completion effects, or terminal/error interpretations without exact Gateway SSE evidence.
- [!] Direct `https://api.x.ai/v1/responses` client support, direct-xAI authentication/base URLs, AI SDK provider-tool result normalization, and generic OpenAI-compatible tool parity remain out of scope.

### Dependency and commit sequence

The chunks below executed sequentially as one encompassing Responses search-request feature item: 26 → 26A → 27 → 27A → 28 → 29 → 30. Chunks 26–29, including the narrowly split 26A and 27A external-contract/audit subchunks, each serialized their draft/documentation hash, exact changed-path inventory, focused verification, review-fix hashes, and clean local rereview result in `planning/IMPLEMENTATION_PLAN.md` before the successor began. Each successor verified and recorded the immediately preceding tracker-only accounting commit's landed full hash, exact subject, and sole tracker path under the acyclic accounting rule; each predecessor recorded only its intended subject and sole path, never its cryptographically impossible self-hash. Chunk 30 consumed those durable records and closed all six predecessor chunks `[x]` under the same finite rule.

### Chunk 26 — Responses `web_search` low-context request declaration

**Depends on:** Chunks 15–16 and 25 `[x]`; evidence reconciliation above `[x]`. Chunk 24's external release gates may remain `[!]` under the established sequence exception.

**Owned paths (exact five):** `responses.go`, `responses_wire.go`, `responses_validate.go`, `responses_test.go`, `planning/IMPLEMENTATION_PLAN.md`.

**Draft commit boundary:** `feat: add responses web search request`. **Serialized accounting commit boundary:** `docs: record responses web search implementation`.

**Current status:** `[x]` implementation, implementation-time verification, both final independent CLEAN rereviews, complete commit/path accounting through `97942dc`, and the encompassing Chunk 30 closure are complete. No actionable Chunk 26 finding remains.

**Draft implementation changed paths (exact four):** `responses.go`, `responses_wire.go`, `responses_validate.go`, `responses_test.go`.

**Exported API decision:** preserve the exact exported field order and types of `ResponsesRequest` so external unkeyed literals remain source-compatible. Add an opt-in wrapper and distinct methods rather than adding a field to the existing struct:

```go
type ResponsesBuiltInToolsRequest struct {
    Request ResponsesRequest
    Tools   []ResponseBuiltInTool
}

type ResponseBuiltInTool interface{ responseBuiltInTool() }

// ResponseWebSearchTool has no configurable fields. It always emits the only
// proven Gateway form: {"type":"web_search","search_context_size":"low"}.
type ResponseWebSearchTool struct{}

func (c *Client) CreateResponseWithBuiltInTools(context.Context, ResponsesBuiltInToolsRequest) (*ResponseResult, error)
func (c *Client) StreamResponseWithBuiltInTools(context.Context, ResponsesBuiltInToolsRequest) (*ResponseStream, error)
```

- [x] Implement the sealed Responses-only declaration above. `ResponseWebSearchTool{}` must be the sole public web-search configuration; do not export a free string, option map, `SearchContextSize` enum, preview type, or generic built-in-tool escape hatch.
- [x] Encode existing function tools first and wrapper built-in tools second, each in caller order. The exact web-search JSON is `{"type":"web_search","search_context_size":"low"}`. Preserve existing omission behavior when both tool collections are empty and preserve the existing SDK-owned `stream:false`/`stream:true` distinction.
- [x] Reuse existing `ResponsesRequest` validation unchanged, then validate wrapper tools before credentials/network. Reject nil and typed-nil interface elements, unsupported private variants, and a combined function-plus-built-in tool count above the existing 10,000-member ceiling with canonical `ValidationError` paths. Duplicate declarations remain caller-visible and serialize in order; do not silently deduplicate.
- [x] Preserve existing generic Responses `tool_choice` and `allowed_tools` request behavior only as already exported; do not add a search-specific selector, name, or compatibility promise. Focused web-search examples and tests must omit both because their search-specific semantics are unproven.
- [x] Keep buffered and streaming output APIs byte-for-byte unchanged. Hermetic fixtures may include raw `web_search_call` JSON only to prove it remains accessible through `RawJSON`/`RawResponseEvent`; tests must not decode or assert undocumented child fields.
- [x] Add exact minimum JSON, mixed function+built-in ordering, duplicate ordering, buffered/stream request parity, nil/typed-nil/unsupported/combined-limit zero-dispatch failures, raw-output preservation, and unchanged legacy request tests. Compile-lock the wrapper, marker implementation, both method signatures, unchanged `ResponsesRequest` field inventory/order, and unchanged result/event APIs in the owned implementation tests; Chunk 26A owns the independent external-consumer compile lock and strict export audit.
- [x] Document in Go comments that the first-party public-HTTP compatibility evidence is OpenAI-provider native search and names `openai/gpt-5.4-mini`; do not hard-code a model catalog or reject future model IDs locally. State that callers must choose a compatible OpenAI model and that Gateway routing/fallback compatibility is not promised.

**Implementation-time verification commands:**

```sh
env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go test -run 'Test(CreateResponse|StreamResponse|ResponsesBuiltIn)' ./...
env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go test -race -run 'Test(CreateResponse|StreamResponse|ResponsesBuiltIn)' ./...
env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go test ./...
```

**Recorded draft verification (`0295db6`):** `gofmt` completed. Each credential-scrubbed command above passed exactly as shown: the focused test command, the race-focused test command, and the full Go test command. Go LSP diagnostics were attempted but unavailable because no Go language server is installed.

**Recorded independent review finding and fix (`25e9a08`):**

- [x] Two independent API/wire reviews confirmed that the value-receiver marker makes both `ResponseWebSearchTool{}` and a non-nil `*ResponseWebSearchTool` valid public `ResponseBuiltInTool` values, but the validator and encoder accepted only the value form. The defect was owned by `responses_validate.go` and `responses_wire.go`; `responses_test.go` owns the value/pointer wire-parity regression coverage. Typed-nil pointers remain rejected before credentials or network work.
- [x] Fix commit `25e9a08` (`fix: accept pointer web search tools`) changed exactly `responses_validate.go`, `responses_wire.go`, and `responses_test.go`.
- [x] Post-fix `gofmt` completed, and all three exact credential-scrubbed gates passed:

```sh
env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go test -run 'Test(CreateResponse|StreamResponse|ResponsesBuiltIn)' ./...
env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go test -race -run 'Test(CreateResponse|StreamResponse|ResponsesBuiltIn)' ./...
env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go test ./...
```

Go LSP diagnostics remain unavailable from the prior attempt because no Go language server is installed.

**Recorded rereview finding and test-only fix (`d3dd182`):**

- [x] The independent rereview confirmed the validator and encoder accept both value and non-nil pointer forms while preserving typed-nil rejection, the fixed low-context wire shape and ordering, unchanged raw buffered/SSE boundaries, and validation before credentials/network. It found that the non-nil pointer regression exercised only the internal encoder, so a future validator regression could still break both public methods without failing the test.
- [x] Test-only fix commit `d3dd182` changed exactly `responses_test.go` and adds regression coverage that sends the non-nil pointer form through both public paths: `CreateResponseWithBuiltInTools` for buffered generation and `StreamResponseWithBuiltInTools` for streaming generation. This makes validation plus encoding observable for the fixed public behavior.
- [x] Post-fix `gofmt` completed, and all three exact credential-scrubbed gates passed:

```sh
env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go test -run 'Test(CreateResponse|StreamResponse|ResponsesBuiltIn)' ./...
env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go test -race -run 'Test(CreateResponse|StreamResponse|ResponsesBuiltIn)' ./...
env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go test ./...
```

The test-only fix received two fresh CLEAN independent final rereviews; neither found an actionable defect.

**Complete commit/path chain through final pre-rereview accounting:**

- [x] `f91352e` changed only `planning/IMPLEMENTATION_PLAN.md`.
- [x] `0295db6` (`feat: add responses web search request`) changed exactly `responses.go`, `responses_wire.go`, `responses_validate.go`, and `responses_test.go`.
- [x] `ca37703` changed only `planning/IMPLEMENTATION_PLAN.md`.
- [x] `25e9a08` (`fix: accept pointer web search tools`) changed exactly `responses_validate.go`, `responses_wire.go`, and `responses_test.go`.
- [x] `56da6b6` changed only `planning/IMPLEMENTATION_PLAN.md`.
- [x] `d3dd182` changed exactly `responses_test.go`.
- [x] `97942dc` changed only `planning/IMPLEMENTATION_PLAN.md`.

**Recorded final clean rereviews:**

- [x] `agent://chunk26-final-review-1` re-audited the complete Chunk 26 chain through `97942dc` across `responses.go`, `responses_wire.go`, `responses_validate.go`, `responses_test.go`, and the current tracker accounting. It confirmed source compatibility, the sealed no-options built-in-tool surface, value/non-nil-pointer parity with nil and typed-nil rejection before credential/network access, exact ordering and low-context encoding, combined limits and canonical validation paths, unchanged raw buffered/SSE boundaries and legacy encoding, and sufficient focused regression coverage. It found no actionable patch-introduced defect.
- [x] `agent://chunk26-final-review-2` independently re-audited the same complete chain and exact path inventory. It confirmed no security, API/wire, test, or accounting defect; no configurable or generic search escape hatch; public buffered and streaming pointer-path coverage; accurate credential-scrubbed verification accounting; and no skipped locally doable Chunk 26 task. It found no actionable finding and confirmed the clean independent rereview gate is satisfied.

**Review/accounting gate:**

- [x] Two clean independent API/wire final rereviews confirm the pointer-form fix plus the fixed low-context shape, source compatibility, zero undocumented knobs, validation-before-credential/network, unchanged raw output/event boundaries, no direct-provider output inference, and no remaining actionable finding.
- [x] The tracker records the complete draft, review-fix, test-fix, and accounting hash/path chain through `97942dc`, the exact credential-scrubbed focused verification results, and both clean final rereviews. This satisfied the serialized local review/accounting gate, permitted Chunk 26A to begin, and is now encompassed by Chunk 30's `[x]` closure.
- [x] Rollback ownership is the wrapper, its two methods, encoder/validator branches, focused tests, Chunk 26A's external contract/audit, and the matching `web_search` portions of Chunks 28–29. These paths form the atomic `web_search` rollback group defined in Chunk 30: they are removed in one committed state only after the `x_search` group is gone, restoring the prior function-only Responses request surface and unsupported-status documentation without changing shared transport, results, or streams.

### Chunk 26A — `web_search` external-consumer contract and strict audit

**Depends on:** Chunk 26 implementation, focused verification, committed accounting, and clean code/API rereview were recorded before Chunk 26A began; Chunk 30 has now closed both chunks `[x]`.

**Current status:** `[x]` implementation-time verification, all review fixes, both fresh independent CLEAN rereviews, complete commit/path/fix/accounting chain through `a16b990`, and the encompassing Chunk 30 closure are complete. No actionable Chunk 26A finding remains.

**Owned paths (exact three):** `contract_external_test.go`, `scripts/verify-local-consumer.sh`, `planning/IMPLEMENTATION_PLAN.md`.

**Draft commit boundary:** `test: audit responses web search exports`. **Serialized accounting commit boundary:** `docs: record responses web search audit`.

**Recorded green draft (`4dced7c`):** commit `4dced7c` (`test: lock web search external contract`) changed exactly `contract_external_test.go` and `scripts/verify-local-consumer.sh`. `gofmt` completed on `contract_external_test.go`. Both exact credential-scrubbed verification commands passed:

```sh
env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go test -run 'TestExternalContract' ./...
env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK ./scripts/verify-local-consumer.sh
```

**Recorded independent-review findings and green fix (`b902f48`):** the review confirmed three gaps: the external contract proved concrete assignability but did not lock `ResponseBuiltInTool` as an exactly single-method sealed interface; it did not lock the unchanged exported field order, names, and types of legacy `ResponsesRequest`; and the strict AST audit did not inspect exported methods declared inside exported interfaces. Commit `b902f48` fixed all three and changed exactly `contract_external_test.go` and `scripts/verify-local-consumer.sh`. The strengthened contract now reflection-locks `ResponseBuiltInTool` to the sole unexported zero-argument/zero-result `responseBuiltInTool` marker, reflection-locks the complete legacy `ResponsesRequest` field inventory/order/names/export status/types, and makes the strict expected/export audit inventory exported interface members (including the expected `TokenSource.Token`) while rejecting any exported member added to `ResponseBuiltInTool`. `gofmt` completed on `contract_external_test.go`. Both exact credential-scrubbed post-fix verification commands passed:

```sh
env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go test -run 'TestExternalContract' ./...
env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK ./scripts/verify-local-consumer.sh
```

**Recorded selector-embedding rereview finding and green fix (`72aaae0`):** the rereview found that `scripts/verify-local-consumer.sh` did not reject selector-based exported interface embeddings because its interface-member audit did not recognize `*ast.SelectorExpr`, allowing qualified embeddings such as `io.Closer` to escape the claimed bidirectional export inventory. Commit `72aaae0` changed exactly `scripts/verify-local-consumer.sh`. The audit now rejects qualified exported interface embeddings and their pointer, indexed, multi-indexed, and parenthesized forms, preserves the qualified name in diagnostics, and includes a hermetic self-check covering qualified, pointer-qualified, and indexed-qualified embeddings. Both exact credential-scrubbed Chunk 26A gates passed afterward:

```sh
env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go test -run 'TestExternalContract' ./...
env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK ./scripts/verify-local-consumer.sh
```

**Recorded final-rereview findings and green fix (`949f95c`):** both final rereviews found remaining false-negative paths in `scripts/verify-local-consumer.sh`. The first found that the audit ignored unexported interface methods, so removing or changing a sealed public interface's marker could pass; the second found that embedding a private local interface could promote exported methods into a public interface while escaping the exported-member inventory. Commit `949f95c` (`test: lock sealed interface contracts`) changed exactly `scripts/verify-local-consumer.sh`. The audit now records and enforces the exact unexported marker name and formatted function signature for every sealed exported interface (`Question.questionType func() string`, `Answer.answerType func() string`, `ResponseBuiltInTool.responseBuiltInTool func()`, `ResponseInput.responseInput func()`, `ResponseInputItem.responseInputItem func()`, `ResponseToolChoice.responseToolChoice func()`, `ResponseTextFormat.responseTextFormat func()`, `ResponseEvent.responseEvent func()`, `ChatServerTool.chatServerTool func()`, `ChatExaText.chatExaText func()`, `ChatExaHighlights.chatExaHighlights func()`, `ChatExaSubpageTarget.chatExaSubpageTarget func()`, `ChatPerplexityQuery.chatPerplexityQuery func()`, `ChatMessageContent.chatMessageContent func()`, `ChatContentPart.chatContentPart func()`, `ChatStop.chatStop func()`, `ChatToolChoice.chatToolChoice func()`, and `ChatResponseFormat.chatResponseFormat func()`), rejects missing, changed, or unexpected unexported methods, and rejects every interface embedding regardless of visibility or expression form. This includes private local embeddings whose exported methods would otherwise be promoted into the public method set. Hermetic mutation self-checks prove that the exact marker is accepted while marker removal, marker-signature mutation, and private embedding are rejected; the existing qualified/pointer/indexed embedding self-checks remain in force. Both exact credential-scrubbed Chunk 26A gates passed afterward:

```sh
env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go test -run 'TestExternalContract' ./...
env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK ./scripts/verify-local-consumer.sh
```
**Complete commit/path/fix/accounting chain through final pre-rereview accounting (`a16b990`):**

- `4dced7c` (`test: lock web search external contract`) changed exactly `contract_external_test.go` and `scripts/verify-local-consumer.sh`; `74d72ce` changed exactly `planning/IMPLEMENTATION_PLAN.md` to serialize that green draft and its credential-scrubbed gates.
- `b902f48` changed exactly `contract_external_test.go` and `scripts/verify-local-consumer.sh` to lock the sealed marker, legacy `ResponsesRequest` fields, and exported interface methods; `6ff6772` changed exactly `planning/IMPLEMENTATION_PLAN.md` to serialize those findings, fixes, paths, and passing gates.
- `72aaae0` changed exactly `scripts/verify-local-consumer.sh` to reject selector-based and wrapped qualified interface embeddings; `0243f6c` changed exactly `planning/IMPLEMENTATION_PLAN.md` to serialize that finding, fix, path, and passing gates.
- `949f95c` (`test: lock sealed interface contracts`) changed exactly `scripts/verify-local-consumer.sh` to enforce exact unexported marker signatures and reject private and all other interface embedding forms; `a16b990` changed exactly `planning/IMPLEMENTATION_PLAN.md` to serialize both final-rereview findings, the fix, exact path, and both passing credential-scrubbed gates.

**Recorded final clean rereviews:**

- [x] `agent://chunk26a-clean-review-1` re-audited the complete strengthened external contract through `a16b990` and returned **CLEAN**. It confirmed the exact wrapper and fieldless-tool shapes, both public method signatures, the sealed `ResponseBuiltInTool` marker, unchanged legacy `ResponsesRequest` inventory, strict bidirectional export/interface/method accounting, rejection of all embedding forms, and the offline external-consumer exercise; both exact credential-scrubbed Chunk 26A gates passed.
- [x] `agent://chunk26a-clean-review-2` independently re-audited the complete Chunk 26/26A chain through `a16b990`, verified every 26A commit's exact changed-path inventory and serialized gate evidence, and returned **CLEAN**. It found no security, portability, API, wire, test, or accounting defect; confirmed all prior findings are resolved by `b902f48`, `72aaae0`, and `949f95c`; and confirmed that no Chunk 27 or later `x_search` scope has landed.

**Review/accounting gate:**

- [x] Both fresh independent rereviews are clean and no actionable Chunk 26A finding remains.
- [x] The tracker records the complete draft, review-fix, selector-fix, sealed-interface-fix, and serialized accounting hash/path chain through `a16b990`, together with both credential-scrubbed gate results and both clean rereviews. This satisfied the serialized local review/accounting gate, made Chunk 26A dependency-ready for Chunk 27, and is now encompassed by Chunk 30's `[x]` closure.

- [x] Extend the external compile contract for `ResponsesBuiltInToolsRequest`, sealed `ResponseBuiltInTool`, `ResponseWebSearchTool`, and both `CreateResponseWithBuiltInTools` and `StreamResponseWithBuiltInTools` method signatures while compile-locking unchanged legacy request/result/event contracts. Reflection-lock the sealed interface's exact sole unexported zero-argument/zero-result marker, the wrapper's exact fields, the fieldless tool, and the complete legacy `ResponsesRequest` exported field order/names/types.
- [x] Update the strict bidirectional expected-export inventory and external temporary-consumer compile exercise for the wrapper, web-search tool, and both methods. Inventory exported interface members as well as top-level declarations, retain the expected `TokenSource.Token` member, and reject any exported `ResponseBuiltInTool` member, missing expected declaration, unexpected export, alias, configurable search knob, or inferred typed output/event API.
- [x] Run the focused external contract test and `./scripts/verify-local-consumer.sh` with all credentials and live-cost acknowledgements unset.
- [x] Independently review the expected/export equality and consumer exercise, then cleanly rereview every fix. Both fresh rereviews returned **CLEAN** with no actionable finding.
- [x] Before Chunk 27 starts, commit to the tracker the draft/review-fix hashes, exact changed paths, focused command results, and clean rereview result. The complete chain through `a16b990` and both clean rereviews are recorded above, satisfying the serialized dependency gate. Chunk 26A belongs to the atomic `web_search` rollback group with Chunk 26 and the surviving `web_search` claims/accounting; it is never committed as rolled back separately from that API surface.

### Chunk 27 — Gateway-native SpaceXAI `x_search` request declaration

**Current status:** `[x]` implementation-time verification, evidence/API review, both final CLEAN reviews, both supporting CLEAN evidence adjudications, complete `79137a6` → `3c313d5` → `5c2c7c3` accounting, rollback ownership, and the encompassing Chunk 30 closure are complete. The stale removal finding remains discarded for the recorded superseded-premise reason, and no actionable Chunk 27 finding remains.

**Depends on:** Chunk 26A's external contract, strict audit, verification, committed accounting, and clean rereview were recorded before Chunk 27 began; Chunks 26 and 26A are now closed `[x]`. The positive `x_search_call` discriminator evidence above is `[x]`.

**Owned paths (exact five):** `responses_tools.go`, `responses_wire.go`, `responses_validate.go`, `responses_test.go`, `planning/IMPLEMENTATION_PLAN.md`.

**Draft commit boundary:** `feat: add responses x search request`. **Serialized accounting commit boundary:** `docs: record gateway x search implementation`.

**Green draft commit:** `79137a6` (`feat: add responses x search request`) changed exactly `responses_tools.go`, `responses_wire.go`, `responses_validate.go`, and `responses_test.go`. The four changed Go files were formatted with `gofmt`.

**Exported API decision:** extend the sealed `ResponseBuiltInTool` union without changing `ResponsesBuiltInToolsRequest` or either method:

```go
type ResponseXSearchTool struct{}
```

- [x] Extend the sealed Responses-only union with fieldless `ResponseXSearchTool{}`. It encodes exactly `{"type":"x_search"}` and no other member; no option fields, arbitrary maps, provider options, aliases, query/result configuration, or direct-xAI client behavior were added.
- [x] Reuse the wrapper validation established by Chunk 26. `ResponseXSearchTool{}` has no option-specific validation; only the shared nil/typed-nil/unsupported-union and combined tool-count rules apply before credentials/network. No direct-xAI handle limits, mutual-exclusion rules, calendar/date-order checks, null rules, or defaults were imported.
- [x] Preserve model policy as documentation rather than a closed runtime enum: `spacexai/grok-4.6` remains the exact positively probed route. The implementation does not claim all SpaceXAI models, fallback portability, or option compatibility and does not reject dynamic future model IDs locally; unsupported routes may fail server-side.
- [x] Add exact minimum JSON, mixed function+`web_search`+fieldless `x_search` ordering, duplicate ordering, typed-nil union and combined-limit zero-dispatch cases, buffered raw `x_search_call` preservation, streaming raw event preservation, and unchanged legacy/web-search regressions. No all-options, option-presence, date, handle-list, or option-validation tests were added.
- [x] Compile-lock the fieldless exported type and unchanged wrapper/method/result/event contracts in owned implementation tests. Chunk 27A still owns the independent external compile contract and strict consumer audit. No typed `x_search_call`, posts, actions, sources, citations, lifecycle events, server-side tool results, or configurable-option compatibility promise was added.

**Implementation-time verification commands:**

```sh
env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go test -run 'Test(CreateResponse|StreamResponse|ResponsesBuiltIn|XSearch)' ./...
env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go test -race -run 'Test(CreateResponse|StreamResponse|ResponsesBuiltIn|XSearch)' ./...
env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go test ./...
```

All three commands above passed against green draft `79137a6` with `AI_GATEWAY_API_KEY`, `VERCEL_OIDC_TOKEN`, `AI_GATEWAY_LIVE_COST_ACK`, and `AI_GATEWAY_PUBLIC_LIVE_COST_ACK` unset: focused `go test`, focused race `go test -race`, and full `go test ./...`.

**Recorded evidence adjudication and final clean reviews:**

- [x] `agent://chunk27-evidence-review-1` and `agent://chunk27-evidence-review-2` independently returned **CLEAN**. They confirmed that the authoritative search-request continuation supersedes the historical pre-probe request blocker only for the narrow fieldless declaration. The owner-authorized structural probe sent public `POST https://ai-gateway.vercel.sh/v1/responses` with model `spacexai/grok-4.6`, tools exactly `[{"type":"x_search"}]`, general `tool_choice:"required"`, and no `x_search` option fields. It returned HTTP 200, top-level `object:"response"`, status `completed`, no API error, and an output item with type `x_search_call` and status `completed`. This clears only `ResponseXSearchTool{}` encoding exactly `{"type":"x_search"}`; configurable direct-xAI fields and validation, broader model compatibility, typed buffered output, citations/actions/posts/sources, streaming lifecycle/events, direct-xAI client support, and generic extension points remain blocked.
- [x] The retained internal probe report preserves exact input prompts and is therefore non-publishable. That prompt-retention sanitation caveat does not invalidate the structural request/discriminator evidence; Chunks 28–29 retain responsibility for sanitized durable evidence and must not copy, quote, paraphrase, or claim non-retention of those prompts.
- [x] `agent://chunk27-rereview-1`'s finding, “Remove the unverified Gateway-native x_search API,” is **DISCARDED** because it relied on superseded pre-probe authority stating that Gateway-native `x_search` lacked Gateway evidence. The later authoritative continuation and the exact owner-authorized probe above supersede that premise for fieldless request support only while preserving every configurable-output/event blocker.
- [x] `agent://chunk27-clean-review-1` re-audited the complete Chunk 27 implementation/accounting chain through `5c2c7c3` and returned **CLEAN**. It confirmed the fieldless sealed API; exact value and non-nil-pointer encoding; nil, typed-nil, unsupported-union, and combined-limit pre-dispatch failures; caller ordering and duplicates; unchanged raw buffered/streaming boundaries; focused coverage; and absence of configurable options, inferred validation, typed output/event contracts, model allowlists, generic extension maps, or direct-xAI support.
- [x] `agent://chunk27-clean-review-2` independently returned **CLEAN**, confirmed the exact owned-path union and gate state, found no actionable implementation, API/wire, validation, security, test, or accounting defect, and confirmed that the serialized record made Chunk 27 dependency-ready for Chunk 27A. Chunk 30 has now closed Chunk 27 `[x]`.

**Complete commit/path and gate chain:**

- [x] `79137a623191f28a888c806c38e635c3a31458c8` (`feat: add responses x search request`) is the green implementation draft and changed exactly `responses_test.go`, `responses_tools.go`, `responses_validate.go`, and `responses_wire.go`. All three recorded credential-scrubbed gates passed against this draft: focused `go test`, focused `go test -race`, and full `go test ./...`.
- [x] `3c313d563f855def3016a994a877320b16254fb8` (`docs: record x search draft verification`) changed only `planning/IMPLEMENTATION_PLAN.md` and serialized the draft verification/accounting.
- [x] `5c2c7c3c8a9cdb5a952db435f25c375a902e5ce6` (`docs: fix x search commit accounting`) changed only `planning/IMPLEMENTATION_PLAN.md` and corrected the tracker to Git's exact draft subject. The observed union through this commit is exactly the four implementation paths above plus `planning/IMPLEMENTATION_PLAN.md`; no Chunk 27A or later-chunk path changed.

**Review/accounting gate:**

- [x] Independent evidence/API review maps the sole emitted `type:"x_search"` member to the exact owner-authorized structural Gateway probe, confirms `ResponseXSearchTool` is fieldless, and rejects every direct-xAI option, inferred validation rule, result/event contract, wider-model claim, and generic extension point.
- [x] The complete draft/accounting/correction hash, exact subject, exact changed-path, focused-verification, evidence-adjudication, stale-finding disposition, and two-clean-review chain is serialized above. This satisfied the local review/accounting gate, permitted Chunk 27A to begin, and is now encompassed by Chunk 30's `[x]` closure.
- [x] Rollback ownership is the fieldless `ResponseXSearchTool`, its encoder/validator branches, focused tests, Chunk 27A's external contract/audit, and the matching `x_search` portions of Chunks 28–29. These paths form the atomic `x_search` rollback group defined in Chunk 30; removing that group must leave Chunk 26/26A `web_search` support, audit, documentation, and accounting intact.

### Chunk 27A — `x_search` external-consumer contract and strict audit

**Current status:** `[x]` implementation-time verification, accounting corrections, both exact credential-scrubbed gates, the complete chain through `c60f04948409bd0feb2d43ff6a0cd258f9c96d32`, both fresh independent CLEAN rereviews, rollback ownership, and the encompassing Chunk 30 closure are complete. No actionable Chunk 27A finding remains.

**Depends on:** Chunk 27 implementation, focused verification, committed accounting, and clean evidence/API rereview were recorded before Chunk 27A began; the earlier chunks are now closed `[x]`.

**Owned paths (exact three):** `contract_external_test.go`, `scripts/verify-local-consumer.sh`, `planning/IMPLEMENTATION_PLAN.md`.

**Complete commit/path chain through the accounting correction:** `f9cb666ed78cd220717b4ceab08ac12a9d67ceb8` (`test: lock x search external contract`) changed exactly `contract_external_test.go` and `scripts/verify-local-consumer.sh`; `7a61d28799272979e6bc8aee980364a2e33c94a4` (`docs: record x search contract verification`) changed only `planning/IMPLEMENTATION_PLAN.md`; `c60f04948409bd0feb2d43ff6a0cd258f9c96d32` (`docs: fix x search contract accounting`) changed only `planning/IMPLEMENTATION_PLAN.md`. The complete Chunk 27A path union remains exactly the three owned paths above; no Chunk 28-owned documentation path changed.

**Recorded green draft:** `f9cb666ed78cd220717b4ceab08ac12a9d67ceb8` changed exactly the two implementation paths above. `gofmt` completed on `contract_external_test.go`. Both exact credential-scrubbed verification commands passed:

```sh
env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go test -run 'TestExternalContract' ./...
env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK ./scripts/verify-local-consumer.sh
```

- [x] Extended the external compile contract and strict expected-export inventory for fieldless `ResponseXSearchTool` while retaining the wrapper, `ResponseWebSearchTool`, and both built-in-tool methods added by 26A. The contract locks the exact fieldless shape and both value and non-nil-pointer tool forms.
- [x] Extended the temporary external-consumer compile exercise to construct both tool types in the wrapper and invoke both methods. Bidirectional expected/export equality remains enforced, including rejection of configurable x-search fields, generic escape hatches, aliases, and typed search output/event APIs.
- [x] Ran the focused external contract test and `./scripts/verify-local-consumer.sh` with all credentials and live-cost acknowledgements unset; both exact commands recorded above passed.
- [x] `agent://chunk27a-review-1` independently reviewed the complete wrapper/two-method/two-tool external contract, strict audit, offline verifier behavior, and exact commit/path accounting and returned **CLEAN** with no actionable implementation or contract defect.
- [x] `agent://chunk27a-review-2` independently reviewed implementation soundness, strict bidirectional export enforcement, security, portability, credential scrubbing, and offline behavior and returned **CLEAN** with no actionable defect.
- [x] `agent://chunk27a-review-3` found one accounting defect: the tracker abbreviated the draft hash, omitted its exact landed subject, and omitted the accounting commit's exact hash, subject, and tracker-only path. The exact two-commit chain and changed-path inventory above correct that finding without changing either implementation path or marking Chunk 27A complete.
- [x] `agent://chunk27a-rereview-1` re-audited the complete Chunk 27A chain through `c60f04948409bd0feb2d43ff6a0cd258f9c96d32` and returned **CLEAN**. It confirmed the fieldless exported tool, value/non-nil-pointer sealed-interface conformance, exact wrapper and method contracts, strict bidirectional export audit, offline compile-only consumer exercise, preserved `web_search` and legacy inventories, and accurate correction accounting; it found no correctness, security, offline-verification, or regression defect.
- [x] `agent://chunk27a-rereview-2` independently re-audited the same complete three-commit chain and returned **CLEAN**. It confirmed every full hash, exact subject, and changed-path boundary; both credential-scrubbed passing gates; the earlier clean reviews and corrected accounting finding; and the exact three-path ownership set. Chunk 30 has now closed Chunk 27A `[x]`.
**Review/accounting and rollback gate:**

- [x] Both fresh independent rereviews are clean and no actionable Chunk 27A implementation, contract, security, offline-verification, or accounting finding remains.
- [x] The tracker preserves the complete draft, initial accounting, and correction chain through `c60f04948409bd0feb2d43ff6a0cd258f9c96d32`, including every exact subject and changed-path boundary, both passing credential-scrubbed gates, the earlier review finding and its correction, and both clean fresh rereviews. This satisfied the serialized local review/accounting gate, made Chunk 27A dependency-ready for Chunk 28, and is now encompassed by Chunk 30's `[x]` closure.
- [x] Rollback ownership is satisfied: Chunk 27A belongs to the atomic `x_search` rollback group with Chunk 27 and the matching `x_search` claims/accounting. Rollback restores the Chunk 26A `web_search`-only external contract and strict audit in the same committed state that removes the fieldless `ResponseXSearchTool`, while leaving the surviving `web_search` API, audit, documentation, and accounting intact.

### Chunk 28 — Public search request documentation and evidence record

**Current status:** `[x]` the public request-only documentation, sanitized structural evidence record, changelog entry, documentation-time verification, review fixes, final local accounting through `9be6f1c6f30c6433f15601fa796ef88f123b1521`, finite handoff commit `b274715a2cd4c5fcc3ada4d43a8e62a09ee4e980`, CLEAN technical/release reviews, rollback ownership, and the encompassing Chunk 30 closure are complete. The discarded self-hash request remains cryptographically impossible and no follow-up self-hash commit is required.

**Depends on:** Chunks 26, 26A, 27, and 27A had committed their implementation/audit hashes, exact changed paths, verification, and clean rereviews before Chunk 28 began; all are now closed `[x]`.

**Owned paths (exact five):** `docs/generation.md`, `docs/x-search.md`, `README.md`, `CHANGELOG.md`, `planning/IMPLEMENTATION_PLAN.md`.

**Complete landed predecessor commit/path chain:** `311adef73795fefd2a793ebdd133d00fe140ee63` (`docs: describe responses search requests`) changed exactly `CHANGELOG.md`, `README.md`, `docs/generation.md`, and `docs/x-search.md`; `909eb0ec73e25e874cae3d316a1a8faf3b625164` (`docs: record search documentation verification`) changed only `planning/IMPLEMENTATION_PLAN.md`; `73cfd2ddece333f091b280d7996f3e6a7024d9c7` (`docs: correct search stream boundaries`) changed exactly `README.md` and `planning/IMPLEMENTATION_PLAN.md`; `e31153ffcf652854295d0b2d9441afbd7cb60a31` (`docs: record search documentation fixes`) changed only `planning/IMPLEMENTATION_PLAN.md`; `4c4eef8c778ce4ee855acab7a18872c5bb080adc` (`docs: close search documentation review`) changed only `planning/IMPLEMENTATION_PLAN.md`; `9be6f1c6f30c6433f15601fa796ef88f123b1521` (`docs: finalize search documentation accounting`) changed only `planning/IMPLEMENTATION_PLAN.md`. The new tracker-only handoff commit intentionally records no self-hash; its exact intended subject is `docs: close search documentation handoff` and its sole intended changed path is `planning/IMPLEMENTATION_PLAN.md`.

- [x] Add buffered and streaming contract examples for `ResponseWebSearchTool{}` and fieldless `ResponseXSearchTool{}`, showing the opt-in wrapper, existing function-tool coexistence, fixed web-search low context, exact x-search `{"type":"x_search"}` serialization, and raw result/event inspection without printing response prose.
- [x] Add a model-compatibility table: exact documented `web_search` route `openai/gpt-5.4-mini`; exact positively probed fieldless `x_search` route `spacexai/grok-4.6`; dynamic catalog/no universal-support claim; no Gateway fallback, configurable-x-search-option, or direct xAI client promise.
- [x] Replace every stale blanket support statement in these four owned documents with the precise request-only boundary: the two exact request declarations are supported; configurable `x_search` options and typed search calls/results/actions/posts/sources/citations/annotations/refusals/errors/usage/cost and lifecycle events remain blocked; `web_search_preview`, other current/preview forms, and undocumented request options remain blocked.
- [x] Add a changelog entry describing additive request-only support and the source-compatible wrapper; explicitly state that existing `ResponsesRequest`, `CreateResponse`, `StreamResponse`, `ResponseResult`, and `ResponseEvent` contracts are unchanged. All implementation, audit, documentation, and accounting chunks remain intentionally incomplete after these claims land; Chunk 30 owns final split verification and the tracker-only commit that marks Chunks 26, 26A, 27, 27A, 28, and 29 `[x]` together.
- [x] Perform a sanitation review of every retained probe artifact before writing durable evidence. `docs/x-search.md` retains only the allowed structural evidence: endpoint, tested model, request tool shape/discriminator and actually exercised options, HTTP status, top-level object/status, output discriminator/status sets, and source URLs. `agent://search-evidence-1` remains non-publishable internal orchestration evidence because it retains both exact prompt inputs; none of those prompts was copied, quoted, paraphrased, or used to claim non-retention. The repository record contains no credentials, Authorization values, prompt text, generated prose, full bodies, headers, IDs, or raw payloads. Future reruns require explicit owner authorization and append a dated structural record rather than overwrite history.

**Documentation-time verification commands:**

```sh
env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go test ./examples/...
env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK ./scripts/verify-local-consumer.sh
```

Documentation commit `311adef73795fefd2a793ebdd133d00fe140ee63` passed both commands above exactly as written with all four credential/cost-acknowledgement variables unset. `git diff --check` also passed. These are documentation-time checks only and do not constitute a paid live run or broaden the exact request-only support boundary.

**Review/accounting gate:**
- [x] `agent://chunk28-review-1` found that README applied the Responses-only `RawResponseEvent` fallback to the adjacent Chat server-search surface. Commit `73cfd2ddece333f091b280d7996f3e6a7024d9c7` resolved it by separating the streaming contracts: Responses streams typed text deltas and preserves unsupported events as `RawResponseEvent`, while Chat returns Chat completion chunks with their own `RawJSON()` preservation and never uses `RawResponseEvent`.
- [x] `agent://chunk28-review-3` found that the tracker did not record the exact Git subject for `311adef73795fefd2a793ebdd133d00fe140ee63`. The Chunk 28 accounting now records its exact subject, `docs: describe responses search requests`, and exact four-path documentation boundary.
- [x] Review-fix commit `73cfd2ddece333f091b280d7996f3e6a7024d9c7` has exact subject `docs: correct search stream boundaries` and changed exactly `README.md` and `planning/IMPLEMENTATION_PLAN.md`.
- [x] Post-fix `git diff --check` passed.
- [x] Post-fix `env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go test ./examples/...` passed.
- [x] Post-fix `env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK ./scripts/verify-local-consumer.sh` passed.

- [x] `agent://chunk28-rereview-1` re-audited the complete Chunk 28 documentation changes through `e31153ffcf652854295d0b2d9441afbd7cb60a31` and returned **CLEAN**. It confirmed the documented wrappers, methods, and types match the exported Responses built-in-tool APIs; fixed `web_search` serializes `search_context_size:"low"`; fieldless `x_search` serializes correctly; function tools precede built-ins; and the documentation consistently separates request serialization from typed output/event support and the narrow buffered hosted probes. It found no actionable technical inaccuracy.
- [x] `agent://chunk28-final-review-1` re-audited the public documentation and implementation through `4c4eef8c778ce4ee855acab7a18872c5bb080adc` and returned **CLEAN**. It confirmed the exact request-only wire shapes, wrapper and method names, ordering, raw buffered and streaming boundaries, Chat/Responses distinction, model-specific evidence limits, remaining blockers, sanitation, and changelog accounting, with no actionable technical, security, sanitation, or scope defect.
- [x] `agent://chunk28-final-review-2` correctly identified that prior closure commit `4c4eef8c778ce4ee855acab7a18872c5bb080adc` (`docs: close search documentation review`) and its sole changed path, `planning/IMPLEMENTATION_PLAN.md`, were missing from the predecessor chain; they are now recorded exactly above. Its further demand that the correcting tracker-only commit contain its own final hash is **DISCARDED** because Git commit hashes cover commit content, so inserting that hash changes the hash and creates an impossible cryptographic self-reference. The acyclic accounting rule preserves the valid audit requirement: this commit records its exact intended subject and sole tracker path, and Chunk 29 must verify and record the landed full hash, subject, and path before starting.
- [x] `agent://chunk28-release-review-1` found that the handoff named an intended subject that did not match landed predecessor `9be6f1c6f30c6433f15601fa796ef88f123b1521`. The predecessor chain now records its exact landed subject, `docs: finalize search documentation accounting`, and sole path, `planning/IMPLEMENTATION_PLAN.md`; every Chunk 29 prerequisite now points to the new finite-handoff subject instead. Its protocol review was otherwise **CLEAN**: the chain through `4c4eef8c778ce4ee855acab7a18872c5bb080adc`, subject/path inventory, self-reference disposition, and intentionally incomplete Chunk 28 state were coherent.
- [x] `agent://chunk28-release-review-2` independently found the same landed-subject mismatch, now fixed by the exact predecessor record and corrected Chunk 29 prerequisite. Its public-documentation review was otherwise **CLEAN**: exported names and methods, exact request wire shapes and ordering, request-only/raw buffered and streaming boundaries, Responses/Chat distinction, model-specific evidence limits, direct-xAI separation, sanitation exclusions, unchanged legacy contracts, earlier review fixes, and the absence of Chunk 29-or-later implementation claims were all accurate.
- [x] Local review and accounting are satisfied: every landed predecessor hash, exact subject, and path boundary through `9be6f1c6f30c6433f15601fa796ef88f123b1521`, verification result, prior finding and fix, the final CLEAN technical review, and both fixed release-review findings with their otherwise CLEAN conclusions are durably recorded. Under the finite self-hash rule, the tracker-only handoff recorded the intended subject `docs: close search documentation handoff` and sole path `planning/IMPLEMENTATION_PLAN.md`, but not its own hash; Chunk 29 verified the landed commit as `b274715a2cd4c5fcc3ada4d43a8e62a09ee4e980`. Chunk 30 has now closed Chunk 28 `[x]`.
- [x] Rollback ownership is satisfied. Split documentation claims by the surviving surface inside the atomic groups defined in Chunk 30: removing `x_search` retains truthful `web_search` documentation, while removing `web_search` restores the pre-continuation unsupported status. Documentation must remain committed in the same state as its corresponding API and audit accounting.

### Chunk 29 — Package, live-evidence, release, and maintainer accounting

**Current status:** `[x]` package-scope, live-evidence, release, and maintainer reconciliation; the complete landed chain through `ef6b9ed5969fef89d926a1c86e638045a4ba6760`; tracker-only handoff `49e9c269996aa240f0886ae052676128218aa48c`; both exact credential-scrubbed documentation-time gates; both fresh independent CLEAN rereviews; rollback ownership; and the encompassing Chunk 30 closure are complete. No actionable Chunk 29 finding remains.

**Depends on:** Chunk 28's public boundary, sanitation work, verification, hashes, exact changed paths, and clean local rereview were durably recorded before Chunk 29 began. The immediate landed predecessor was verified from Git as `b274715a2cd4c5fcc3ada4d43a8e62a09ee4e980` with exact subject `docs: close search documentation handoff` and sole changed path `planning/IMPLEMENTATION_PLAN.md`; all predecessor chunks are now closed `[x]`.

**Owned paths (exact five):** `doc.go`, `docs/evaluation-live-evidence.md`, `docs/releasing.md`, `AGENTS.md`, `planning/IMPLEMENTATION_PLAN.md`.

**Landed documentation commit boundary:** `docs: update responses search status`. **Serialized accounting commit boundary:** `docs: record search support reconciliation`.

**Complete landed predecessor/documentation/review-fix/accounting commit path chain:** `b274715a2cd4c5fcc3ada4d43a8e62a09ee4e980` (`docs: close search documentation handoff`) changed only `planning/IMPLEMENTATION_PLAN.md`; `faea1a25fe9319ec02432556fc22be23c21b5101` (`docs: update responses search status`) changed exactly `doc.go`, `docs/evaluation-live-evidence.md`, `docs/releasing.md`, and `AGENTS.md`; `4d9c5e03819dc1b7e0c3f47a3118052e413a0568` (`docs: record search status verification`) changed only `planning/IMPLEMENTATION_PLAN.md`; `2a3c263d15cce6a9efde600e84fd8527c51c9626` (`docs: reconcile search support status`) changed only `planning/IMPLEMENTATION_PLAN.md`; `ef6b9ed5969fef89d926a1c86e638045a4ba6760` (`docs: record search status fix`) changed only `planning/IMPLEMENTATION_PLAN.md`.

- [x] Narrow every current blanket unsupported-search statement in `doc.go`, `docs/evaluation-live-evidence.md`, and `docs/releasing.md` to request-only support for exact `{"type":"web_search","search_context_size":"low"}` and fieldless `{"type":"x_search"}` declarations. Preserve explicit blockers for configurable `x_search` options, `web_search_preview` and all other undocumented web-search forms/options, typed buffered outputs, typed stream events/lifecycle, citations/annotations, wider model compatibility, and direct-xAI support.
- [x] Keep live-evidence accounting truthful: the dated structural probes corroborate only the two exact request declarations and observed raw discriminators; they do not satisfy typed output/event contracts, configurable-option evidence, optional Chat live corroboration, provider-evaluation live evidence, or any other owner-authorized paid-live gate. Preserve the sanitation exception for the non-publishable internal artifact without reproducing its prompts.
- [x] Keep release accounting truthful: request-only support does not clear Chunk 24 or release gates for protected hosted CI/environment evidence, owner-authorized paid generation and provider-evaluation contracts, owner-selected license, hosted metadata/provenance/permissions/publication review, hosting authorization, immutable tagging, direct-VCS/public-proxy/checksum evidence, or final promotion/publication. Do not relabel those gates complete or make request support a release substitute.
- [x] Add an `AGENTS.md` lesson requiring field-by-field Gateway evidence, fixed types for single proven values, separate direct-provider provenance from Gateway acceptance evidence, raw preservation rather than inferred typed output, and no credential, prompt, generated-prose, or raw-payload retention in repository evidence.
- [x] Reconcile package-scope, live-evidence, release, and maintainer wording against Chunk 28 so no current repository statement still says the two request declarations are wholly unsupported and no statement broadens support beyond their exact wire shapes.

**Documentation-time verification commands:**

```sh
env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK go doc -all .
env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK git diff --check
```

- [x] After commit `2a3c263d15cce6a9efde600e84fd8527c51c9626`, the exact credential-scrubbed `go doc -all .` command passed.
- [x] After commit `2a3c263d15cce6a9efde600e84fd8527c51c9626`, the exact credential-scrubbed `git diff --check` command passed.

**Completed gate — independent documentation/security/accounting rereview and handoff:**

- [x] `agent://chunk29-review-3` found that retained Chunk 24-era current-status passages still used present-tense blanket blockers for Responses built-in search and Gateway-native `x_search`, contradicting the later implemented request-only boundary. Commit `2a3c263d15cce6a9efde600e84fd8527c51c9626` (`docs: reconcile search support status`), whose sole changed path was `planning/IMPLEMENTATION_PLAN.md`, resolved the stale-status finding by preserving the earlier evidence as explicitly historical/superseded and updating active/current-status rules to the exact implemented scope: fixed low-context public Responses `{"type":"web_search","search_context_size":"low"}` and fieldless Gateway `{"type":"x_search"}` are supported through the opt-in wrapper; configurable options, `web_search_preview` and other forms, typed outputs/events, wider model compatibility, direct-xAI, and unrelated surfaces remain blocked. The exact credential-scrubbed `go doc -all .` and `git diff --check` commands both passed after the fix.
- [x] `agent://chunk29-rereview-1` re-audited `doc.go`, `docs/evaluation-live-evidence.md`, `docs/releasing.md`, `AGENTS.md`, and the active/current tracker passages through `ef6b9ed5969fef89d926a1c86e638045a4ba6760` and returned **CLEAN**. It confirmed the exact request-only boundary, concrete unsupported-search and release blockers, separation of the narrow structural probes from the protected live contracts, sanitation of durable evidence, explicit exclusion of the non-publishable prompt-bearing artifact, and exact commit/subject/path accounting, with no actionable defect.
- [x] `agent://chunk29-rereview-2` independently re-audited the complete predecessor/documentation/accounting chain through `ef6b9ed5969fef89d926a1c86e638045a4ba6760` and returned **CLEAN**. It confirmed every exact hash, subject, and path boundary; the then-correct intentionally incomplete statuses; the absence of later-scope or release-gate overclaiming; and the finite handoff protocol, with no actionable documentation, security, accounting, or patch defect. Chunk 30 has now closed Chunk 29 `[x]`.
- [x] Local review and accounting are satisfied: the tracker preserves the complete earlier chain and findings, both exact credential-scrubbed verification results, the stale-status fix, the predecessor accounting commit `ef6b9ed5969fef89d926a1c86e638045a4ba6760` with exact subject `docs: record search status fix` and sole path `planning/IMPLEMENTATION_PLAN.md`, and both CLEAN rereviews. Under the finite self-hash rule, the tracker-only handoff recorded the intended subject `docs: record search support reconciliation` and sole path `planning/IMPLEMENTATION_PLAN.md`, but not its own hash; Chunk 30 verified the landed commit as `49e9c269996aa240f0886ae052676128218aa48c` and has now closed Chunk 29 `[x]`.
- [x] Rollback ownership is satisfied: Chunk 29's surface-specific package/status claims travel with the corresponding atomic `web_search` or `x_search` rollback group, and its shared evidence/release/accounting text must describe exactly the surfaces that remain after each committed rollback. No rollback may retain a support claim for a removed surface or erase the still-truthful blockers, sanitation record, and release gates for a surviving surface.

### Chunk 30 — Search continuation final verification and split review

**Completed prerequisite:** Chunks 26, 26A, 27, 27A, 28, and 29 landed all owned work and committed their complete predecessor hash chains, exact changed-path inventories, focused verification, and clean local rereviews to `planning/IMPLEMENTATION_PLAN.md`. Git verification established the immediate predecessor as `49e9c269996aa240f0886ae052676128218aa48c` with exact subject `docs: record search support reconciliation` and sole changed path `planning/IMPLEMENTATION_PLAN.md`.

**Owned closure path and read-only review partitions:** `planning/IMPLEMENTATION_PLAN.md` is Chunk 30's only writable path. The four completed read-only partitions were: (1) **API/encoding/validation/tests** — `responses.go`, `responses_tools.go`, `responses_wire.go`, `responses_validate.go`, `responses_test.go`; (2) **external contract/audit** — `contract_external_test.go`, `scripts/verify-local-consumer.sh`, `planning/IMPLEMENTATION_PLAN.md`; (3) **public docs/evidence sanitation** — `docs/generation.md`, `docs/x-search.md`, `README.md`, `CHANGELOG.md`; and (4) **package/release/status/accounting** — `doc.go`, `docs/evaluation-live-evidence.md`, `docs/releasing.md`, `AGENTS.md`. Chunk 30 changed no implementation, audit, or public-documentation path.

**Final tracker-only commit boundary under the finite self-hash rule:** this closure commit's exact intended subject is `docs: close responses search continuation` and its sole changed path is `planning/IMPLEMENTATION_PLAN.md`. It does not and cannot record its own final hash; no self-hash or follow-up commit is required.

- [x] **Initial complete gate passed with credentials and live acknowledgements unset and `GOPROXY=off`.** `AI_GATEWAY_API_KEY`, `VERCEL_OIDC_TOKEN`, `AI_GATEWAY_LIVE_COST_ACK`, and `AI_GATEWAY_PUBLIC_LIVE_COST_ACK` were unset for the complete gate. Focused ordinary and race tests covering both `ResponseWebSearchTool` and `ResponseXSearchTool` passed; full ordinary `go test -count=1 ./...` and full race `go test -race -count=1 ./...` passed; `go vet ./...` passed; `go test ./examples/...` passed; full `go doc -all .` passed; the external `TestExternalContract` contract passed; `./scripts/verify-local-consumer.sh` passed its local consumer/export audit; workspace diagnostics reported exactly `No issues found`; and `git diff --check` passed. No paid probe was rerun.
- [x] **All four split final reviews are CLEAN.** `agent://responses-search-final-review-1` found the API/encoding/validation/tests partition CLEAN; `agent://responses-search-final-review-2` found the external contract/audit/accounting partition CLEAN; `agent://responses-search-final-review-3` found the public docs/evidence-sanitation partition CLEAN; and `agent://responses-search-final-review-4` found the package/release/status/accounting partition CLEAN. No review fix was required, so the initial complete gate is the gate for the final reviewed chain and no post-fix rerun was necessary.
- [x] **One-to-one owned-path coverage reconciliation is complete.** Chunk 26's `responses.go`, `responses_wire.go`, `responses_validate.go`, and `responses_test.go`, plus Chunk 27's `responses_tools.go` and shared implementation paths, received substantive review in partition 1. Chunks 26A/27A's `contract_external_test.go` and `scripts/verify-local-consumer.sh`, and every chunk's serialized tracker accounting in `planning/IMPLEMENTATION_PLAN.md`, received substantive review in partition 2. Chunk 28's `docs/generation.md`, `docs/x-search.md`, `README.md`, and `CHANGELOG.md` received substantive review in partition 3. Chunk 29's `doc.go`, `docs/evaluation-live-evidence.md`, `docs/releasing.md`, and `AGENTS.md` received substantive review in partition 4. Every owned substantive path appears in its declared partition; shared implementation claims were cross-checked against both documentation partitions, exports against the API and external-contract partitions, and tracker status/evidence against all reports. No path was accepted through inventory metadata alone and no owned path was omitted.
- [x] **Finding routing is closed with no findings.** The four CLEAN reports produced no implementation, external-contract/audit, documentation/sanitation, package/release/status, or Chunk-30 accounting fix to route. The established ownership rules and coordinated atomic rollback groups remain the required disposition if a later regression is discovered; no rollback is performed by this successful closure.
- [x] **Final acceptance passed.** Function-only callers remain unchanged; both exact wrapper encodings, built-in-tool methods, and fieldless tool types are present in the strict export inventory and external consumer; shared invalid configurations fail before credentials/network; no generic/configurable search escape hatch exists; raw buffered and streaming boundaries remain unchanged; all substantive paths and eight public/package/release documents received reconciled review; the repository evidence contains no credential, prompt text, generated prose, or raw payload; and all unsupported surfaces retain concrete blockers. Chunks 26, 26A, 27, 27A, 28, and 29 are closed `[x]` together.
- [x] **Rollback accounting is complete without executing a rollback.** Any future rollback must use the already-recorded coordinated atomic groups: remove the tracker-only closure metadata when appropriate, then remove the `x_search` API/audit/docs/accounting group while retaining truthful `web_search` support, and only then remove the `web_search` group and restore the pre-continuation unsupported state. No intermediate committed state may separate an API from its audit, documentation, evidence, or tracker truth.

### Remaining evidence blockers after request support

- [!] `web_search_preview` and every configurable `web_search` value/field beyond exact `search_context_size:"low"`: require an exact first-party Gateway request example/schema/source/test defining discriminator, field type, presence, enum/bounds, and compatible routing.
- [!] Search-specific tool choice, `allowed_tools`, function/built-in precedence, and fallback behavior: require exact first-party Gateway Responses semantics; generic Responses modes alone do not prove built-in-search behavior.
- [!] Typed `web_search_call` and `x_search_call` outputs, actions, posts, sources, citations/annotations/offsets, statuses, refusals, provider errors, usage/cost, and unknown variants: require a first-party Gateway schema/source/test or an owner-authorized retained sanitized fixture set establishing every field's discriminator, type, requiredness, nullability, cardinality, unknown-field policy, and raw-preservation rule. The two structural probes alone do not clear this gate.
- [!] Search lifecycle SSE typing and terminal semantics: require first-party Gateway event schemas or authorized sanitized event fixtures establishing event names, payloads, ordering, deltas, completion/failure effects, and unknown-event behavior. Until then all non-text-delta events remain `RawResponseEvent`.
- [!] Wider model/provider compatibility: require exact first-party Vercel evidence or separately authorized model-specific probes. Evidence for `openai/gpt-5.4-mini` and `spacexai/grok-4.6` must not be generalized to all OpenAI, SpaceXAI, or Gateway models.
- [!] Configurable `x_search` fields are blocked independently from the fieldless declaration. Each exact field or inseparable option family requires Gateway-specific first-party evidence or an owner-authorized exact-field Gateway probe covering presence form, accepted/rejected boundary, and relevant route. Conflicting direct-xAI versions remain candidate vocabulary only and must not be combined into local Gateway validation.
- [!] Direct xAI remains deferred, not an unchecked continuation task: a client for `https://api.x.ai/v1/responses`, direct-xAI authentication/base URLs, provider-specific option validation, provider-tool result normalization, and generic OpenAI-compatible parity require a separately approved direct-provider package/API plan and authoritative direct-xAI contracts. Gateway fieldless `x_search` support does not authorize this surface.
- [!] Unrelated live and release surfaces remain blocked, not over-marked complete: public `/v1/evaluate` still lacks an exhaustive first-party strict HTTP contract; the general paid public-generation and provider-evaluation live contracts still require owner authorization, protected credentials, exact cost acknowledgements, approved pinned models/environments, and sanitized independently reviewed evidence; and candidate/tag/publication still require owner-selected license, verified hosted repository metadata and protected CI/environment evidence, provenance/permissions/publication review, hosting authorization, immutable tag creation, direct-VCS/public-proxy fetched-module verification, checksum capture, and final promotion/publication authorization.
- [!] Every remaining search-continuation item is therefore explicitly blocked or deferred by the evidence/authorization prerequisite stated in this section. There is no locally doable unchecked continuation item after Chunk 30 closure.

## 2026-09-21 configurable Gateway `x_search` continuation — current authority

This section is the current authority for any future configurable `x_search` work. It appends to, and does not rewrite or reopen, the completed Chunk 26 → 30 history above. **Implementation has not started.** The repository still supports only fieldless `ResponseXSearchTool{}`, encoded exactly as `{"type":"x_search"}`, through the existing Gateway Responses built-in-tool wrapper. No configurable type, option encoder, option validation, configurable external-consumer contract, or configurable support claim exists yet.

### Contract target, candidate vocabulary, and non-authoritative conflicts

- [x] The target is additive, request-only configuration for public Vercel AI Gateway `POST /v1/responses`; it is not a direct `https://api.x.ai/v1/responses` client. Existing methods, wrappers, `ResponsesRequest`, raw buffered output, raw non-text streaming events, retry behavior, authentication, endpoint ownership, and fieldless `ResponseXSearchTool{}` remain unchanged.
- [x] Version-pinned `ai@7.0.107` / `@ai-sdk/xai@5.0.4` establish only this candidate direct-xAI vocabulary and mapping: `allowedXHandles` → `allowed_x_handles`, `excludedXHandles` → `excluded_x_handles`, `fromDate` → `from_date`, `toDate` → `to_date`, `enableImageUnderstanding` → `enable_image_understanding`, and `enableVideoUnderstanding` → `enable_video_understanding`. All six candidates are optional in that direct-provider helper; this is not Gateway acceptance evidence.
- [x] The pinned direct provider caps each handle list at 10 and does not encode mutual exclusion, while current direct xAI prose says up to 20 and forbids combining allowed and excluded handles. Those statements conflict and are **non-authoritative for Gateway**. Neither 10 nor 20, mutual exclusion nor coexistence, date grammar/order, empty-list behavior, null behavior, defaults, or any other invariant may be selected locally until exact Gateway evidence resolves it.
- [x] Pinned/current Gateway package and Gateway documentation do not provide a native `xSearch` helper or public `/v1/responses` configurable `x_search` schema. Their silence is not proof of rejection, but it is insufficient proof of pass-through. The prior fieldless `spacexai/grok-4.6` probe exercised none of the six option fields.
- [x] The additive option-bearing sealed Responses built-in-tool type is named exactly `ResponseXSearchOptionsTool`; Gate A determines only which exported fields and Go presence/validation semantics it contains. Preserve `ResponseXSearchTool struct{}` and its exact fieldless wire and reflection contract. Do not add fields to it, add an arbitrary map/provider-options escape hatch, change either built-in-tool method, or add fields to `ResponsesBuiltInToolsRequest` or `ResponsesRequest`.
- [x] Direct-xAI output metadata, `query/posts`, `x_search_call` details, citations, actions, lifecycle events, and provider-executed result normalization remain out of scope. `ResponseResult.RawJSON()` and `RawResponseEvent` remain the complete boundaries for untyped Gateway data.

### Gate A — exact Gateway option evidence before any implementation

**Gate status and next action:** `[x]` **EVIDENCE AND REVIEW COMPLETE; GATE A COMPLETE; CHUNK 31 NEXT.** The reviewed 14-call matrix, four-call interaction run, and focused two-call run clear only the six singleton forms, the exact neutral four-field form, and the two exact maximal five-field forms described below. The focused run recorded fieldless HTTP 200 and excluded maximal five-field HTTP 200 and passed in 132.84 seconds. Local validation must reject simultaneous non-empty allowed and excluded handle lists. Independent evidence/accounting review is clean. The tracker-only Gate A accounting commit containing this entry is the pre-implementation boundary, but this commit cannot record its own hash; Chunk 31's successor accounting must record that landed predecessor.

Each family clears independently; evidence for one family does not authorize another:

- [x] **Allowed handles — `allowed_x_handles`: singleton request acceptance cleared for the tested non-empty JSON string-array form; empty array accepted only in the exact neutral four-field combination.** Omission is accepted through the fieldless control. Null, element syntax, duplicates, list limits, semantics, arbitrary subsets, and arbitrary coexistence remain blocked.
- [x] **Excluded handles — `excluded_x_handles`: singleton request acceptance cleared for the tested non-empty JSON string-array form; empty array accepted only in the exact neutral four-field combination.** Omission is accepted through the fieldless control. The exact excluded maximal five-field form—excluded handles, both date strings, and both true understanding flags, with allowed handles omitted—is accepted. Simultaneous non-empty coexistence with `allowed_x_handles` is a reviewed safe rejection boundary and must be rejected locally. Null, element syntax, duplicates, list limits, semantics, arbitrary subsets, and arbitrary coexistence remain blocked.
- [x] **Dates — `from_date` and `to_date`: each tested singleton JSON string form is request-accepted independently.** Their coexistence is accepted only in each exact maximal five-field form. Date grammar, empty/null forms, ordering, inclusivity, range semantics, defaults, arbitrary subsets, and other combined behavior remain blocked.
- [x] **Image understanding — `enable_image_understanding`: singleton JSON true is request-accepted; explicit false is accepted only in the exact neutral four-field combination.** True coexistence is accepted only in each exact maximal five-field form. Semantics, null, default, arbitrary subsets, and other non-neutral coexistence remain blocked.
- [x] **Video understanding — `enable_video_understanding`: singleton JSON true is request-accepted; explicit false is accepted only in the exact neutral four-field combination.** True coexistence is accepted only in each exact maximal five-field form. Semantics, null, default, arbitrary subsets, and other non-neutral coexistence remain blocked.
- [x] **Cross-family validation and presence semantics: cleared only for exact proven forms.** The exact neutral four-field form contains both empty handle lists and both explicit false booleans. The allowed and excluded maximal five-field forms each contain exactly one non-empty handle list, both date strings, and both true booleans. Omission is accepted. Simultaneous non-empty handle lists must reject locally before network. Handle lists use typed Go `[]string`, dates use strings, and booleans require explicit presence support. Wrong JSON kinds are unrepresentable through the typed API; the matrix's HTTP 500 cases do not establish Gateway wrong-kind validation. Every arbitrary or untested subset remains unproven without inventing a local rejection.

**Implemented matrix and interaction evidence harnesses:**

- [x] The dedicated Gate A matrix harness is implemented and its third authorized run completed against only public `POST https://ai-gateway.vercel.sh/v1/responses` with `spacexai/grok-4.6`. It used the existing fail-closed prerequisites, runtime-only inputs, structural sanitation, strict 90-second request contexts derived from one 22-minute overall deadline, no retries/fallback, and a 14-call serial ceiling.
- [x] Owner authorization and bounded-cost acceptance for the completed matrix are recorded without any credential or private input value. The retained artifact contains only safe structural facts: 10 HTTP 200 completed responses, one ambiguous canonical-all-six HTTP 400, three inconclusive wrong-kind HTTP 500 responses, and the aggregate test failure on the wrong-kind expectations.
- [x] Independent adjudication records that the six singleton fields and exact neutral combination are accepted request forms only. The canonical all-six failure clears or rejects no individual field, and the wrong-kind 500s add no rejected boundary. Observed structural output types do not establish semantic efficacy or complete/typed output and event contracts.
- [x] The reviewed four-call interaction run used exact selector `^TestGatewayXSearchOptionsInteractionContract$` with the same fail-closed prerequisites, runtime-only private inputs, sanitation, endpoint/model, 90-second per-request contexts, 22-minute overall bound, and no retry/fallback. It recorded fieldless HTTP 200, allowed maximal five-field HTTP 200, handle-pair-only HTTP 400 satisfying the safe rejection rule, and excluded maximal execution failure/inconclusive. The test emitted its final failure after all fixed cases were attempted.
- [x] The focused follow-up ran under exact selector `^TestGatewayXSearchOptionsExcludedInteractionContract$` with a hard ceiling of two serial calls: fieldless control, then excluded handles plus both dates and both true booleans with allowed handles omitted. Both returned HTTP 200 completed responses, and the test passed in 132.84 seconds. It reused the existing prerequisites, runtime inputs, sanitation, 90-second request and 22-minute overall bounds, and no retry/fallback. The required fieldless control was repeated, while neither completed selector/full matrix was rerun. All positive responses in the complete chain structurally contained `x_search_call`; this establishes neither semantic efficacy nor typed output/event contracts.
- [x] Durable output remains structurally allowlisted to case label, option class set/count, HTTP status, safe top-level object/status, safe output discriminator/status set, `error_present`, and sanitized error category/code classes. The third-run artifact and adjudication retain no credentials, acknowledgement/header values, prompts, queries, private handles/dates, bodies, headers, IDs, generated prose, tool arguments/results, provider metadata, usage, raw events, or raw errors. Safe accounting is limited to 14 completed calls, 10 HTTP 200 responses, one HTTP 400, three HTTP 500 responses, 564.18 seconds wall-clock, and the exact structural classifications recorded above.
- [!] The general `public-generation` and `provider-evaluation` live jobs retain their separate status and acknowledgements. This dedicated gate neither runs in nor clears either job, and those jobs cannot substitute for configurable-option evidence.

**Gate A acceptance and accounting:**

- [x] Gate A evidence is complete only for the six singleton forms, exact neutral four-field form, both exact maximal five-field forms, omission, and the required local simultaneous-nonempty-handle-list rejection. Still blocked: list limits, handle syntax, duplicates, null, date grammar/order/inclusivity, empty date strings, semantic effects, arbitrary/untested subsets, other models, fallback, direct-xAI behavior, typed outputs/events, and Gateway wrong-kind error behavior.
- [x] **Complete probe/evidence chain and clean review accounting:**
  - `17a42e85c8690dc01d16a5e17f403af718356e76` — `test: add x search option live probe` — changed paths `.github/workflows/live-contract.yml`, `docs/evaluation-live-evidence.md`, `docs/releasing.md`, `internal/livecontract/x_search_options_test.go`, and `planning/IMPLEMENTATION_PLAN.md`.
  - `9668edbde4a0411f1de820f1e39eb5cb34a5dbb2` — `fix: harden x search option probe` — changed paths `.github/workflows/live-contract.yml`, `docs/evaluation-live-evidence.md`, `docs/releasing.md`, `internal/livecontract/x_search_options_test.go`, and `planning/IMPLEMENTATION_PLAN.md`.
  - `9c6a0afb0f07ba19b9c6fe424f4882803968364b` — `fix: bound x search option probes` — changed paths `.github/workflows/live-contract.yml`, `docs/evaluation-live-evidence.md`, `docs/releasing.md`, `internal/livecontract/x_search_options_test.go`, and `planning/IMPLEMENTATION_PLAN.md`.
  - `5b597e73bafef9484f68624af7b763fb289239f5` — `fix: classify x search probe failures` — changed paths `docs/evaluation-live-evidence.md`, `internal/livecontract/x_search_options_test.go`, and `planning/IMPLEMENTATION_PLAN.md`.
  - `e33b4737e57b7cec53269d49c854e745829c4409` — `fix: extend x search probe timeout` — changed paths `.github/workflows/live-contract.yml`, `docs/evaluation-live-evidence.md`, `docs/releasing.md`, `internal/livecontract/x_search_options_test.go`, and `planning/IMPLEMENTATION_PLAN.md`.
  - `5b58fa0f639c10c4ac7a6043658473874f411669` — `test: add x search interaction probe` — changed paths `.github/workflows/live-contract.yml`, `docs/evaluation-live-evidence.md`, `docs/releasing.md`, `internal/livecontract/x_search_options_test.go`, and `planning/IMPLEMENTATION_PLAN.md`.
  - `38b53b235bbe7af15b13978176fde395c502cff2` — `fix: complete x search interaction evidence` — changed paths `docs/evaluation-live-evidence.md`, `docs/releasing.md`, `internal/livecontract/x_search_options_test.go`, and `planning/IMPLEMENTATION_PLAN.md`.
  - `833e5f29c57dccd1dc012ae34c99fe54a0a80fce` — `test: add excluded x search interaction probe` — changed paths `.github/workflows/live-contract.yml`, `docs/evaluation-live-evidence.md`, `docs/releasing.md`, `internal/livecontract/x_search_options_test.go`, and `planning/IMPLEMENTATION_PLAN.md`.
  - `2274d359e8360dbb89a9ce26d7117288f9cb4af5` — `docs: record x search option live evidence` — changed paths `docs/evaluation-live-evidence.md` and `docs/releasing.md`.
- [x] Independent evidence and accounting review found no unresolved Gate A item. This tracker-only accounting commit has intended subject `docs: record gateway x search option evidence` and sole path `planning/IMPLEMENTATION_PLAN.md`. It deliberately records every predecessor through `2274d359e8360dbb89a9ce26d7117288f9cb4af5`, not its impossible self-hash. Once landed, this accounting commit itself is the pre-implementation base; Chunk 31's successor accounting records its full hash. Gate A is complete and Chunk 31 is dependency-ready.

### Chunk 31 — Additive option-bearing request API and exact wire behavior

**Status:** `[-]` Implementation, focused ordinary/race/full verification, fixes, and three clean rereviews are complete. Chunk 31 becomes `[x]` complete only when the tracker-only accounting commit described below lands. **Commit boundary:** `feat: add gateway x search options`.

**Owned paths (exact five):** `responses_tools.go`, `responses_wire.go`, `responses_validate.go`, `responses_test.go`, `planning/IMPLEMENTATION_PLAN.md`.

- [x] Added `ResponseXSearchOptionsTool` alongside—not instead of—`ResponseXSearchTool{}` with exactly six exported fields: `AllowedXHandles []string`, `ExcludedXHandles []string`, `FromDate *string`, `ToDate *string`, `EnableImageUnderstanding *bool`, and `EnableVideoUnderstanding *bool`. Both value and non-nil pointer forms satisfy the sealed `ResponseBuiltInTool` union; nil and typed-nil tools reject before credentials or network.
- [x] Exact wire behavior is request-only `{"type":"x_search"}` plus only present proven option keys: nil slices/pointers omit keys, non-nil empty slices encode as `[]`, explicit false encodes as `false`, caller order and duplicates are preserved, and no option encodes as `null`. Function-tools-first ordering, caller order among built-ins, `stream` ownership, both public methods, fieldless `x_search`, `web_search`, and raw buffered/event contracts remain unchanged.
- [x] Validation retains the existing generic collection/string/request bounds and adds only the proven canonical-path rejection for simultaneous non-empty `allowed_x_handles` and `excluded_x_handles`. Empty or otherwise unproven date values are not rejected by invented syntax, order, or inclusivity rules; no direct-provider 10/20 limit, handle syntax, duplicate rule, default, wrong-kind server claim, arbitrary-subset claim, direct-xAI behavior, or typed x-search output/event API was added.
- [x] Gate A accounting predecessor `d0dfb25326cbf58775c9c109420920e7c9148e08` (`docs: record gateway x search option evidence`) changed only `planning/IMPLEMENTATION_PLAN.md`. Green draft `dda9dfe055cc1a590afc2f3de49409472f3752a0` (`feat: add gateway x search options`) changed exactly `responses_test.go`, `responses_tools.go`, `responses_validate.go`, and `responses_wire.go`; focused ordinary, focused race, and full ordinary tests passed. Independent review found an invented empty-date rejection and missing independent singleton/public-method coverage. Fix commit `e3697d84252ac1d59fd65e19d95b5b52252bb38b` (`fix: correct gateway x search option contract`) changed exactly `responses_test.go` and `responses_validate.go`; focused ordinary, focused race, full ordinary, and full race tests passed after the fix, followed by three clean independent rereviews. No Go LSP server was available, so no Go workspace diagnostics were run.
- [x] **Exact verification accounting:** Before fix `e3697d84252ac1d59fd65e19d95b5b52252bb38b`, the following pre-fix commands passed:
```sh
env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK -u AI_GATEWAY_X_SEARCH_LIVE_COST_ACK go test -count=1 -run 'Test(CreateResponse|StreamResponse|ResponsesBuiltIn|XSearch)' ./...
env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK -u AI_GATEWAY_X_SEARCH_LIVE_COST_ACK go test -race -count=1 -run 'Test(CreateResponse|StreamResponse|ResponsesBuiltIn|XSearch)' ./...
env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK -u AI_GATEWAY_X_SEARCH_LIVE_COST_ACK go test -count=1 ./...
```
No pre-fix full-race command was run. After fix `e3697d84252ac1d59fd65e19d95b5b52252bb38b`, the following post-fix commands passed:
```sh
env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK -u AI_GATEWAY_X_SEARCH_LIVE_COST_ACK go test -count=1 -run 'Test(CreateResponse|StreamResponse|ResponsesBuiltIn|XSearch)' ./...
env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK -u AI_GATEWAY_X_SEARCH_LIVE_COST_ACK go test -race -count=1 -run 'Test(CreateResponse|StreamResponse|ResponsesBuiltIn|XSearch)' ./...
env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK -u AI_GATEWAY_X_SEARCH_LIVE_COST_ACK go test -count=1 ./...
env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK -u AI_GATEWAY_X_SEARCH_LIVE_COST_ACK go test -race -count=1 ./...
```
No Go LSP server was available, so no Go workspace diagnostics were run.
- [x] The tracker-only accounting handoff has intended subject `docs: record gateway x search option implementation` and sole path `planning/IMPLEMENTATION_PLAN.md`. It records every predecessor through `e3697d84252ac1d59fd65e19d95b5b52252bb38b`, not its impossible self-hash. Once it lands, Chunk 31 is complete and Chunk 31A is dependency-ready; Chunk 31A's successor accounting must record this handoff's full hash. Rollback ownership remains one coordinated configurable-options rollback: remove this option type, encoder/validator branches, focused tests, later audit/docs/status claims, and their accounting while preserving fieldless `ResponseXSearchTool{}` and truthful append-only history.

**Chunk 31 acceptance commands (credentials and all live acknowledgements absent):**

```sh
env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK -u AI_GATEWAY_X_SEARCH_LIVE_COST_ACK go test -count=1 -run 'Test(CreateResponse|StreamResponse|ResponsesBuiltIn|XSearch)' ./...
env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK -u AI_GATEWAY_X_SEARCH_LIVE_COST_ACK go test -race -count=1 -run 'Test(CreateResponse|StreamResponse|ResponsesBuiltIn|XSearch)' ./...
```

### Chunk 31A — External consumer contract and strict export audit

**Status:** `[-]` External contract/export implementation, the corrective fix, both post-fix acceptance commands, and a clean independent rereview are complete. Chunk 31A becomes `[x]` complete only when the tracker-only accounting commit described below lands; that landing makes Chunk 32 dependency-ready. **Commit boundary:** `test: audit gateway x search options`.

**Owned paths (exact three):** `contract_external_test.go`, `scripts/verify-local-consumer.sh`, `planning/IMPLEMENTATION_PLAN.md`.

- [x] The external contract compile- and reflection-locks the exact exported type `ResponseXSearchOptionsTool` with ordered fields `AllowedXHandles []string`, `ExcludedXHandles []string`, `FromDate *string`, `ToDate *string`, `EnableImageUnderstanding *bool`, and `EnableVideoUnderstanding *bool`. Both value and non-nil-pointer forms satisfy the sealed `ResponseBuiltInTool` interface whose sole unexported method remains `responseBuiltInTool()`. The contract covers omitted options, present-empty slices, explicit false booleans, non-empty dates, present-empty date strings, maximal allowed/excluded forms, and both `CreateResponseWithBuiltInTools` and `StreamResponseWithBuiltInTools` consumer boundaries while preserving fieldless `ResponseXSearchTool{}`, wrapper shape, legacy `ResponsesRequest`, public method signatures, and raw result/event contracts.
- [x] The strict bidirectional export inventory admits only `ResponseXSearchOptionsTool` as the new export and locks its exact six-field order/types. It continues to reject alternate or compatibility names, aliases, generic option maps, provider-option escape hatches, direct-xAI clients/auth/base URLs, typed x-search output/event types, accidental method changes, and every uncleared candidate field. The temporary external module uses a local `replace`, `GOWORK=off`, and `GOPROXY=off`; its generated consumer is compiled but not executed.
- [x] Chunk 31 accounting predecessor `f9b5b296f305e2c368f781b573e307228013e61c` (`docs: record gateway x search option implementation`) changed only `planning/IMPLEMENTATION_PLAN.md`. Green draft `94e9efa1c4f5f44a983da5efdce43e9a75adc70d` (`test: audit gateway x search options`) changed exactly `contract_external_test.go` and `scripts/verify-local-consumer.sh`; both acceptance commands below passed. Independent review found that the externally constructible present-empty `FromDate`/`ToDate` form was missing, and a separate audit caught incomplete environment scrubbing. Fix `8180a6471adbfe54b50ab67ace5f4bb411cf7cf1` (`fix: complete x search external contract`) changed exactly the same two paths, added the present-empty date form to both external consumers through both public methods, and centralized scrubbing of all ten variables: `AI_GATEWAY_API_KEY`, `VERCEL_OIDC_TOKEN`, `AI_GATEWAY_LIVE_COST_ACK`, `AI_GATEWAY_PUBLIC_LIVE_COST_ACK`, `AI_GATEWAY_X_SEARCH_LIVE_COST_ACK`, `AI_GATEWAY_X_SEARCH_PROBE_INPUT`, `AI_GATEWAY_X_SEARCH_PROBE_HANDLE_A`, `AI_GATEWAY_X_SEARCH_PROBE_HANDLE_B`, `AI_GATEWAY_X_SEARCH_PROBE_FROM_DATE`, and `AI_GATEWAY_X_SEARCH_PROBE_TO_DATE`. After the fix, the exact external-contract test and offline local-consumer commands below passed, and a fresh independent rereview was clean. No Go LSP claim is made.
- [x] The tracker-only accounting handoff has intended subject `docs: record gateway x search option contract` and sole path `planning/IMPLEMENTATION_PLAN.md`. It records every predecessor through `8180a6471adbfe54b50ab67ace5f4bb411cf7cf1`, not its impossible self-hash. Once it lands, Chunk 31A is complete and Chunk 32 is dependency-ready; Chunk 32's successor accounting must record this handoff's full hash. Rollback ownership remains the one coordinated configurable-options rollback: remove the option type, encoder/validator branches and focused tests, these external contract/export entries, later option-specific docs/status claims, and their accounting while preserving fieldless `ResponseXSearchTool{}`, its wrapper/public methods, raw outputs/events, and truthful append-only history.

**Chunk 31A acceptance commands:**

```sh
env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK -u AI_GATEWAY_X_SEARCH_LIVE_COST_ACK go test -count=1 -run '^TestExternalContract' ./...
env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK -u AI_GATEWAY_X_SEARCH_LIVE_COST_ACK GOPROXY=off ./scripts/verify-local-consumer.sh
```

### Chunk 32 — Public documentation, migration note, and durable evidence

**Status:** `[!]` Blocked until Chunk 31A's tracker-only accounting handoff lands; that landing makes this chunk dependency-ready. **Commit boundary:** `docs: describe gateway x search options`.

**Owned paths (exact five):** `docs/x-search.md`, `docs/generation.md`, `README.md`, `CHANGELOG.md`, `planning/IMPLEMENTATION_PLAN.md`.

- [!] Document only Gate-A-cleared fields and exact presence/validation semantics, the exact evidenced Gateway route(s), request-only scope, no universal model/fallback promise, unchanged fieldless form, and unchanged raw output/event boundaries. List every uncleared candidate field/family as blocked rather than implying all-or-nothing support.
- [!] Add a safe configurable example that prints no handles, prompts, raw bodies/events, generated prose, citations, IDs, or provider metadata. Explain that existing callers require no migration because `ResponseXSearchTool{}`, wrappers, and methods are preserved; configuration is opt-in through the new sealed type.
- [!] Reconcile the changelog/support matrix without erasing the historical fieldless record or claiming direct-xAI support. Cite the pinned direct sources only as vocabulary provenance and the Gate A Gateway evidence as acceptance authority.
- [!] Record sanitized evidence exactly within the Gate A retention policy. A paid probe does not clear general public-generation/provider-evaluation live gates or typed-output/event gates.
- [!] Review loop: independently review API names/examples, evidence provenance, sanitation/privacy/cost wording, migration accuracy, and remaining blockers; fix findings; rerun both commands; obtain a clean rereview. After closure, land a tracker-only accounting handoff commit with intended subject `docs: account gateway x search option docs` and sole path `planning/IMPLEMENTATION_PLAN.md`; record the Chunk 31A accounting hash and every Chunk 32 implementation/fix hash, subject, changed path, result, and review outcome without claiming the handoff's own hash. Chunk 33 may begin only after this handoff lands.

**Chunk 32 acceptance commands:**

```sh
env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK -u AI_GATEWAY_X_SEARCH_LIVE_COST_ACK go test -count=1 ./examples/...
env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK -u AI_GATEWAY_X_SEARCH_LIVE_COST_ACK GOPROXY=off ./scripts/verify-local-consumer.sh
```

### Chunk 33 — Package/release/maintainer reconciliation

**Status:** `[!]` Blocked on Chunk 32's landed tracker-only accounting handoff. **Commit boundary:** `docs: reconcile gateway x search option status`.

**Owned paths (exact five):** `doc.go`, `docs/evaluation-live-evidence.md`, `docs/releasing.md`, `AGENTS.md`, `planning/IMPLEMENTATION_PLAN.md`.

- [!] Update package, live-evidence, release, and maintainer statements to the exact cleared option subset while preserving fieldless support, raw boundaries, model-specific evidence limits, direct-xAI separation, unrelated live/release blockers, and immutable-tag/bad-release policy.
- [!] Update and consolidate the existing adjacent `x_search` lessons in `AGENTS.md` with the exact Gate-A-cleared option subset: keep direct-provider schemas as candidate vocabulary separate from exact public Gateway acceptance evidence; use the exact exported name `ResponseXSearchOptionsTool`; preserve fieldless `ResponseXSearchTool{}` and raw output/event boundaries; and do not add a duplicate lesson or duplicate plan.
- [!] Review loop: independently audit documentation/source consistency, evidence sanitation, release implications, rollback wording, and acyclic commit accounting; fix and rerun the commands; obtain a clean rereview. After closure, land a tracker-only accounting handoff commit with intended subject `docs: account gateway x search option status` and sole path `planning/IMPLEMENTATION_PLAN.md`; record the Chunk 32 accounting hash and every Chunk 33 implementation/fix hash, subject, changed path, result, and review outcome without claiming the handoff's own hash. Chunk 34 may begin only after this handoff lands.

**Chunk 33 acceptance commands:**

```sh
env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK -u AI_GATEWAY_X_SEARCH_LIVE_COST_ACK go doc -all .
git diff --check
```

### Chunk 34 — Final verification, split review, and exact completion accounting

**Status:** `[!]` Blocked on the landed tracker-only accounting handoffs for Chunks 31, 31A, 32, and 33, with complete acyclic predecessor accounting and clean local rereviews. **Final tracker-only commit boundary:** `docs: close configurable x search continuation`; sole changed path `planning/IMPLEMENTATION_PLAN.md`; never attempt to embed its own hash.

- [!] Run the complete credential-scrubbed gate from a clean committed checkout with `GOPROXY=off`: focused ordinary/race tests, full ordinary/race tests, vet, examples, documentation, external contract, local consumer/export audit, workspace diagnostics if available, and diff check. This is hermetic proof only and must not be described as a paid live run.
- [!] Perform four independent read-only split reviews: (1) API/encoding/validation/tests; (2) external contract/export audit/backward compatibility; (3) public docs/evidence sanitation/privacy/cost; and (4) package/release/maintainer/accounting/rollback. Route every finding to its owning chunk, commit the fix in that chunk's path boundary, rerun that chunk's gates and then the complete gate, and repeat the affected review until clean.
- [!] Reconcile one-to-one coverage of every changed path, every candidate field disposition (cleared or still blocked), every review finding/fix, and every predecessor full hash/subject/path. Only after all four partitions are clean may this tracker mark the implementation chunks `[x]` together.

**Chunk 34 complete verification commands:**

```sh
env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK -u AI_GATEWAY_X_SEARCH_LIVE_COST_ACK GOPROXY=off go test -count=1 -run 'Test(CreateResponse|StreamResponse|ResponsesBuiltIn|XSearch)' ./...
env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK -u AI_GATEWAY_X_SEARCH_LIVE_COST_ACK GOPROXY=off go test -race -count=1 -run 'Test(CreateResponse|StreamResponse|ResponsesBuiltIn|XSearch)' ./...
env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK -u AI_GATEWAY_X_SEARCH_LIVE_COST_ACK GOPROXY=off go test -count=1 ./...
env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK -u AI_GATEWAY_X_SEARCH_LIVE_COST_ACK GOPROXY=off go test -race -count=1 ./...
env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK -u AI_GATEWAY_X_SEARCH_LIVE_COST_ACK GOPROXY=off go vet ./...
env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK -u AI_GATEWAY_X_SEARCH_LIVE_COST_ACK GOPROXY=off go test -count=1 ./examples/...
env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK -u AI_GATEWAY_X_SEARCH_LIVE_COST_ACK GOPROXY=off go doc -all .
env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK -u AI_GATEWAY_X_SEARCH_LIVE_COST_ACK GOPROXY=off go test -count=1 -run '^TestExternalContract' ./...
env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK -u AI_GATEWAY_X_SEARCH_LIVE_COST_ACK GOPROXY=off ./scripts/verify-local-consumer.sh
git diff --check <recorded-Gate-A-accounting-hash>..HEAD
```

### Configurable-options rollback group and exact current accounting

- [x] The configurable-options rollback group is separate from the completed fieldless `x_search` rollback group. If required, one coordinated committed rollback removes the new `ResponseXSearchOptionsTool`, its encoder/validator branches and focused tests, Chunk 31A external contract/export entries, option-specific docs/evidence/support claims, and maintainer/release statements. It must preserve all completed Chunk 31–34 and historical Chunk 26–30 accounting as truthful append-only history, then append rollback accounting with the removed API, exact predecessor and rollback hashes/subjects/paths, verification results, and review disposition. It must leave `ResponseXSearchTool{}`, its exact `{"type":"x_search"}` encoding, the two built-in-tool methods/wrapper, fieldless evidence, `web_search`, and raw outputs/events intact.
- [x] Safe sequential implementation, audit, documentation, status, and tracker-only accounting commits may exist, but no intermediate state may mark the configurable feature complete or release it before audit, docs, accounting handoffs, and Chunk 34 close. No state may retain support claims for removed options, remove the fieldless API with only the configurable group, or rewrite old closed-chunk history. If a published configurable contract is defective, issue a new patch and follow the repository's immutable-tag/retract/advisory procedure; never move or reuse a tag.
- [-] **Current implementation accounting:** Gate A accounting landed as `d0dfb25326cbf58775c9c109420920e7c9148e08` (`docs: record gateway x search option evidence`) with only `planning/IMPLEMENTATION_PLAN.md`. Chunk 31's green draft `dda9dfe055cc1a590afc2f3de49409472f3752a0` (`feat: add gateway x search options`) changed exactly `responses_test.go`, `responses_tools.go`, `responses_validate.go`, and `responses_wire.go`; focused ordinary, focused race, and full ordinary tests passed. Review found the invented empty-date rejection and missing singleton/public-method coverage. Fix `e3697d84252ac1d59fd65e19d95b5b52252bb38b` (`fix: correct gateway x search option contract`) changed exactly `responses_test.go` and `responses_validate.go`; post-fix focused ordinary, focused race, full ordinary, and full race tests passed and three independent rereviews were clean. Chunk 31 accounting `f9b5b296f305e2c368f781b573e307228013e61c` (`docs: record gateway x search option implementation`) changed only `planning/IMPLEMENTATION_PLAN.md`. Chunk 31A green draft `94e9efa1c4f5f44a983da5efdce43e9a75adc70d` (`test: audit gateway x search options`) changed exactly `contract_external_test.go` and `scripts/verify-local-consumer.sh`; both initial acceptance commands passed. Review found missing present-empty date external coverage, and a separate audit found incomplete environment scrubbing. Fix `8180a6471adbfe54b50ab67ace5f4bb411cf7cf1` (`fix: complete x search external contract`) changed exactly those same two paths; both post-fix acceptance commands passed and a fresh independent rereview was clean. The external boundary is the exact six-field `ResponseXSearchOptionsTool`, value/non-nil-pointer sealed-interface conformance, all cleared presence forms including present-empty dates, both built-in-tool consumer methods, strict rejection of unapproved exports/fields/aliases/provider escapes/direct-xAI and typed output/event APIs, and the unchanged fieldless/wrapper/method/raw contracts. The consumer audit scrubs all ten credential, live-acknowledgement, and X-search-probe variables named in Chunk 31A and remains offline with `GOWORK=off` and `GOPROXY=off`. No Go LSP claim is made. The tracker-only handoff has intended subject `docs: record gateway x search option contract` and sole path `planning/IMPLEMENTATION_PLAN.md`; it records predecessors through `8180a6471adbfe54b50ab67ace5f4bb411cf7cf1` without claiming its own hash. Chunk 31A becomes complete and Chunk 32 becomes dependency-ready only after that handoff lands. The configurable option API, audits, and all later option-specific docs/status claims remain one rollback group, while fieldless `x_search` and immutable historical accounting remain preserved.
