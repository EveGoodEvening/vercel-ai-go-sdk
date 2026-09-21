#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd -P)
consumer_dir=$(mktemp -d "${TMPDIR:-/tmp}/gateway-local-consumer.XXXXXX")
trap 'rm -rf -- "$consumer_dir"' EXIT

cat >"$consumer_dir/go.mod" <<EOF
module example.com/gateway-local-consumer

go 1.26

require github.com/EveGoodEvening/vercel-ai-go-sdk v0.0.0

replace github.com/EveGoodEvening/vercel-ai-go-sdk => $repo_root
EOF

cat >"$consumer_dir/main.go" <<'EOF'
package main

import (
	"context"
	"errors"
	"net/http"
	"time"

	gateway "github.com/EveGoodEvening/vercel-ai-go-sdk"
)

type tokenSource struct{}

func (tokenSource) Token(context.Context) (string, error) { return "token", nil }

func compileEvaluation(client *gateway.Client) (*gateway.EvaluationResult, error) {
	zero := 0
	zero64 := int64(0)
	details := "details"
	request := gateway.EvaluationRequest{
		State: map[string]any{"candidate": "answer"},
		Questions: map[string]gateway.Question{
			"boolean": gateway.BooleanQuestion{Instructions: "judge", Criteria: &gateway.BooleanCriteria{True: gateway.OptionalJSON{Set: true, Value: "yes"}, False: gateway.OptionalJSON{Set: true, Value: "no"}}},
			"choice":  gateway.ChoiceQuestion{Instructions: "choose", Criteria: map[string]any{"a": "A"}},
			"score":   gateway.ScoreQuestion{Instructions: "score", Criteria: []any{"low", "high"}},
		},
		ProviderOptions: map[string]map[string]any{"provider": {"flag": true}},
	}
	_ = gateway.RetryPolicy{MaxAttempts: 2, InitialDelay: time.Millisecond, MaxDelay: time.Second, Multiplier: 2, Jitter: 0}
	_ = gateway.EvaluationResult{
		Answers: map[string]gateway.Answer{
			"boolean": gateway.BooleanAnswer{Probability: 1},
			"choice":  gateway.ChoiceAnswer{Choice: "a", Probabilities: map[string]float64{"a": 1}},
			"score":   gateway.ScoreAnswer{Score: 1, Probabilities: map[string]float64{"1": 1}},
		},
		Rounding: &gateway.Rounding{ProbabilityDecimals: &zero, ScoreDecimals: &zero},
		Usage: &gateway.Usage{InputTokens: &zero64, OutputTokens: &zero64},
		Warnings: []gateway.Warning{{Type: gateway.WarningUnsupported, Feature: "feature", Details: &details}},
		ProviderMetadata: map[string]map[string]any{"provider": {"key": "value"}},
		Response: gateway.ResponseMetadata{ModelID: "provider/model", Headers: http.Header{}, Body: []byte{}},
	}
	_ = []gateway.WarningType{gateway.WarningUnsupported, gateway.WarningCompatibility, gateway.WarningDeprecated, gateway.WarningOther}
	return client.Evaluate(context.Background(), "provider/model", request)
}

