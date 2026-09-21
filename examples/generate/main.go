// Command generate demonstrates credentialed buffered and streaming requests to
// the Responses and Chat Completions surfaces.
//
// Set AI_GATEWAY_API_KEY or VERCEL_OIDC_TOKEN before running it. The command
// may incur provider charges. It deliberately avoids printing prompts, model
// output, raw bodies, tool arguments or results, headers, credentials, or
// secrets.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	gateway "github.com/EveGoodEvening/vercel-ai-go-sdk"
)

const prompt = "Reply with a short greeting."

func main() {
	surface := flag.String("surface", "", "required: responses or chat")
	mode := flag.String("mode", "", "required: buffered or stream")
	model := flag.String("model", "", "required provider/model identifier")
	flag.Parse()

	if flag.NArg() != 0 || *model == "" || (*surface != "responses" && *surface != "chat") || (*mode != "buffered" && *mode != "stream") {
		flag.Usage()
		os.Exit(2)
	}

	client, err := gateway.NewClient()
	if err != nil {
		printError(err)
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	switch {
	case *surface == "responses" && *mode == "buffered":
		err = runBufferedResponse(ctx, client, *model)
	case *surface == "responses" && *mode == "stream":
		err = runResponseStream(ctx, client, *model)
	case *surface == "chat" && *mode == "buffered":
		err = runBufferedChat(ctx, client, *model)
	case *surface == "chat" && *mode == "stream":
		err = runChatStream(ctx, client, *model)
	}
	if err != nil {
		printError(err)
		os.Exit(1)
	}
}

func runBufferedResponse(ctx context.Context, client *gateway.Client, model string) error {
	_, err := client.CreateResponse(ctx, gateway.ResponsesRequest{
		Model: model,
		Input: gateway.ResponseTextInput(prompt),
	})
	if err == nil {
		fmt.Println("responses buffered request completed")
	}
	return err
}

func runResponseStream(ctx context.Context, client *gateway.Client, model string) error {
	stream, err := client.StreamResponse(ctx, gateway.ResponsesRequest{
		Model: model,
		Input: gateway.ResponseTextInput(prompt),
	})
	if err != nil {
		return err
	}

	events, textBytes := 0, 0
	for stream.Next() {
		events++
		if event, ok := stream.Event().(gateway.ResponseOutputTextDeltaEvent); ok {
			textBytes += len(event.Delta)
		}
	}
	streamErr := stream.Err()
	closeErr := stream.Close()
	if streamErr != nil {
		return streamErr
	}
	if closeErr != nil {
		return closeErr
	}
	fmt.Printf("responses stream completed: events=%d text_bytes=%d\n", events, textBytes)
	return nil
}

func runBufferedChat(ctx context.Context, client *gateway.Client, model string) error {
	result, err := client.CreateChatCompletion(ctx, gateway.ChatCompletionRequest{
		Model: model,
		Messages: []gateway.ChatMessage{{
			Role:    "user",
			Content: gateway.ChatTextContent(prompt),
		}},
	})
	if err != nil {
		return err
	}
	choiceCount := 0
	if result.Choices.Present && !result.Choices.Null {
		choiceCount = len(result.Choices.Value)
	}
	fmt.Printf("chat buffered request completed: choices=%d usage_present=%t\n", choiceCount, result.Usage.Present)
	return nil
}

func runChatStream(ctx context.Context, client *gateway.Client, model string) error {
	stream, err := client.StreamChatCompletion(ctx, gateway.ChatCompletionRequest{
		Model: model,
		Messages: []gateway.ChatMessage{{
			Role:    "user",
			Content: gateway.ChatTextContent(prompt),
		}},
	})
	if err != nil {
		return err
	}

	chunks, textBytes := 0, 0
	for stream.Next() {
		chunks++
		for _, choice := range stream.Event().Choices {
			textBytes += len(choice.Delta.Content)
		}
	}
	streamErr := stream.Err()
	closeErr := stream.Close()
	if streamErr != nil {
		return streamErr
	}
	if closeErr != nil {
		return closeErr
	}
	fmt.Printf("chat stream completed: chunks=%d text_bytes=%d\n", chunks, textBytes)
	return nil
}

func printError(err error) {
	var configurationErr *gateway.ConfigurationError
	var validationErr *gateway.ValidationError
	var transportErr *gateway.TransportError
	var responseErr *gateway.ResponseError
	var responseValidationErr *gateway.ResponseValidationError

	switch {
	case errors.As(err, &configurationErr):
		log.Printf("configuration error")
	case errors.As(err, &validationErr):
		log.Printf("validation error: path=%q", validationErr.Path())
	case errors.As(err, &responseValidationErr):
		log.Printf("response validation error: status=%d path=%q request_id_present=%t response_id_present=%t truncated=%t",
			responseValidationErr.StatusCode(), responseValidationErr.Path(), responseValidationErr.RequestID() != "",
			responseValidationErr.ResponseID() != "", responseValidationErr.BodyTruncated())
	case errors.As(err, &responseErr):
		_, hasRetryAfter := responseErr.RetryAfter()
		log.Printf("response error: status=%d request_id_present=%t response_id_present=%t generation_id_present=%t retryable=%t retry_after_present=%t truncated=%t",
			responseErr.StatusCode(), responseErr.RequestID() != "", responseErr.ResponseID() != "",
			responseErr.GenerationID() != "", responseErr.Retryable(), hasRetryAfter, responseErr.BodyTruncated())
	case errors.As(err, &transportErr):
		log.Printf("transport error: operation=%q", transportErr.Operation())
	default:
		log.Printf("generation failed: %T", err)
	}
}
