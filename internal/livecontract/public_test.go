//go:build livecontract

package livecontract_test

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	gateway "github.com/EveGoodEvening/vercel-ai-go-sdk"
)

const (
	publicLiveModel   = "openai/gpt-5-nano"
	publicLiveCostAck = "I_ACCEPT_LIVE_PUBLIC_API_COSTS"
)

func TestGatewayResponsesContract(t *testing.T) {
	client := requirePublicLiveClient(t)
	ctx := context.Background()
	maxOutputTokens := 16
	request := gateway.ResponsesRequest{
		Model:           publicLiveModel,
		Input:           gateway.ResponseTextInput("Reply with OK."),
		MaxOutputTokens: &maxOutputTokens,
	}

	result, err := client.CreateResponse(ctx, request)
	if err != nil {
		t.Fatalf("create live Gateway response: %v", err)
	}
	assertBufferedResponseStructure(t, result.RawJSON())

	stream, err := client.StreamResponse(ctx, request)
	if err != nil {
		t.Fatalf("start live Gateway response stream: %v", err)
	}
	defer stream.Close()
	events := 0
	lastType := ""
	for stream.Next() {
		events++
		event := stream.Event()
		switch event := event.(type) {
		case gateway.ResponseOutputTextDeltaEvent:
			lastType = event.Type
		case gateway.RawResponseEvent:
			lastType = event.Type
			assertStreamResponseModel(t, event.RawJSON(), event.Type == "response.completed")
		}
	}
	if err := stream.Err(); err != nil {
		t.Fatalf("read live Gateway response stream: %v", err)
	}
	if events == 0 {
		t.Fatal("live Gateway response stream contained no events")
	}
	if lastType != "response.completed" {
		t.Fatal("live Gateway response stream did not terminate with response.completed")
	}
}

func TestGatewayChatContract(t *testing.T) {
	client := requirePublicLiveClient(t)
	ctx := context.Background()
	maxTokens := 16
	request := gateway.ChatCompletionRequest{
		Model: publicLiveModel,
		Messages: []gateway.ChatMessage{{
			Role:    "user",
			Content: gateway.ChatTextContent("Reply with OK."),
		}},
		MaxTokens: &maxTokens,
	}

	result, err := client.CreateChatCompletion(ctx, request)
	if err != nil {
		t.Fatalf("create live Gateway chat completion: %v", err)
	}
	if !result.Model.Present || result.Model.Null || result.Model.Value != publicLiveModel {
		t.Fatal("live Gateway chat completion omitted or changed the pinned model")
	}
	if !result.Choices.Present || result.Choices.Null || len(result.Choices.Value) == 0 {
		t.Fatal("live Gateway chat completion omitted choices")
	}
	for _, choice := range result.Choices.Value {
		if !choice.FinishReason.Present || choice.FinishReason.Null || strings.TrimSpace(choice.FinishReason.Value) == "" {
			t.Fatal("live Gateway chat completion omitted a finish reason")
		}
	}

	stream, err := client.StreamChatCompletion(ctx, request)
	if err != nil {
		t.Fatalf("start live Gateway chat completion stream: %v", err)
	}
	defer stream.Close()
	chunks := 0
	for stream.Next() {
		chunks++
		var envelope struct {
			Model string `json:"model"`
		}
		if err := json.Unmarshal(stream.Event().RawJSON(), &envelope); err != nil {
			t.Fatalf("decode live Gateway chat stream structure: %v", err)
		}
		if envelope.Model != "" && envelope.Model != publicLiveModel {
			t.Fatal("live Gateway chat stream changed the pinned model")
		}
	}
	if err := stream.Err(); err != nil {
		t.Fatalf("read live Gateway chat completion stream: %v", err)
	}
	if chunks == 0 {
		t.Fatal("live Gateway chat completion stream contained no chunks")
	}
}

func assertBufferedResponseStructure(t *testing.T, raw []byte) {
	t.Helper()
	var envelope struct {
		Model  string `json:"model"`
		Output []struct {
			Type string `json:"type"`
		} `json:"output"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatalf("decode live Gateway response structure: %v", err)
	}
	if envelope.Model != publicLiveModel {
		t.Fatal("live Gateway response omitted or changed the pinned model")
	}
	if len(envelope.Output) == 0 {
		t.Fatal("live Gateway response omitted output items")
	}
	for _, item := range envelope.Output {
		if strings.TrimSpace(item.Type) == "" {
			t.Fatal("live Gateway response output item omitted its type")
		}
	}
}

func assertStreamResponseModel(t *testing.T, raw []byte, required bool) {
	t.Helper()
	var envelope struct {
		Model    *string `json:"model"`
		Response *struct {
			Model *string `json:"model"`
		} `json:"response"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatalf("decode live Gateway response stream structure: %v", err)
	}
	observed := envelope.Model
	if envelope.Response != nil && envelope.Response.Model != nil {
		observed = envelope.Response.Model
	}
	if observed != nil && *observed != publicLiveModel {
		t.Fatal("live Gateway response stream changed the pinned model")
	}
	if required && observed == nil {
		t.Fatal("live Gateway response.completed omitted the pinned model")
	}
}

func requirePublicLiveClient(t *testing.T) *gateway.Client {
	t.Helper()
	if os.Getenv("AI_GATEWAY_PUBLIC_LIVE_COST_ACK") != publicLiveCostAck {
		t.Fatalf("live Gateway public API contract requires AI_GATEWAY_PUBLIC_LIVE_COST_ACK=I_ACCEPT_LIVE_PUBLIC_API_COSTS exactly")
	}
	if _, present := os.LookupEnv("AI_GATEWAY_LIVE_COST_ACK"); present {
		t.Fatalf("live Gateway public API contract requires AI_GATEWAY_LIVE_COST_ACK to be absent")
	}

	apiKey := os.Getenv("AI_GATEWAY_API_KEY")
	oidcToken := os.Getenv("VERCEL_OIDC_TOKEN")
	hasAPIKey := strings.TrimSpace(apiKey) != ""
	hasOIDCToken := strings.TrimSpace(oidcToken) != ""
	if hasAPIKey == hasOIDCToken {
		t.Fatalf("live Gateway public API contract requires exactly one non-empty AI_GATEWAY_API_KEY or VERCEL_OIDC_TOKEN")
	}

	var credential gateway.Option
	if hasAPIKey {
		credential = gateway.WithAPIKey(apiKey)
	} else {
		credential = gateway.WithOIDCToken(oidcToken)
	}
	client, err := gateway.NewClient(credential)
	if err != nil {
		t.Fatalf("construct live Gateway public API client: %v", err)
	}
	return client
}