func compileResponses(client *gateway.Client) {
	description, strict, enabled := "description", true, true
	request := gateway.ResponsesRequest{
		Model: "provider/model",
		Input: gateway.ResponseItemsInput{
			gateway.ResponseMessage{Role: "user", Content: "hello"},
			gateway.ResponseFunctionCall{ID: "item", CallID: "call", Name: "lookup", Arguments: `{}`},
			gateway.ResponseFunctionCallOutput{CallID: "call", Output: `{}`},
		},
		Tools: []gateway.ResponseTool{{Name: "lookup", Description: &description, Parameters: map[string]any{"type": "object"}, Strict: &strict}},
		ToolChoice: gateway.ResponseSpecificToolChoice{Name: "lookup"},
		ParallelToolCalls: &enabled,
		Reasoning: &gateway.ResponseReasoning{Effort: "low", Summary: &description},
		Text: &gateway.ResponseText{Format: gateway.ResponseJSONSchemaFormat{Name: "answer", Description: &description, Schema: map[string]any{"type": "object"}, Strict: &strict}},
	}
	_ = gateway.ResponsesRequest{Model: "provider/model", Input: gateway.ResponseTextInput("hello"), ToolChoice: gateway.ResponseToolChoiceAuto, Text: &gateway.ResponseText{Format: gateway.ResponseTextFormatText}}
	_ = []gateway.ResponseToolChoiceMode{gateway.ResponseToolChoiceAuto, gateway.ResponseToolChoiceRequired, gateway.ResponseToolChoiceNone}
	_ = []gateway.ResponseTextFormatType{gateway.ResponseTextFormatText, gateway.ResponseTextFormatJSONObject}
	var input gateway.ResponseInput = gateway.ResponseItemsInput{}
	var item gateway.ResponseInputItem = gateway.ResponseMessage{}
	var choice gateway.ResponseToolChoice = gateway.ResponseToolChoiceNone
	var format gateway.ResponseTextFormat = gateway.ResponseTextFormatJSONObject
	_, _, _, _ = input, item, choice, format
	result, _ := client.CreateResponse(context.Background(), request)
	_ = result.RawJSON()
	stream, _ := client.StreamResponse(context.Background(), request)
	if stream != nil {
		_, _, _, _ = stream.Next(), stream.Event(), stream.Err(), stream.Close()
	}
	var event gateway.ResponseEvent = gateway.ResponseOutputTextDeltaEvent{Type: "response.output_text.delta", Event: "event", ID: "id", Delta: "text"}
	event = gateway.RawResponseEvent{Type: "other", Event: "event", ID: "id"}
	_ = event.(gateway.RawResponseEvent).RawJSON()
}

