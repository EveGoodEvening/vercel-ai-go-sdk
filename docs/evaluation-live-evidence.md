# Evaluation live-contract evidence

## Current status

**Live execution:** NOT RUN

**Blocker:** This implementation task did not have an authorized live-run context: network access was prohibited, and no credential or cost-acknowledgement value was inspected. The release-candidate live-run checklist remains incomplete until an authorized operator runs the exact command below with one production credential already present and the exact cost sentinel.

This file is the sole record for the Chunk 09 live evaluation run. Do not paste credentials, authorization headers, raw response headers, raw response bodies, request state, provider-option values, or provider-metadata values into this file or CI artifacts.

## Pinned contract

- Model: `typesafe-ai/jev-latest`
- Endpoint: default `POST https://ai-gateway.vercel.sh/v4/ai/evaluation-model`
- Protocol evidence baseline: 2026-09-20
- `ai`: `7.0.107`
- `@ai-sdk/gateway`: `4.0.87`
- `@ai-sdk/provider`: `4.0.17`
- `@ai-sdk/provider-utils`: `5.0.45`

## Authorized command

Run with exactly one authorized production credential (`AI_GATEWAY_API_KEY` or `VERCEL_OIDC_TOKEN`) already present in the environment:

```sh
AI_GATEWAY_LIVE_COST_ACK=I_ACCEPT_LIVE_EVALUATION_COSTS go test -tags=livecontract ./internal/livecontract -run TestGatewayEvaluationContract
```

The test skips without sending a request unless a credential variable is present and `AI_GATEWAY_LIVE_COST_ACK` exactly equals `I_ACCEPT_LIVE_EVALUATION_COSTS`. It never substitutes a mock and never sends an error-provoking request.

## Sanitized run record template

Complete this section only from a real successful run. Never infer or invent values.

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
- Protocol drift: **PENDING LIVE RUN** (record `none observed` only after a successful run, otherwise describe the sanitized structural difference and stop release-candidate promotion)

## Sanitization review

Before retaining evidence, confirm all of the following:

- [ ] No credential or authorization value is present.
- [ ] No raw request state or provider-option value is present.
- [ ] No raw response header or body is present.
- [ ] No provider-metadata value is present.
- [ ] No subjective answer text, selected choice, score, probability, token count, warning payload, request ID, or response ID is present.
- [ ] The recorded result came from the exact authorized command and was not reconstructed from a mock or fixture.
