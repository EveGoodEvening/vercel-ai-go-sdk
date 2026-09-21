# Evaluation Model V4 guide

This package implements the experimental AI SDK Gateway provider protocol at `POST https://ai-gateway.vercel.sh/v4/ai/evaluation-model`. `Evaluate` judges shared state against keyed questions; it does not generate text. The same client separately implements public Responses and Chat generation documented in [generation.md](generation.md), using an independently configurable `/v1` base URL. Neither surface implements the separate public REST `POST /v1/evaluate` API. Search remains unsupported: the [Gateway-native xAI `x_search` decision](x-search.md) is unconfirmed, direct xAI support is a different contract, and this package provides no direct xAI client.

Hermetic fixtures cover provider evaluation, but the authorized paid evaluation live-contract check has not run. [`evaluation-live-evidence.md`](evaluation-live-evidence.md) is the sole record and currently says provider evaluation `NOT RUN` / `PENDING LIVE RUN`; public generation has its own independently pending record. Neither contract is evidence for the other.

## Constructing a client

`NewClient` applies options in order, stops at the first error, copies accepted value configuration, and performs no network call.

### Credentials

Credential selection is fixed:

1. explicit `WithAPIKey`;
2. nonblank `AI_GATEWAY_API_KEY`;
3. explicit `WithOIDCToken` or `WithOIDCTokenSource` (whichever explicit OIDC option appears last);
4. nonblank `VERCEL_OIDC_TOKEN`;
5. `ConfigurationError("credentials", "no credential configured")`.

An environment API key beats explicit OIDC. An explicit API key always wins, even if rejected by the server; there is no request-time fallback. Empty/all-Unicode-whitespace environment values are absent. Explicit fixed credentials reject blank values; otherwise all credential strings are preserved exactly, including surrounding whitespace in a nonblank value.

A selected `TokenSource` is invoked once for each HTTP attempt with the `Evaluate` context. A source error or blank token is `TransportError` operation `resolve OIDC token`, is never replaced by another credential, and remains reachable through `Unwrap`.

### Option contract

- `WithBaseURL`: input must be nonempty, contain no leading/trailing Unicode whitespace, and parse as an absolute, non-opaque `http` or `https` URL with host and no userinfo, query, or fragment. Existing paths are allowed. Every trailing `/` is removed from the path; if empty, the origin has no trailing slash. Scheme, host, port, escaping, and remaining path are preserved, then `/evaluation-model` is appended. Invalid input is not trimmed or repaired.
- `WithPublicBaseURL`: configures only the public Responses and Chat base URL described in [generation.md](generation.md). It does not affect `Evaluate`; conversely, `WithBaseURL` does not affect generation. It applies the same URL requirements and additionally rejects a literal fragment delimiter.
- `WithHTTPClient`: nil is invalid. The exact pointer is stored and used; the SDK does not clone or mutate the client or transport. The caller owns concurrency safety for subsequent mutation.
- `WithAPIKey`, `WithOIDCToken`, `WithTeam`: empty/all-Unicode-whitespace input is invalid; accepted input is preserved exactly. Team has no environment fallback.
- `WithOIDCTokenSource`: nil interfaces and typed-nil sources are invalid.
- `WithHeaders`: nil means no caller headers. Non-nil input is cloned with `http.Header.Clone` when applied and cloned again for each request. Original key spelling, value order, duplicates, nil/empty slices, and empty strings are retained; later caller mutation cannot affect the client.
- Protected header names are matched case-insensitively: `Authorization`, `Content-Type`, `Ai-Gateway-Protocol-Version`, `Ai-Gateway-Auth-Method`, `Ai-Evaluation-Model-Specification-Version`, `Ai-Model-Id`, and `X-Vercel-Ai-Gateway-Team`. Presence alone is invalid, even with a nil/empty value slice. Other caller headers are added before the SDK writes one owned value for each protocol header.
- `WithRetryPolicy` copies and resolves the policy as specified under retries.

### `ConfigurationError` closed mappings

`Option()` returns only `WithBaseURL`, `WithPublicBaseURL`, `WithHTTPClient`, `WithAPIKey`, `WithOIDCToken`, `WithOIDCTokenSource`, `WithTeam`, `WithHeaders`, `WithRetryPolicy`, or `credentials`.