func compileChat(client *gateway.Client) {
	description, detail, strict := "description", "auto", true
	request := gateway.ChatCompletionRequest{
		Model: "provider/model",
		Messages: []gateway.ChatMessage{
			{Role: "system", Content: gateway.ChatTextContent("help")},
			{Role: "user", Content: gateway.ChatPartsContent{gateway.ChatTextPart{Text: "hello"}, gateway.ChatImageURLPart{URL: "https://example.com/image.png", Detail: &detail}, gateway.ChatFilePart{Data: "ZGF0YQ==", MediaType: "text/plain", Filename: "input.txt"}}},
		},
		Stop: gateway.ChatStopStrings{"stop"},
		Tools: []gateway.ChatTool{{Name: "lookup", Description: &description, Parameters: map[string]any{"type": "object"}}},
		ToolChoice: gateway.ChatSpecificToolChoice{Name: "lookup"},
		ResponseFormat: gateway.ChatJSONSchemaResponseFormat{Name: "answer", Description: &description, Schema: map[string]any{"type": "object"}, Strict: &strict},
		ProviderOptions: &gateway.ChatProviderOptions{Gateway: gateway.ChatGatewayOptions{Order: []string{"provider"}, Models: []string{"provider/model"}, ProviderTimeouts: &gateway.ChatProviderTimeouts{BYOK: map[string]int{"provider": 1000}}}},
		Provider: &gateway.ChatProvider{Sort: "price"},
	}
	_ = gateway.ChatCompletionRequest{Model: "provider/model", Messages: []gateway.ChatMessage{{Role: "user", Content: gateway.ChatTextContent("hello")}}, Stop: gateway.ChatStopString("stop"), ToolChoice: gateway.ChatToolChoiceAuto, ResponseFormat: gateway.ChatResponseFormatText}
	_ = []gateway.ChatToolChoiceMode{gateway.ChatToolChoiceAuto, gateway.ChatToolChoiceNone}
	_ = []gateway.ChatResponseFormatType{gateway.ChatResponseFormatText, gateway.ChatResponseFormatJSON}
	_ = gateway.ChatLegacyJSONResponseFormat{Schema: map[string]any{"type": "object"}, Name: &description, Description: &description}
	var content gateway.ChatMessageContent = gateway.ChatTextContent("hello")
	var part gateway.ChatContentPart = gateway.ChatTextPart{Text: "hello"}
	var stop gateway.ChatStop = gateway.ChatStopString("stop")
	var choice gateway.ChatToolChoice = gateway.ChatToolChoiceNone
	var format gateway.ChatResponseFormat = gateway.ChatResponseFormatJSON
	_, _, _, _, _ = content, part, stop, choice, format
	result, _ := client.CreateChatCompletion(context.Background(), request)
	if result != nil {
		_ = result.RawJSON()
	}
	_ = gateway.ChatCompletionResult{ID: gateway.JSONField[string]{Present: true, Value: "id"}, Object: gateway.JSONField[string]{}, Created: gateway.JSONField[int64]{}, Model: gateway.JSONField[string]{}, Choices: gateway.JSONField[[]gateway.ChatChoice]{Value: []gateway.ChatChoice{{Index: gateway.JSONField[int]{}, Message: gateway.JSONField[gateway.ChatAssistantMessage]{Value: gateway.ChatAssistantMessage{Role: gateway.JSONField[string]{}, Content: gateway.JSONField[string]{}, ToolCalls: gateway.JSONField[[]gateway.ChatToolCall]{Value: []gateway.ChatToolCall{{ID: gateway.JSONField[string]{}, Type: gateway.JSONField[string]{}, Function: gateway.JSONField[gateway.ChatFunctionCall]{Value: gateway.ChatFunctionCall{Name: gateway.JSONField[string]{}, Arguments: gateway.JSONField[string]{}}}}}}}}, FinishReason: gateway.JSONField[string]{}}}}, Usage: gateway.JSONField[gateway.ChatUsage]{Value: gateway.ChatUsage{PromptTokens: gateway.JSONField[int64]{}, CompletionTokens: gateway.JSONField[int64]{}, TotalTokens: gateway.JSONField[int64]{}}}}
	stream, _ := client.StreamChatCompletion(context.Background(), request)
	if stream != nil {
		_, _, _, _ = stream.Next(), stream.Event(), stream.Err(), stream.Close()
	}
	chunk := gateway.ChatCompletionChunk{Object: "chat.completion.chunk", Choices: []gateway.ChatCompletionChunkChoice{{Delta: gateway.ChatCompletionChunkDelta{Content: "text"}}}}
	_ = chunk.RawJSON()
}

func inspectErrors(err error) {
	var configuration *gateway.ConfigurationError
	if errors.As(err, &configuration) {
		_, _, _ = configuration.Error(), configuration.Option(), configuration.Reason()
	}
	var validation *gateway.ValidationError
	if errors.As(err, &validation) {
		_, _, _ = validation.Error(), validation.Path(), validation.Reason()
	}
	var transport *gateway.TransportError
	if errors.As(err, &transport) {
		_, _, _ = transport.Error(), transport.Operation(), transport.Unwrap()
	}
	var response *gateway.ResponseError
	if errors.As(err, &response) {
		_, _, _, _ = response.Error(), response.StatusCode(), response.Message(), response.Type()
		_, _, _, _ = response.Code(), response.Param(), response.GenerationID(), response.RequestID()
		_ = response.ResponseID()
		_, _ = response.RetryAfter()
		_, _, _, _ = response.Retryable(), response.BodyTruncated(), response.RawResponseBody(), response.Unwrap()
	}
	var responseValidation *gateway.ResponseValidationError
	if errors.As(err, &responseValidation) {
		_, _, _, _ = responseValidation.Error(), responseValidation.StatusCode(), responseValidation.Path(), responseValidation.Reason()
		_, _, _, _ = responseValidation.RequestID(), responseValidation.ResponseID(), responseValidation.BodyTruncated(), responseValidation.RawResponseBody()
		_ = responseValidation.Unwrap()
	}
}

