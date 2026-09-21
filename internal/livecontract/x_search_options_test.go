//go:build livecontract

package livecontract_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const (
	xSearchOptionsEndpoint       = "https://ai-gateway.vercel.sh/v1/responses"
	xSearchOptionsModel          = "spacexai/grok-4.6"
	xSearchOptionsAckEnv         = "AI_GATEWAY_X_SEARCH_LIVE_COST_ACK"
	xSearchOptionsAck            = "I_ACCEPT_LIVE_X_SEARCH_COSTS"
	xSearchOptionsBodyMax        = 1 << 20
	xSearchOptionsMaxCalls       = 14
	xSearchOptionsRequestTimeout = 30 * time.Second
	xSearchOptionsOverallTimeout = 8 * time.Minute
)

var xSearchGeneralAckEnvs = [...]string{"AI_GATEWAY_LIVE_COST_ACK", "AI_GATEWAY_PUBLIC_LIVE_COST_ACK"}
var xSearchPrivateInputEnvs = [...]string{
	"AI_GATEWAY_X_SEARCH_PROBE_INPUT",
	"AI_GATEWAY_X_SEARCH_PROBE_HANDLE_A",
	"AI_GATEWAY_X_SEARCH_PROBE_HANDLE_B",
	"AI_GATEWAY_X_SEARCH_PROBE_FROM_DATE",
	"AI_GATEWAY_X_SEARCH_PROBE_TO_DATE",
}

type xSearchCredential struct {
	value string
}

type xSearchPrivateInputs struct {
	probeInput string
	handleA    string
	handleB    string
	fromDate   string
	toDate     string
}

type xSearchPrerequisites struct {
	credential xSearchCredential
	private    xSearchPrivateInputs
}

type xSearchProbeCase struct {
	label          string
	options        map[string]any
	wantSuccess    bool
	optionClasses  []string
	wrongKindCount int
}

type xSearchProbeRecord struct {
	caseLabel      string
	optionClasses  []string
	optionCount    int
	httpStatus     int
	wrongKindCount int
	objectClass    string
	statusClass    string
	outputTypes    []string
	outputStatuses []string
	errorPresent   bool
	errorCategory  string
	errorCode      string
}

type xSearchResponseEnvelope struct {
	Object string `json:"object"`
	Status string `json:"status"`
	Output []struct {
		Type   string `json:"type"`
		Status string `json:"status"`
	} `json:"output"`
	Error json.RawMessage `json:"error"`
}

type countingRoundTripper struct {
	calls atomic.Int64
}

func (transport *countingRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	transport.calls.Add(1)
	return nil, errors.New("unexpected dispatch")
}