`Reason()` returns only `must not be empty or whitespace`, `must be an absolute http or https URL`, `must not contain userinfo, query, or fragment`, `must not be nil`, `contains protected header`, `invalid MaxAttempts`, `invalid InitialDelay`, `invalid MaxDelay`, `MaxDelay is less than InitialDelay`, `invalid Multiplier`, `invalid Jitter`, or `no credential configured`.

URL parse/shape failures use the absolute-URL reason; userinfo/query/fragment use their dedicated reason. Blank fixed credential/team/base URL, nil client/source, protected header, retry field, retry cross-field, and exhausted credential selection map directly to the option/reason names above. No other pair is emitted.

## Public request

```go
type EvaluationRequest struct {
    State           any
    Questions       map[string]Question
    ProviderOptions map[string]map[string]any
}
```

The model ID and question IDs must be nonempty; `Questions` must be nonempty and each value non-nil. Every question requires `Instructions`.

`State`, `Instructions`, and each non-null criteria description must be a JSON-compatible string, object, or array. General nested JSON values may be null, booleans, finite numbers, strings, arrays/slices, and string-keyed objects. Validation occurs before network I/O and rejects unsupported values, non-string map keys, cycles, NaN, and infinity.

`ProviderOptions` is optional. Each provider value must be a non-nil object and its nested values follow general JSON rules. Nil omits `providerOptions`; a non-nil empty map emits `{}`.

Questions are a closed interface:

- `BooleanQuestion`: `Criteria == nil` omits `criteria`; `&BooleanCriteria{}` emits `{}`. `OptionalJSON{Set:false, Value:nil}` omits its key, `Set:true` emits `Value` including explicit JSON null, and `Set:false` with non-nil `Value` is invalid.
- `ChoiceQuestion`: `Criteria` needs at least one keyed option.
- `ScoreQuestion`: `Criteria` needs at least two ordered levels; response score indexes them from zero.

The body contains only `state`, `questions`, and optional `providerOptions`. The model ID is sent only in `ai-model-id`, never as a JSON `model` member.

### Request validation paths and reasons

`ValidationError.Path()` starts at `$`. Map/object members append `[` plus a Go `strconv.Quote` JSON-style quoted UTF-8 key plus `]`; arrays append a base-10 index without leading zeros. Examples: `$["state"]`, `$["questions"]["quality.score"]`, `$["criteria"][0]`, `$["providerOptions"]["acme"]["a\\\"b"]`. Container failures name the container; bad children name the child. Cycles name the child edge that first points into the active recursion stack. Defined fields are visited in order, map keys lexicographically, and indexes ascending, making the first error deterministic.

`Reason()` is exactly one of:

- `required`
- `must be nonempty`
- `must be a string, object, or array`
- `must be JSON-compatible`
- `map keys must be strings`
- `number must be finite`
- `cycle detected`
- `must contain at least one option`
- `must contain at least two levels`
- `must be a non-nil question`
- `question type is unsupported`
- `must be omitted when Set is false`
- `must be a non-nil object`

`Evaluate(nil, ...)` returns path `$["context"]`, reason `required`, before credential-source or transport work.

## Wire request

Each attempt sends `POST {baseURL}/evaluation-model` with `Authorization: Bearer <credential>`, `Content-Type: application/json`, `ai-gateway-protocol-version: 0.0.1`, `ai-gateway-auth-method: api-key|oidc`, `ai-evaluation-model-specification-version: 4`, `ai-model-id: <modelID>`, and optional `x-vercel-ai-gateway-team`.

## Response classification and validation

Only status 200 is success. Every other status, including other 2xx statuses, is a `ResponseError`. Response `Content-Type` is ignored because an intermediary may omit or rewrite it.

A status-200 body must be at most 1 MiB, contain exactly one JSON value, and then only JSON whitespace. Syntax errors are `malformed JSON`; a second value is `trailing JSON value`. Duplicate keys are rejected recursively, including answer, probability, provider, and nested provider-metadata maps. Unknown fields are rejected at each fixed DTO level: top-level result, answer variants, rounding, usage, and warning variants. Answer maps, probability maps, provider-name maps, and nested JSON metadata are open only where their contract says so; duplicate, key-set, and value invariants still apply.