func main() {
	client, err := gateway.NewClient(
		gateway.WithAPIKey("key"),
		gateway.WithOIDCToken("token"),
		gateway.WithOIDCTokenSource(tokenSource{}),
		gateway.WithBaseURL("http://127.0.0.1"),
		gateway.WithPublicBaseURL("http://127.0.0.1"),
		gateway.WithHTTPClient(&http.Client{}),
		gateway.WithTeam("team"),
		gateway.WithHeaders(http.Header{"X-Consumer": {"value"}}),
		gateway.WithRetryPolicy(gateway.RetryPolicy{}),
	)
	inspectErrors(err)
	if client != nil {
		_, _, _ = compileEvaluation, compileResponses, compileChat
	}
}
EOF

cat >"$consumer_dir/check_api.go" <<'EOF'
//go:build ignore

package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

var allowedTypes = names(
	"Client", "Option", "TokenSource", "RetryPolicy",
	"ConfigurationError", "ValidationError", "TransportError", "ResponseError", "ResponseValidationError",
	"EvaluationRequest", "Question", "BooleanQuestion", "ChoiceQuestion", "ScoreQuestion", "OptionalJSON", "BooleanCriteria", "EvaluationResult", "Rounding", "Usage", "WarningType", "Warning", "ResponseMetadata", "Answer", "BooleanAnswer", "ChoiceAnswer", "ScoreAnswer",
	"ResponsesRequest", "ResponseInput", "ResponseTextInput", "ResponseItemsInput", "ResponseInputItem", "ResponseMessage", "ResponseFunctionCall", "ResponseFunctionCallOutput", "ResponseTool", "ResponseToolChoice", "ResponseToolChoiceMode", "ResponseSpecificToolChoice", "ResponseReasoning", "ResponseText", "ResponseTextFormat", "ResponseTextFormatType", "ResponseJSONSchemaFormat", "ResponseResult", "ResponseEvent", "ResponseOutputTextDeltaEvent", "RawResponseEvent", "ResponseStream",
	"ChatCompletionRequest", "ChatMessage", "ChatMessageContent", "ChatTextContent", "ChatPartsContent", "ChatContentPart", "ChatTextPart", "ChatImageURLPart", "ChatFilePart", "ChatStop", "ChatStopString", "ChatStopStrings", "ChatTool", "ChatToolChoice", "ChatToolChoiceMode", "ChatSpecificToolChoice", "ChatResponseFormat", "ChatResponseFormatType", "ChatJSONSchemaResponseFormat", "ChatLegacyJSONResponseFormat", "ChatProviderOptions", "ChatGatewayOptions", "ChatProviderTimeouts", "ChatProvider", "JSONField", "ChatCompletionResult", "ChatChoice", "ChatAssistantMessage", "ChatToolCall", "ChatFunctionCall", "ChatUsage", "ChatCompletionChunk", "ChatCompletionChunkChoice", "ChatCompletionChunkDelta", "ChatCompletionStream",
)