func TestGatewayXSearchOptionsContract(t *testing.T) {
	prerequisites, err := resolveXSearchOptionsPrerequisites(os.LookupEnv)
	if err != nil {
		t.Fatal(err)
	}

	overallCtx, cancelOverall := context.WithTimeout(t.Context(), xSearchOptionsOverallTimeout)
	defer cancelOverall()
	client := &http.Client{
		Transport: http.DefaultTransport,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	controlCase := xSearchProbeCase{label: "fieldless-control", wantSuccess: true}
	configurableCases := []xSearchProbeCase{
		{label: "canonical-all-six", options: canonicalXSearchOptions(prerequisites.private), wantSuccess: true, optionClasses: []string{"boolean", "date", "handle-list"}},
		{label: "explicit-empty-and-false", options: emptyXSearchOptions(), wantSuccess: true, optionClasses: []string{"boolean", "handle-list"}},
		{label: "from-only", options: map[string]any{"from_date": prerequisites.private.fromDate}, wantSuccess: true, optionClasses: []string{"date"}},
		{label: "to-only", options: map[string]any{"to_date": prerequisites.private.toDate}, wantSuccess: true, optionClasses: []string{"date"}},
		{label: "wrong-handle-list-family", options: map[string]any{"allowed_x_handles": true, "excluded_x_handles": []string{prerequisites.private.handleB}}, optionClasses: []string{"wrong-handle-list"}, wrongKindCount: 1},
		{label: "wrong-date-family", options: map[string]any{"from_date": false, "to_date": prerequisites.private.toDate}, optionClasses: []string{"wrong-date"}, wrongKindCount: 1},
		{label: "wrong-boolean-family", options: map[string]any{"enable_image_understanding": []string{prerequisites.private.handleA}, "enable_video_understanding": true}, optionClasses: []string{"wrong-boolean"}, wrongKindCount: 1},
	}
	diagnostics := individualXSearchOptionDiagnostics(prerequisites.private)
	if 1+len(configurableCases)+len(diagnostics) > xSearchOptionsMaxCalls {
		t.Fatal("x_search options probe matrix exceeds its hard request cap")
	}

	calls := 0
	runProbe := func(probeCase xSearchProbeCase) (xSearchProbeRecord, error) {
		if calls >= xSearchOptionsMaxCalls {
			t.Fatal("x_search options probe reached its hard request cap")
		}
		requestCtx, cancelRequest := context.WithTimeout(overallCtx, xSearchOptionsRequestTimeout)
		defer cancelRequest()
		calls++
		return runXSearchOptionsProbe(requestCtx, client, prerequisites, probeCase)
	}

	controlRecord, controlErr := runProbe(controlCase)
	if controlErr != nil {
		t.Fatalf("fieldless-control failed before configurable probes: %s", controlErr)
	}
	logXSearchProbeRecord(t, controlRecord)
	if controlRecord.httpStatus < 200 || controlRecord.httpStatus >= 300 {
		t.Fatal("fieldless-control failed before configurable probes: unexpected HTTP status class")
	}

	canonicalNeedsDiagnostics := false
	failures := make([]string, 0)
	for _, probeCase := range configurableCases {
		record, probeErr := runProbe(probeCase)
		if probeErr != nil {
			failures = append(failures, probeCase.label+": "+probeErr.Error())
			continue
		}
		logXSearchProbeRecord(t, record)
		succeeded := record.httpStatus >= 200 && record.httpStatus < 300
		ambiguousCanonicalRejection := probeCase.label == "canonical-all-six" &&
			!succeeded && isSafeXSearchAmbiguousRejection(record)
		if probeCase.label == "canonical-all-six" {
			canonicalNeedsDiagnostics = ambiguousCanonicalRejection
		}
		if probeCase.wantSuccess {
			if !succeeded && !ambiguousCanonicalRejection {
				failures = append(failures, probeCase.label+": unexpected HTTP status class")
			}
		} else if !isAttributableXSearchValidationRejection(record) {
			failures = append(failures, probeCase.label+": expected attributable HTTP validation rejection")
		}
	}

	if canonicalNeedsDiagnostics {
		for _, probeCase := range diagnostics {
			record, probeErr := runProbe(probeCase)
			if probeErr != nil {
				failures = append(failures, probeCase.label+": "+probeErr.Error())
				continue
			}
			logXSearchProbeRecord(t, record)
			if record.httpStatus < 200 || record.httpStatus >= 300 {
				failures = append(failures, probeCase.label+": unexpected HTTP status class")
			}
		}
	}
	if calls > xSearchOptionsMaxCalls {
		t.Fatalf("x_search options probe dispatched %d requests, hard cap is %d", calls, xSearchOptionsMaxCalls)
	}
	if len(failures) != 0 {
		t.Fatalf("x_search options contract failed in %d structurally identified case(s): %s", len(failures), strings.Join(failures, "; "))
	}
}

func TestGatewayXSearchOptionsPrerequisiteMismatchesZeroDispatch(t *testing.T) {
	valid := map[string]string{xSearchOptionsAckEnv: xSearchOptionsAck, "AI_GATEWAY_API_KEY": "credential"}
	for _, name := range xSearchPrivateInputEnvs {
		valid[name] = "nonblank"
	}
	tests := []struct {
		name   string
		change func(map[string]string)
	}{
		{name: "missing dedicated acknowledgement", change: func(env map[string]string) { delete(env, xSearchOptionsAckEnv) }},
		{name: "empty dedicated acknowledgement", change: func(env map[string]string) { env[xSearchOptionsAckEnv] = "" }},
		{name: "wrong dedicated acknowledgement", change: func(env map[string]string) { env[xSearchOptionsAckEnv] = "wrong" }},
		{name: "whitespace dedicated acknowledgement", change: func(env map[string]string) { env[xSearchOptionsAckEnv] = " " + xSearchOptionsAck + " " }},
		{name: "case variant dedicated acknowledgement", change: func(env map[string]string) { env[xSearchOptionsAckEnv] = strings.ToLower(xSearchOptionsAck) }},
		{name: "evaluation acknowledgement present empty", change: func(env map[string]string) { env[xSearchGeneralAckEnvs[0]] = "" }},
		{name: "public acknowledgement present empty", change: func(env map[string]string) { env[xSearchGeneralAckEnvs[1]] = "" }},
		{name: "missing credential", change: func(env map[string]string) { delete(env, "AI_GATEWAY_API_KEY") }},
		{name: "blank API key", change: func(env map[string]string) { env["AI_GATEWAY_API_KEY"] = " \t" }},
		{name: "blank OIDC token", change: func(env map[string]string) { delete(env, "AI_GATEWAY_API_KEY"); env["VERCEL_OIDC_TOKEN"] = "\n" }},
		{name: "ambiguous credentials", change: func(env map[string]string) { env["VERCEL_OIDC_TOKEN"] = "credential" }},
		{name: "missing probe input", change: func(env map[string]string) { delete(env, xSearchPrivateInputEnvs[0]) }},
		{name: "blank probe input", change: func(env map[string]string) { env[xSearchPrivateInputEnvs[0]] = " \t" }},
		{name: "missing first handle", change: func(env map[string]string) { delete(env, xSearchPrivateInputEnvs[1]) }},
		{name: "blank first handle", change: func(env map[string]string) { env[xSearchPrivateInputEnvs[1]] = "\n" }},
		{name: "missing second handle", change: func(env map[string]string) { delete(env, xSearchPrivateInputEnvs[2]) }},
		{name: "blank second handle", change: func(env map[string]string) { env[xSearchPrivateInputEnvs[2]] = " " }},
		{name: "missing start date", change: func(env map[string]string) { delete(env, xSearchPrivateInputEnvs[3]) }},
		{name: "blank start date", change: func(env map[string]string) { env[xSearchPrivateInputEnvs[3]] = "\t" }},
		{name: "missing end date", change: func(env map[string]string) { delete(env, xSearchPrivateInputEnvs[4]) }},
		{name: "blank end date", change: func(env map[string]string) { env[xSearchPrivateInputEnvs[4]] = " \n" }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			env := cloneStringMap(valid)
			test.change(env)
			transport := &countingRoundTripper{}
			clientConstructions := 0
			err := executeXSearchOptionsWithPrerequisites(context.Background(), mapLookup(env), func() *http.Client {
				clientConstructions++
				return &http.Client{Transport: transport}
			}, xSearchProbeCase{label: "unreachable"})
			if err == nil {
				t.Fatal("prerequisite mismatch was accepted")
			}
			if clientConstructions != 0 {
				t.Fatalf("HTTP client constructions = %d, want 0", clientConstructions)
			}
			if got := transport.calls.Load(); got != 0 {
				t.Fatalf("RoundTrip calls = %d, want 0", got)
			}
		})
	}
}