There must be exactly one answer for each question and none for unknown IDs. Types must match:

- boolean probability is finite in `[0,1]` and means `P(true)`;
- a choice must name a criterion;
- a score is in `[0,n-1]`.

For choice and score, an absent `probabilities` key exports a nil map and skips distribution/max/weighted-mean checks. Explicit JSON null is invalid. `{}` is invalid. A present object must have exactly all expected keys, no extras, finite values in `[0,1]`, and an acceptable sum. Choice requires the selected probability to be maximal within tolerance; score requires the score to match the probability-weighted mean.

Let `baseTolerance=1e-6`. For declared decimals `d`, `roundingError(d)=0.5*10^(-d)`; absent rounding contributes zero. Decimal counts are integers in `[0,15]`. For `k` probabilities, the sum tolerance is `baseTolerance + k*probabilityError`. For score levels `0..n-1`, `mean=Σ(i*p_i)`, `meanRoundingError=Σ(i*probabilityError)`, and tolerance is `baseTolerance + meanRoundingError + scoreError`. Boundaries are inclusive; values are never renormalized.

### Response validation paths and reasons

`ResponseValidationError.Path()` uses the same `$`, quoted-member, and index grammar, but names wire fields. Top-level members are `$["answers"]`, `$["rounding"]`, `$["usage"]`, `$["warnings"]`, and `$["providerMetadata"]`. Answer fields append the question ID and `type`, `probability`, `choice`, `score`, or `probabilities`; probability entries append their choice or decimal score key. Rounding fields are `probabilityDecimals`/`scoreDecimals`; usage fields are `inputTokens`/`outputTokens`; warning paths include their array index and field; provider metadata appends provider and nested member/index paths. Missing members name where they should be, extras name their actual path, whole-container invariants name the container, syntax/trailing failures use `$`, and duplicate keys name the duplicate's innermost path.

`Reason()` is exactly one of:

- `malformed JSON`, `trailing JSON value`, `duplicate field`, `unknown field`
- `required`, `must be an object`, `must be an array`, `must be a string`, `must be a number`, `must be an integer`, `must be non-null`, `must be nonempty`
- `number must be finite`, `must be between 0 and 1`, `must be between 0 and 15`
- `answer is missing`, `answer is unexpected`, `answer type does not match question`, `answer type is unsupported`
- `choice is not in criteria`, `probability key is missing`, `probability key is unexpected`, `probabilities must sum to 1 within tolerance`, `selected choice is not maximal`
- `score is out of range`, `score does not match weighted mean`
- `warning type is unsupported`, `field is forbidden for warning type`, `must be a non-nil object`

Null normally maps to `must be non-null`; a null provider object maps to `must be a non-nil object`. Structural errors precede semantic validation.

## Successful metadata

- `Rounding`: absent object yields nil; `{}` yields non-nil empty `Rounding`. Each optional integer pointer may hold zero and must be in `[0,15]`.
- `Usage`: absent object yields nil; `{}` yields non-nil empty `Usage`. Present token pointers may hold zero and must be finite, non-negative JSON integers representable as `int64`.
- `Warnings`: absence alone normalizes to a non-nil empty slice; explicit null is invalid. `unsupported`/`compatibility` require `Feature` and optionally `Details`; `deprecated` requires `Setting` and `Message`; `other` requires `Message`. `Details==nil` means omitted, not an empty string. Forbidden fields are rejected.
- `ProviderMetadata`: absence yields nil; `{}` is a non-nil empty map. Provider values must be non-null JSON objects. Nested data is validated without normalization.
- `ResponseMetadata`: `ModelID` is the caller's model ID. `Headers` and `Body` are defensive copies and non-nil after a successfully read response, even for empty body; zero values mean no response metadata. The fixed retained-body limit is 1 MiB.

`ResponseMetadata.Body` can contain echoed state/provider options. Treat it as sensitive and sanitize before logging or persistence.

## Errors and safe diagnostics

All error types support `errors.As`. Their fields are private. Every accessor is nil-safe; strings/status/durations/bools return zero values, `RetryAfter` returns `(0,false)`, `Param`/`RawResponseBody`/`Unwrap` return nil, and nil `Error()` returns the stable phrase `gateway configuration error`, `gateway validation error`, `gateway transport error`, `gateway response error`, or `gateway response validation error`.

