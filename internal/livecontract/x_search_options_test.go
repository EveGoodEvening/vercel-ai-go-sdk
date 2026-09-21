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
	xSearchOptionsEndpoint = "https://ai-gateway.vercel.sh/v1/responses"
	xSearchOptionsModel    = "spacexai/grok-4.6"
	xSearchOptionsAckEnv   = "AI_GATEWAY_X_SEARCH_LIVE_COST_ACK"
	xSearchOptionsAck      = "I_ACCEPT_LIVE_X_SEARCH_COSTS"
	xSearchOptionsBodyMax  = 1 << 20
	xSearchOptionsMaxCalls = 14
)

var xSearchGeneralAckEnvs = [...]string{"AI_GATEWAY_LIVE_COST_ACK", "AI_GATEWAY_PUBLIC_LIVE_COST_ACK"}

type xSearchCredential struct {
	value string
}

type xSearchProbeCase struct {
	label         string
	options       map[string]any
	wantSuccess   bool
	optionClasses []string
}

type xSearchProbeRecord struct {
	caseLabel      string
	optionClasses  []string
	optionCount    int
	httpStatus     int
	objectClass    string
	statusClass    string
	outputTypes    []string
	outputStatuses []string
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
	Error *struct {
		Type string `json:"type"`
		Code string `json:"code"`
	} `json:"error"`
}

type countingRoundTripper struct {
	calls atomic.Int64
}

func (transport *countingRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	transport.calls.Add(1)
	return nil, errors.New("unexpected dispatch")
}

