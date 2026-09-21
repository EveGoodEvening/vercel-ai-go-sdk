# Evaluation Model V4

Evaluation is experimental. `Client.Evaluate` calls the AI SDK Gateway **provider** endpoint:

`POST https://ai-gateway.vercel.sh/v4/ai/evaluation-model`

This is separate from the public API, and this SDK does not implement public `POST /v1/evaluate`. Provider evaluation and public generation also use separately configurable base URLs. See [client.md](client.md) for authentication, endpoints and HTTP options, retries, errors, cancellation, and privacy guidance. The authorized paid live contract remains **NOT RUN**; see [evaluation-live-evidence.md](evaluation-live-evidence.md).

## Run an evaluation

This helper assumes a configured client and a live context. It asks three questions about the same state and prints only the answer count:

```go
func evaluateProposal(ctx context.Context, client *gateway.Client) error {
	result, err := client.Evaluate(ctx, "typesafe-ai/jev-latest", gateway.EvaluationRequest{
		State: map[string]any{
			"requirements": []any{"clear", "testable", "bounded"},
			"proposal":     "Cache successful lookups for five minutes.",
		},
		Questions: map[string]gateway.Question{
			"acceptable": gateway.BooleanQuestion{
				Instructions: "Does the proposal satisfy the requirements?",
			},
			"main_risk": gateway.ChoiceQuestion{
				Instructions: "Select the main risk.",
				Criteria: map[string]any{
					"staleness": "Users may observe stale data.",
					"load":      "The proposal may increase upstream load.",
				},
			},
			"quality": gateway.ScoreQuestion{
				Instructions: "Score the proposal against the requirements.",
				Criteria: []any{"Does not satisfy them", "Partly satisfies them", "Fully satisfies them"},
			},
		},
	})
	if err != nil {
		return err
	}
	fmt.Printf("answers=%d\n", len(result.Answers))
	return nil
}
```

`State` is shared by every question. Model and question IDs must be nonempty; the question map must be nonempty with non-nil questions. State, instructions, and non-null criteria descriptions must be JSON-compatible strings, objects, or arrays. Nested values may also contain null, booleans, and finite numbers. Unsupported values, cycles, non-string map keys, NaN, and infinity fail locally before network work.

`ProviderOptions` is optional: nil omits it, an empty map emits `{}`, and each provider's value must be a non-null JSON-compatible object. The model ID is sent in `ai-model-id`, not the body; the body contains only `state`, `questions`, and optional `providerOptions`.

## Boolean, choice, and score questions

`result.Answers` is keyed by question ID. Type-assert or type-switch each `gateway.Answer` to access its concrete fields:

| Question | Criteria | Answer |
| --- | --- | --- |
| `BooleanQuestion` | Optional true/false descriptions | `BooleanAnswer.Probability`: finite `P(true)` in `[0,1]` |
| `ChoiceQuestion` | At least one keyed option | `ChoiceAnswer.Choice`: a declared key; optional `Probabilities` keyed by those choices |
| `ScoreQuestion` | At least two ordered levels | `ScoreAnswer.Score`: potentially fractional, on the zero-based scale; optional `Probabilities` keyed by decimal indexes |

Question IDs should be stable application-owned names. Map order has no meaning; score criteria order defines the scale. Use [`examples/evaluate`](../examples/evaluate/main.go) for a complete command (`go run ./examples/evaluate`); it requires credentials and may incur charges.

## Omitted values, JSON null, and metadata presence

Boolean criteria need a three-way presence distinction. For example, this describes `true` and explicitly sends null for `false`:

```go
criteria := &gateway.BooleanCriteria{
	True:  gateway.OptionalJSON{Set: true, Value: "Meets the rule"},
	False: gateway.OptionalJSON{Set: true, Value: nil},
}
```

- `Criteria == nil` omits the entire `criteria` field.
- `&gateway.BooleanCriteria{}` emits an empty criteria object.
- `OptionalJSON{Set: false, Value: nil}` omits that key.
- `OptionalJSON{Set: true, Value: nil}` emits that key as JSON null.
- `Set: false` with a non-nil `Value` is invalid.

Successful result metadata also preserves wire presence where it matters:

- Absent `rounding` or `usage` becomes a nil pointer; an explicit `{}` becomes a non-nil empty struct. Their field pointers distinguish an absent key from a present zero.
- Absent `providerMetadata` becomes nil; `{}` becomes a non-nil empty map. Provider values must be non-null objects.
- Absent `warnings` normalizes to a non-nil empty slice. Explicit JSON null is invalid.
- Absent choice/score `probabilities` becomes a nil map. Explicit null and `{}` are invalid.
- Successful `Response.Headers` and `Response.Body` are defensive, non-nil copies. The body may echo sensitive state or provider options; do not log or persist it unsanitized.

## Strict response validation

Only HTTP status `200` is treated as success. Its body is bounded to 1 MiB and strictly validated before an `EvaluationResult` is returned:

- exactly one JSON value is allowed;
- duplicate keys and unknown fields in fixed response objects are rejected;
- there must be exactly one answer per requested question and no extra answers;
- each answer discriminator must match its question type;
- boolean probabilities must be finite and within `[0,1]`;
- choices must name a declared criterion;
- scores must be finite and within `0..len(criteria)-1`.

Choice and score probability maps are optional. When absent, distribution checks are skipped. When present, they must be nonempty, contain exactly the expected keys, contain only finite values in `[0,1]`, and sum to one within tolerance. A selected choice must be maximal within the base tolerance. A score must equal the distribution's weighted mean within tolerance.

The base tolerance is `1e-6`. If the response declares `probabilityDecimals` or `scoreDecimals`, each integer must be in `0..15`; a declaration of `d` contributes half a unit in the last place, `0.5 * 10^-d`. For `k` probabilities, sum tolerance is:

`1e-6 + k*probabilityRoundingError`

For score levels indexed `0..n-1`, weighted-mean tolerance is:

`1e-6 + sum(i*probabilityRoundingError) + scoreRoundingError`

Boundaries are inclusive, and the SDK never renormalizes provider values. Invalid successful responses return `*gateway.ResponseValidationError`, whose `Path()` identifies the wire field and whose `Reason()` describes the failed invariant.

## Full contract reference

Use the source declarations and validation implementation for exhaustive fields and diagnostics rather than copying long inventories into application documentation:

- [evaluation.go](../evaluation.go): public request, question, answer, result, and metadata types
- [evaluation_validate.go](../evaluation_validate.go): request validation and presence rules
- [evaluation_response.go](../evaluation_response.go): strict decoding, answer validation, and rounding tolerances
- [evaluation_wire.go](../evaluation_wire.go): request wire encoding

For shared client behavior, use [client.md](client.md). For generation APIs, use [generation.md](generation.md).
