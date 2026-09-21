//go:build livecontract

package livecontract_test

import (
	"context"
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
	if len(result.RawJSON()) == 0 {
		t.Fatal("live Gateway response was empty")
	}

	stream, err := client.StreamResponse(ctx, request)
	if err != nil {
		t.Fatalf("start live Gateway response stream: %v", err)
	}
	defer stream.Close()
	events := 0
	for stream.Next() {
		events++
	}
	if err := stream.Err(); err != nil {
		t.Fatalf("read live Gateway response stream: %v", err)
	}
	if events == 0 {
		t.Fatal("live Gateway response stream contained no events")
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
	if len(result.RawJSON()) == 0 {
		t.Fatal("live Gateway chat completion was empty")
	}

	stream, err := client.StreamChatCompletion(ctx, request)
	if err != nil {
		t.Fatalf("start live Gateway chat completion stream: %v", err)
	}
	defer stream.Close()
	chunks := 0
	for stream.Next() {
		chunks++
	}
	if err := stream.Err(); err != nil {
		t.Fatalf("read live Gateway chat completion stream: %v", err)
	}
	if chunks == 0 {
		t.Fatal("live Gateway chat completion stream contained no chunks")
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