var allowedFunctions = names("NewClient", "WithAPIKey", "WithOIDCToken", "WithOIDCTokenSource", "WithBaseURL", "WithPublicBaseURL", "WithHTTPClient", "WithTeam", "WithHeaders", "WithRetryPolicy")
var allowedMethods = names(
	"Client.Evaluate", "Client.CreateResponse", "Client.StreamResponse", "Client.CreateChatCompletion", "Client.StreamChatCompletion",
	"ResponseResult.RawJSON", "RawResponseEvent.RawJSON", "ResponseStream.Next", "ResponseStream.Event", "ResponseStream.Err", "ResponseStream.Close",
	"ChatCompletionResult.RawJSON", "ChatCompletionChunk.RawJSON", "ChatCompletionStream.Next", "ChatCompletionStream.Event", "ChatCompletionStream.Err", "ChatCompletionStream.Close",
	"ConfigurationError.Error", "ConfigurationError.Option", "ConfigurationError.Reason",
	"ValidationError.Error", "ValidationError.Path", "ValidationError.Reason",
	"TransportError.Error", "TransportError.Operation", "TransportError.Unwrap",
	"ResponseError.Error", "ResponseError.Unwrap", "ResponseError.StatusCode", "ResponseError.Message", "ResponseError.Type", "ResponseError.Code", "ResponseError.Param", "ResponseError.GenerationID", "ResponseError.RequestID", "ResponseError.ResponseID", "ResponseError.RetryAfter", "ResponseError.Retryable", "ResponseError.BodyTruncated", "ResponseError.RawResponseBody",
	"ResponseValidationError.Error", "ResponseValidationError.Unwrap", "ResponseValidationError.StatusCode", "ResponseValidationError.Path", "ResponseValidationError.Reason", "ResponseValidationError.RequestID", "ResponseValidationError.ResponseID", "ResponseValidationError.BodyTruncated", "ResponseValidationError.RawResponseBody",
)
var allowedValues = names("WarningUnsupported", "WarningCompatibility", "WarningDeprecated", "WarningOther", "ResponseToolChoiceAuto", "ResponseToolChoiceRequired", "ResponseToolChoiceNone", "ResponseTextFormatText", "ResponseTextFormatJSONObject", "ChatToolChoiceAuto", "ChatToolChoiceNone", "ChatResponseFormatText", "ChatResponseFormatJSON")

func names(values ...string) map[string]bool {
	out := make(map[string]bool, len(values))
	for _, value := range values {
		out[value] = true
	}
	return out
}

func requireAllowed(kind, name string, allowed map[string]bool) {
	if !allowed[name] {
		fmt.Fprintf(os.Stderr, "unexpected exported %s found: %s\n", kind, name)
		os.Exit(1)
	}
}

func exportedReceiver(receiver *ast.FieldList) bool {
	if receiver == nil || len(receiver.List) != 1 {
		return false
	}
	return ast.IsExported(receiverName(receiver.List[0].Type))
}

func receiverName(expression ast.Expr) string {
	switch typed := expression.(type) {
	case *ast.Ident:
		return typed.Name
	case *ast.StarExpr:
		return receiverName(typed.X)
	case *ast.IndexExpr:
		return receiverName(typed.X)
	case *ast.IndexListExpr:
		return receiverName(typed.X)
	default:
		return ""
	}
}

func main() {
	entries, err := os.ReadDir(os.Args[1])
	if err != nil {
		panic(err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(os.Args[1], entry.Name()), nil, 0)
		if err != nil {
			panic(err)
		}
		for _, declaration := range file.Decls {
			switch typed := declaration.(type) {
			case *ast.GenDecl:
				for _, specification := range typed.Specs {
					switch spec := specification.(type) {
					case *ast.TypeSpec:
						if !ast.IsExported(spec.Name.Name) {
							continue
						}
						if spec.Assign.IsValid() {
							fmt.Fprintf(os.Stderr, "exported compatibility alias found: %s\n", spec.Name.Name)
							os.Exit(1)
						}
						requireAllowed("type", spec.Name.Name, allowedTypes)
					case *ast.ValueSpec:
						for _, name := range spec.Names {
							if ast.IsExported(name.Name) {
								requireAllowed("value", name.Name, allowedValues)
							}
						}
					}
				}
			case *ast.FuncDecl:
				if !ast.IsExported(typed.Name.Name) {
					continue
				}
				if typed.Recv == nil {
					requireAllowed("function", typed.Name.Name, allowedFunctions)
				} else if exportedReceiver(typed.Recv) {
					receiver := receiverName(typed.Recv.List[0].Type)
					requireAllowed("method", receiver+"."+typed.Name.Name, allowedMethods)
				}
			}
		}
	}
}
EOF

env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK GOWORK=off GOPROXY=off go run "$consumer_dir/check_api.go" "$repo_root"

cd "$consumer_dir"
env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK GOWORK=off GOPROXY=off go build ./...