- `ConfigurationError`: `Option`, `Reason`; never unwraps.
- `ValidationError`: request `Path`, `Reason`; never unwraps.
- `TransportError`: closed `Operation`, `Unwrap`.
- `ResponseError`: `StatusCode`, `Message`, `Type`, `Code`, defensive-copy `Param`, `GenerationID`, `RequestID`, `ResponseID`, `RetryAfter`, `Retryable`, `BodyTruncated`, defensive-copy `RawResponseBody`, and `Unwrap`.
- `ResponseValidationError`: status 200, wire `Path`, `Reason`, `RequestID`, `ResponseID`, `BodyTruncated`, defensive-copy `RawResponseBody`, and `Unwrap`.

`ResponseError` parses only `{error:{message,type,code,param},generationId,requestId,responseId}`. String code is unchanged; numeric code preserves its JSON lexical form; absent/null is empty. IDs come only from top-level string fields, never headers. `Param` returns a fresh recursive copy. Malformed envelopes do not prevent status/body metadata.

`TransportError.Operation()` is closed to `resolve OIDC token`, `encode request`, `create request`, `send request`, and `read response body`. Token source error/blank token, post-validation JSON encoding, request construction, `http.Client.Do`, and bounded body read/overflow/close respectively map to those values. For status 200 a body failure is a `TransportError`; for non-200 it is a `ResponseError` preserving status and wrapping the body-read `TransportError`.

Both `RawResponseBody` methods return nil if nothing was retained and otherwise a fresh copy. They may contain echoed sensitive state/provider options. Do not log or persist them unsanitized. Error strings intentionally omit bodies, headers, credentials, state, provider options, and wrapped error text. Log safe structured fields instead:

```go
var responseErr *gateway.ResponseError
if errors.As(err, &responseErr) {
    log.Printf("gateway status=%d type=%q code=%q request_id=%q retryable=%t",
        responseErr.StatusCode(), responseErr.Type(), responseErr.Code(),
        responseErr.RequestID(), responseErr.Retryable())
}
```

## Retries

`MaxAttempts` includes the initial request. Values 0 and 1 resolve to exactly one attempt; 2–10 enable retries. Enabled-policy zero defaults are `InitialDelay=100*time.Millisecond`, `MaxDelay=2*time.Second`, `Multiplier=2`; jitter zero means none. Explicit `InitialDelay` is 1ms–1m, `MaxDelay` is 1ms–5m and at least the resolved initial delay, `Multiplier` is 1–10, and `Jitter` is 0–1. Negative/out-of-range attempts or durations and NaN/infinite/out-of-range floats are configuration errors. Durations are Go `time.Duration`.

Before retry number `r` (first retry is 1), base delay is `min(MaxDelay, InitialDelay*Multiplier^(r-1))`, saturating rather than overflowing. With uniform `u` in `[0,1)`, jitter changes it to `delay*(1+Jitter*(2*u-1))`, rounded to the nearest nanosecond and capped again. Valid `Retry-After` seconds or HTTP date replaces that delay and is capped; malformed/negative values are absent. Waiting is context-cancellable.

Retryability is **only** `status==408 || status==409 || status==429 || 500<=status<=599`. Parsed or malformed error type/code never changes it; `Retry-After` never makes another status retryable. A retry occurs only with an attempt remaining, successfully read response body, and active context. Body-read failures are never retried. The default is one attempt because evaluation can duplicate billable/non-idempotent work. The same policy is shared by buffered public generation calls; generation streams are never retried after headers.

## Additive generation migration

Generation support does not change this evaluation contract. Existing callers keep `Evaluate` and `WithBaseURL`. New Responses or Chat callers use the APIs in [generation.md](generation.md) and may configure `WithPublicBaseURL`; there is no alias, endpoint auto-detection, or conversion between evaluation and generation requests.

## Runnable example

Run [`../examples/evaluate/main.go`](../examples/evaluate/main.go) only with `AI_GATEWAY_API_KEY` or `VERCEL_OIDC_TOKEN` already set:

```sh
go run ./examples/evaluate
```

The example prints typed answers and safe structured metadata/error fields. It never prints credentials, raw response bodies, raw headers, state, provider options, provider metadata values, or raw diagnostic bodies.