func resolveXSearchOptionsPrerequisites(lookup func(string) (string, bool)) (xSearchPrerequisites, error) {
	ack, present := lookup(xSearchOptionsAckEnv)
	if !present || ack != xSearchOptionsAck {
		return xSearchPrerequisites{}, fmt.Errorf("live Gateway x_search options contract requires %s=%s exactly", xSearchOptionsAckEnv, xSearchOptionsAck)
	}
	for _, name := range xSearchGeneralAckEnvs {
		if _, present := lookup(name); present {
			return xSearchPrerequisites{}, fmt.Errorf("live Gateway x_search options contract requires general acknowledgement %s to be absent", name)
		}
	}
	privateValues := [len(xSearchPrivateInputEnvs)]string{}
	for index, name := range xSearchPrivateInputEnvs {
		value, present := lookup(name)
		if !present || strings.TrimSpace(value) == "" {
			return xSearchPrerequisites{}, fmt.Errorf("live Gateway x_search options contract requires nonblank %s", name)
		}
		privateValues[index] = value
	}
	apiKey, _ := lookup("AI_GATEWAY_API_KEY")
	oidcToken, _ := lookup("VERCEL_OIDC_TOKEN")
	hasAPIKey := strings.TrimSpace(apiKey) != ""
	hasOIDC := strings.TrimSpace(oidcToken) != ""
	if hasAPIKey == hasOIDC {
		return xSearchPrerequisites{}, errors.New("live Gateway x_search options contract requires exactly one nonblank credential")
	}
	credential := xSearchCredential{value: oidcToken}
	if hasAPIKey {
		credential.value = apiKey
	}
	return xSearchPrerequisites{
		credential: credential,
		private: xSearchPrivateInputs{
			probeInput: privateValues[0],
			handleA:    privateValues[1],
			handleB:    privateValues[2],
			fromDate:   privateValues[3],
			toDate:     privateValues[4],
		},
	}, nil
}