func TestGatewayXSearchOptionsContract(t *testing.T) {
	credential, err := resolveXSearchOptionsPrerequisites(os.LookupEnv)
	if err != nil {
		t.Fatal(err)
	}

	client := &http.Client{
		Transport: http.DefaultTransport,
		Timeout:   45 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	baseCases := []xSearchProbeCase{
		{label: "fieldless-control", wantSuccess: true},
		{label: "canonical-all-six", options: canonicalXSearchOptions(), wantSuccess: true, optionClasses: []string{"boolean", "date", "handle-list"}},
		{label: "explicit-empty-and-false", options: emptyXSearchOptions(), wantSuccess: true, optionClasses: []string{"boolean", "handle-list"}},
		{label: "from-only", options: map[string]any{"from_date": privateFromDate()}, wantSuccess: true, optionClasses: []string{"date"}},
		{label: "to-only", options: map[string]any{"to_date": privateToDate()}, wantSuccess: true, optionClasses: []string{"date"}},
		{label: "wrong-handle-list-family", options: map[string]any{"allowed_x_handles": true, "excluded_x_handles": true}, optionClasses: []string{"wrong-handle-list"}},
		{label: "wrong-date-family", options: map[string]any{"from_date": false, "to_date": false}, optionClasses: []string{"wrong-date"}},
		{label: "wrong-boolean-family", options: map[string]any{"enable_image_understanding": privateHandleList(), "enable_video_understanding": privateHandleList()}, optionClasses: []string{"wrong-boolean"}},
	}
	if len(baseCases)+len(individualXSearchOptionDiagnostics()) > xSearchOptionsMaxCalls {
		t.Fatal("x_search options probe matrix exceeds its hard request cap")
	}

	calls := 0
	canonicalSucceeded := false
	failures := make([]string, 0)
	for _, probeCase := range baseCases {
		if calls >= xSearchOptionsMaxCalls {
			t.Fatal("x_search options probe reached its hard request cap")
		}
		record, probeErr := runXSearchOptionsProbe(context.Background(), client, credential, probeCase)
		calls++
		if probeErr != nil {
			failures = append(failures, probeCase.label+": "+probeErr.Error())
			continue
		}
		logXSearchProbeRecord(t, record)
		succeeded := record.httpStatus >= 200 && record.httpStatus < 300
		if probeCase.label == "canonical-all-six" {
			canonicalSucceeded = succeeded
		}
		if succeeded != probeCase.wantSuccess {
			failures = append(failures, probeCase.label+": unexpected HTTP status class")
		}
	}

	if !canonicalSucceeded {
		for _, probeCase := range individualXSearchOptionDiagnostics() {
			if calls >= xSearchOptionsMaxCalls {
				t.Fatal("x_search options probe reached its hard request cap")
			}
			record, probeErr := runXSearchOptionsProbe(context.Background(), client, credential, probeCase)
			calls++
			if probeErr != nil {
				failures = append(failures, probeCase.label+": "+probeErr.Error())
				continue
			}
			logXSearchProbeRecord(t, record)
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

func resolveXSearchOptionsPrerequisites(lookup func(string) (string, bool)) (xSearchCredential, error) {
	ack, present := lookup(xSearchOptionsAckEnv)
	if !present || ack != xSearchOptionsAck {
		return xSearchCredential{}, fmt.Errorf("live Gateway x_search options contract requires %s=%s exactly", xSearchOptionsAckEnv, xSearchOptionsAck)
	}
	for _, name := range xSearchGeneralAckEnvs {
		if _, present := lookup(name); present {
			return xSearchCredential{}, fmt.Errorf("live Gateway x_search options contract requires general acknowledgement %s to be absent", name)
		}
	}
	apiKey, _ := lookup("AI_GATEWAY_API_KEY")
	oidcToken, _ := lookup("VERCEL_OIDC_TOKEN")
	hasAPIKey := strings.TrimSpace(apiKey) != ""
	hasOIDC := strings.TrimSpace(oidcToken) != ""
	if hasAPIKey == hasOIDC {
		return xSearchCredential{}, errors.New("live Gateway x_search options contract requires exactly one nonblank credential")
	}
	if hasAPIKey {
		return xSearchCredential{value: apiKey}, nil
	}
	return xSearchCredential{value: oidcToken}, nil
}

func executeXSearchOptionsWithPrerequisites(ctx context.Context, lookup func(string) (string, bool), newClient func() *http.Client, probeCase xSearchProbeCase) error {
	credential, err := resolveXSearchOptionsPrerequisites(lookup)
	if err != nil {
		return err
	}
	_, err = runXSearchOptionsProbe(ctx, newClient(), credential, probeCase)
	return err
}

func runXSearchOptionsProbe(ctx context.Context, client *http.Client, credential xSearchCredential, probeCase xSearchProbeCase) (xSearchProbeRecord, error) {
	tool := map[string]any{"type": "x_search"}
	for key, value := range probeCase.options {
		tool[key] = value
	}
	payload, err := json.Marshal(map[string]any{
		"model":       xSearchOptionsModel,
		"input":       privateProbeInput(),
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
	req.Header.Set("Authorization", "Bearer "+credential.value)
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
		caseLabel:     probeCase.label,
		optionClasses: append([]string(nil), probeCase.optionClasses...),
		optionCount:   len(probeCase.options),
		httpStatus:    resp.StatusCode,
		objectClass:   safeObjectClass(envelope.Object),
		statusClass:   safeStatusClass(envelope.Status),
	}
	for _, item := range envelope.Output {
		record.outputTypes = append(record.outputTypes, safeOutputType(item.Type))
		record.outputStatuses = append(record.outputStatuses, safeStatusClass(item.Status))
	}
	record.outputTypes = uniqueSorted(record.outputTypes)
	record.outputStatuses = uniqueSorted(record.outputStatuses)
	if envelope.Error != nil {
		record.errorCategory = safeErrorCategory(envelope.Error.Type)
		record.errorCode = safeErrorCode(envelope.Error.Code)
	}
	return record, nil
}

func canonicalXSearchOptions() map[string]any {
	return map[string]any{
		"allowed_x_handles":          privateHandleList(),
		"excluded_x_handles":         privateOtherHandleList(),
		"from_date":                  privateFromDate(),
		"to_date":                    privateToDate(),
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

func individualXSearchOptionDiagnostics() []xSearchProbeCase {
	canonical := canonicalXSearchOptions()
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

func logXSearchProbeRecord(t *testing.T, record xSearchProbeRecord) {
	t.Helper()
	t.Logf("case=%s option_classes=%v option_count=%d http_status=%d object=%s status=%s output_types=%v output_statuses=%v error_category=%s error_code=%s",
		record.caseLabel, record.optionClasses, record.optionCount, record.httpStatus, record.objectClass, record.statusClass,
		record.outputTypes, record.outputStatuses, record.errorCategory, record.errorCode)
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

func safeErrorCategory(value string) string {
	switch value {
	case "invalid_request_error", "authentication_error", "permission_error", "rate_limit_error", "server_error":
		return value
	case "":
		return "absent"
	default:
		return "other"
	}
}

func safeErrorCode(value string) string {
	switch value {
	case "invalid_request", "invalid_tool", "invalid_argument", "model_not_found", "unauthorized", "forbidden", "rate_limit_exceeded":
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

func privateProbeInput() string {
	return "Find one recent public post relevant to orbital launch operations and answer briefly."
}
func privateHandleList() []string      { return []string{"SpaceX"} }
func privateOtherHandleList() []string { return []string{"xai"} }
func privateFromDate() string          { return "2026-01-01" }
func privateToDate() string            { return "2026-09-20" }
