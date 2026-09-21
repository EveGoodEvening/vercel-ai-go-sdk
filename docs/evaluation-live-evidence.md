# Live-contract evidence

## Current status

**Public generation live execution:** NOT RUN

**Provider evaluation live execution:** NOT RUN

**Blocker:** No owner-authorized paid-network execution, protected-environment run, or sanitized live result exists. No credential or acknowledgement value was inspected while preparing this template. All records below remain **PENDING LIVE RUN** until an authorized operator runs the isolated contracts.

This file is the sole sanitized record for the live contracts. Never paste credentials, authorization headers, prompts, evaluation state, generated output, tool arguments or results, raw request or response payloads, raw headers, provider-option values, or provider-metadata values into this file or CI artifacts.

## Pinned public generation contract

- Model: `openai/gpt-5-nano`
- Responses endpoint: default `POST https://ai-gateway.vercel.sh/v1/responses`
- Chat endpoint: default `POST https://ai-gateway.vercel.sh/v1/chat/completions`

## Pinned provider evaluation contract

- Model: `typesafe-ai/jev-latest`
- Endpoint: default `POST https://ai-gateway.vercel.sh/v4/ai/evaluation-model`
- Protocol evidence baseline: 2026-09-20
- `ai`: `7.0.107`
- `@ai-sdk/gateway`: `4.0.87`
- `@ai-sdk/provider`: `4.0.17`
- `@ai-sdk/provider-utils`: `5.0.45`

## Authorized isolated commands

Each command requires exactly one nonblank production credential across `AI_GATEWAY_API_KEY` and `VERCEL_OIDC_TOKEN`. The test gate fails before network if its acknowledgement is not the sole exact sentinel, if the opposite acknowledgement is present at any value, or if zero or multiple credentials are available. The selected credential is passed explicitly to the client.

Public generation, covering Responses non-stream/stream and Chat non-stream/stream with minimal prompts:

```sh
env -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK AI_GATEWAY_PUBLIC_LIVE_COST_ACK=I_ACCEPT_LIVE_PUBLIC_API_COSTS go test -tags=livecontract ./internal/livecontract -run '^TestGateway(Responses|Chat)Contract$' -count=1
```

Provider evaluation:

```sh
env -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK AI_GATEWAY_LIVE_COST_ACK=I_ACCEPT_LIVE_EVALUATION_COSTS go test -tags=livecontract ./internal/livecontract -run '^TestGatewayEvaluationContract$' -count=1
```

These examples select an already-present `AI_GATEWAY_API_KEY`. For an authorized OIDC run, invert the credential selection by unsetting `AI_GATEWAY_API_KEY` and retaining only `VERCEL_OIDC_TOKEN`. Never run both contracts in one process or set both acknowledgements.

## Sanitized public generation record

Complete only from real successful runs. Record only the structural facts asserted by the live contract, never content. The contract checks the pinned model wherever the wire response exposes it, requires every buffered Chat choice to carry a nonblank finish reason, requires a `response.completed` terminal Responses stream event, and minimally decodes buffered Responses output item discriminators. These checks do not authorize retaining raw payloads or generated values.

- Execution date (UTC): **PENDING LIVE RUN**
- Operator/run reference (non-secret): **PENDING LIVE RUN**
- Model ID observed: **PENDING LIVE RUN** (record exact-match pass/fail for `openai/gpt-5-nano`; do not retain any other model value)
- Responses non-stream result: **PENDING LIVE RUN** (record pass/fail and structural output item types only)
- Responses stream result: **PENDING LIVE RUN** (record pass/fail, structural event types, and `response.completed` presence only)
- Chat non-stream result: **PENDING LIVE RUN** (record pass/fail and finish-reason presence only; omit reason values)
- Chat stream result: **PENDING LIVE RUN** (record pass/fail and pinned-model match wherever exposed only)
- Usage shape: **PENDING LIVE RUN** (record absent/present and field names only; omit counts)
- Response metadata shape: **PENDING LIVE RUN** (record model-ID match and metadata presence only; omit IDs, headers, and body bytes)
- Protocol drift: **PENDING LIVE RUN** (record `none observed` only after every applicable generation smoke succeeds; otherwise record only the sanitized structural difference)

## Sanitized provider evaluation record

Complete only from a real successful run. Never infer or invent values.

- Execution date (UTC): **PENDING LIVE RUN**
- Operator/run reference (non-secret): **PENDING LIVE RUN**
- Result: **PENDING LIVE RUN**
- Model ID observed: **PENDING LIVE RUN**
- Answer shape: **PENDING LIVE RUN** (record only `boolean`, `choice`, and `score` type presence; omit answer values)
- Rounding shape: **PENDING LIVE RUN** (record only absent/present and which integer fields were present)
- Usage shape: **PENDING LIVE RUN** (record only absent/present and which integer fields were present; omit counts)
- Warning shape: **PENDING LIVE RUN** (record only warning discriminators; omit feature, setting, details, and message values)
- Provider metadata shape: **PENDING LIVE RUN** (record only absent/present and provider names after confirming names contain no sensitive data; omit all values)
- Response metadata shape: **PENDING LIVE RUN** (record only exact model-ID match, non-nil headers, non-nil body, and body byte length; omit header names/values and body bytes)
- Protocol drift: **PENDING LIVE RUN** (record `none observed` only after a successful run; otherwise describe only the sanitized structural difference and stop release-candidate promotion)

## Sanitization review

Before retaining evidence, confirm all of the following:

- [ ] No credential, acknowledgement, authorization value, request ID, or response ID is present.
- [ ] No prompt, evaluation state, generated output, output-item content, finish-reason value, selected choice, score, probability, tool argument, or tool result is present.
- [ ] No raw request, response header, response body, stream event payload, provider-option value, or provider-metadata value is present.
- [ ] No token count, warning payload, or other billable-content detail is present.
- [ ] Public model evidence records only exact-match pass/fail for the pinned model, never an unexpected model value.
- [ ] Each recorded result came from its exact isolated authorized command and was not reconstructed from a mock or fixture.
- [ ] Public generation and provider evaluation evidence remain independently attributable; neither run inherited the opposite acknowledgement.