func executeXSearchOptionsWithPrerequisites(ctx context.Context, lookup func(string) (string, bool), newClient func() *http.Client, probeCase xSearchProbeCase) error {
	prerequisites, err := resolveXSearchOptionsPrerequisites(lookup)
	if err != nil {
		return err
	}
	_, err = runXSearchOptionsProbe(ctx, newClient(), prerequisites, probeCase)
	return err
}

func runXSearchOptionsProbe(ctx context.Context, client *http.Client, prerequisites xSearchPrerequisites, probeCase xSearchProbeCase) (xSearchProbeRecord, error) {
	tool := map[string]any{"type": "x_search"}
	for key, value := range probeCase.options {
		tool[key] = value
	}
	payload, err := json.Marshal(map[string]any{
		"model":       xSearchOptionsModel,
		"input":       prerequisites.private.probeInput,
		"tools":       []any{tool},
		"tool_choice": "required",
		"stream":      false,
	})
	if err != nil {
		return xSearchProbeRecord{}, errors.New("encode request")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, xSearchOptionsEndpoint, bytes.NewReader(payload))
	if err != nil {
		return xSearchProbeRecord{}, errors.New("construct request")
	}
	req.Header.Set("Authorization", "Bearer "+prerequisites.credential.value)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return xSearchProbeRecord{}, errors.New("dispatch failed")
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, xSearchOptionsBodyMax+1))
	if err != nil {
		return xSearchProbeRecord{}, errors.New("bounded response read failed")
	}
	if len(body) > xSearchOptionsBodyMax {
		return xSearchProbeRecord{}, errors.New("response exceeded bounded read limit")
	}
	var envelope xSearchResponseEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		body = nil
		return xSearchProbeRecord{}, errors.New("response was not structural JSON")
	}
	body = nil

	record := xSearchProbeRecord{
		caseLabel:      probeCase.label,
		optionClasses:  append([]string(nil), probeCase.optionClasses...),
		optionCount:    len(probeCase.options),
		httpStatus:     resp.StatusCode,
		wrongKindCount: probeCase.wrongKindCount,
		objectClass:    safeObjectClass(envelope.Object),
		statusClass:    safeStatusClass(envelope.Status),
		errorCategory:  "absent",
		errorCode:      "absent",
	}
	for _, item := range envelope.Output {
		record.outputTypes = append(record.outputTypes, safeOutputType(item.Type))
		record.outputStatuses = append(record.outputStatuses, safeStatusClass(item.Status))
	}
	record.outputTypes = uniqueSorted(record.outputTypes)
	record.outputStatuses = uniqueSorted(record.outputStatuses)
	if category, code, present := classifyXSearchError(envelope.Error); present {
		record.errorPresent = true
		record.errorCategory = category
		record.errorCode = code
	}
	return record, nil
}

func canonicalXSearchOptions(private xSearchPrivateInputs) map[string]any {
	return map[string]any{
		"allowed_x_handles":          []string{private.handleA},
		"excluded_x_handles":         []string{private.handleB},
		"from_date":                  private.fromDate,
		"to_date":                    private.toDate,
		"enable_image_understanding": true,
		"enable_video_understanding": true,
	}
}

func emptyXSearchOptions() map[string]any {
	return map[string]any{
		"allowed_x_handles":          []string{},
		"excluded_x_handles":         []string{},
		"enable_image_understanding": false,
		"enable_video_understanding": false,
	}
}

func individualXSearchOptionDiagnostics(private xSearchPrivateInputs) []xSearchProbeCase {
	canonical := canonicalXSearchOptions(private)
	keys := []struct {
		label string
		key   string
		class string
	}{
		{"diagnostic-allowed-handles", "allowed_x_handles", "handle-list"},
		{"diagnostic-excluded-handles", "excluded_x_handles", "handle-list"},
		{"diagnostic-from-date", "from_date", "date"},
		{"diagnostic-to-date", "to_date", "date"},
		{"diagnostic-image-understanding", "enable_image_understanding", "boolean"},
		{"diagnostic-video-understanding", "enable_video_understanding", "boolean"},
	}
	result := make([]xSearchProbeCase, 0, len(keys))
	for _, item := range keys {
		result = append(result, xSearchProbeCase{label: item.label, options: map[string]any{item.key: canonical[item.key]}, wantSuccess: true, optionClasses: []string{item.class}})
	}
	return result
}

func isAttributableXSearchValidationRejection(record xSearchProbeRecord) bool {
	return record.wrongKindCount == 1 && isSafeXSearchAmbiguousRejection(record)
}

func isSafeXSearchAmbiguousRejection(record xSearchProbeRecord) bool {
	if !record.errorPresent {
		return false
	}
	if record.httpStatus != http.StatusBadRequest && record.httpStatus != http.StatusUnprocessableEntity {
		return false
	}
	switch record.errorCategory {
	case "authentication_error", "permission_error", "rate_limit_error", "server_error", "service_error", "transient_error", "timeout_error", "conflict_error":
		return false
	}
	switch record.errorCode {
	case "model_not_found", "unauthorized", "forbidden", "rate_limit_exceeded", "server_error", "internal_server_error", "service_unavailable", "overloaded", "timeout", "request_timeout", "conflict":
		return false
	}
	return true
}

func logXSearchProbeRecord(t *testing.T, record xSearchProbeRecord) {
	t.Helper()
	t.Logf("case=%s option_classes=%v option_count=%d http_status=%d object=%s status=%s output_types=%v output_statuses=%v error_present=%t error_category=%s error_code=%s",
		record.caseLabel, record.optionClasses, record.optionCount, record.httpStatus, record.objectClass, record.statusClass,
		record.outputTypes, record.outputStatuses, record.errorPresent, record.errorCategory, record.errorCode)
}

func safeObjectClass(value string) string {
	if value == "response" {
		return "response"
	}
	if value == "" {
		return "absent"
	}
	return "other"
}

func safeStatusClass(value string) string {
	switch value {
	case "completed", "in_progress", "incomplete", "failed":
		return value
	case "":
		return "absent"
	default:
		return "other"
	}
}

func safeOutputType(value string) string {
	switch value {
	case "x_search_call", "message", "reasoning":
		return value
	case "":
		return "absent"
	default:
		return "other"
	}
}

func classifyXSearchError(raw json.RawMessage) (category string, code string, present bool) {
	category, code = "absent", "absent"
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) || trimmed[0] != '{' {
		return category, code, false
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &fields); err != nil {
		return category, code, false
	}
	return safeErrorJSONClass(fields["type"], safeErrorCategory), safeErrorJSONClass(fields["code"], safeErrorCode), true
}

func safeErrorJSONClass(raw json.RawMessage, classify func(string) string) string {
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return "absent"
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "other"
	}
	return classify(value)
}

func safeErrorCategory(value string) string {
	switch value {
	case "invalid_request_error", "authentication_error", "permission_error", "rate_limit_error", "server_error", "service_error", "transient_error", "timeout_error", "conflict_error":
		return value
	case "":
		return "absent"
	default:
		return "other"
	}
}

func safeErrorCode(value string) string {
	switch value {
	case "invalid_request", "invalid_tool", "invalid_argument", "model_not_found", "unauthorized", "forbidden", "rate_limit_exceeded", "server_error", "internal_server_error", "service_unavailable", "overloaded", "timeout", "request_timeout", "conflict":
		return value
	case "":
		return "absent"
	default:
		return "other"
	}
}

func uniqueSorted(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		seen[value] = struct{}{}
	}
	result := make([]string, 0, len(seen))
	for value := range seen {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func mapLookup(values map[string]string) func(string) (string, bool) {
	return func(name string) (string, bool) {
		value, present := values[name]
		return value, present
	}
}

func cloneStringMap(values map[string]string) map[string]string {
	clone := make(map[string]string, len(values))
	for key, value := range values {
		clone[key] = value
	}
	return clone
}
