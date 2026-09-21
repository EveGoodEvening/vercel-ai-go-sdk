package gateway

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"reflect"
	"sync/atomic"
	"testing"
)

func imageClient(t *testing.T, fn func(*http.Request) *http.Response) *Client {
	t.Helper()
	c, err := NewClient(WithAPIKey("key"), WithBaseURL("https://unit.test/v4/ai"), WithHTTPClient(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) { return fn(r), nil })}))
	if err != nil {
		t.Fatal(err)
	}
	return c
}
func TestGenerateImageExactWirePresenceAndResult(t *testing.T) {
	prompt := "draw"
	size := ImageSize("")
	aspect := ImageAspectRatio("16:9")
	seed := int64(0)
	var got []byte
	client := imageClient(t, func(r *http.Request) *http.Response {
		if r.URL.Path != "/v4/ai/image-model" || r.Header.Get(headerImageModelSpecificationVersion) != "4" {
			t.Fatalf("request %s headers=%v", r.URL.Path, r.Header)
		}
		got, _ = io.ReadAll(r.Body)
		return &http.Response{StatusCode: 200, Header: http.Header{"X-Test": {"one"}}, Body: io.NopCloser(bytes.NewBufferString(`{"images":["aGk="],"isRetryable":false,"warnings":[],"providerMetadata":{"p":{"x":1}},"usage":{"inputTokens":-1.5,"outputTokens":0,"totalTokens":2}}`)), Request: r}
	})
	result, err := client.GenerateImage(context.Background(), "p/m", ImageRequest{Prompt: &prompt, Count: 1, Size: &size, AspectRatio: &aspect, Seed: &seed, Files: []ImageInput{}, Mask: func() *ImageInput { var x ImageInput = ImageBytes{MediaType: "", Data: []byte("x")}; return &x }()})
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `{"prompt":"draw","n":1,"aspectRatio":"16:9","files":[],"mask":{"type":"file","mediaType":"","data":"eA=="}}` {
		t.Fatalf("wire=%s", got)
	}
	if string(result.Images[0]) != "hi" || result.Retryable == nil || *result.Retryable || result.Usage == nil || *result.Usage.InputTokens != -1.5 || result.Warnings == nil || string(result.ProviderMetadata["p"]) != `{"x":1}` {
		t.Fatalf("result=%#v", result)
	}
	got[0] = 'X'
	result.Response.Headers.Set("X-Test", "mutated")
	if string(result.Response.Body)[0] != '{' {
		t.Fatal("response body aliases request")
	}
}
func TestImageFilesNilVersusEmpty(t *testing.T) {
	for _, tc := range []struct {
		name  string
		files []ImageInput
		want  string
	}{{"nil", nil, `{"n":1}`}, {"empty", []ImageInput{}, `{"n":1,"files":[]}`}, {"forms", []ImageInput{ImageURL{URL: ""}, &ImageBase64{MediaType: "x", Data: "opaque"}, ImageBytes{MediaType: "x", Data: nil}}, `{"n":1,"files":[{"type":"url","url":""},{"type":"file","mediaType":"x","data":"opaque"},{"type":"file","mediaType":"x","data":""}]}`}} {
		t.Run(tc.name, func(t *testing.T) {
			b, e := prepareImageRequest("p/m", ImageRequest{Count: 1, Files: tc.files})
			if e != nil {
				t.Fatal(e)
			}
			if string(b) != tc.want {
				t.Fatalf("%s", b)
			}
		})
	}
}
func TestImageValidationAndTypedNil(t *testing.T) {
	var nilURL *ImageURL
	var mask ImageInput = nilURL
	cases := []struct {
		r    ImageRequest
		path string
	}{{ImageRequest{}, `$["count"]`}, {ImageRequest{Count: 17}, `$["count"]`}, {ImageRequest{Count: 1, Files: []ImageInput{nilURL}}, `$["files"][0]`}, {ImageRequest{Count: 1, Mask: &mask}, `$["mask"]`}}
	for _, tc := range cases {
		_, e := prepareImageRequest("p/m", tc.r)
		var v *ValidationError
		if !errors.As(e, &v) || v.Path() != tc.path {
			t.Fatalf("err=%v path want %s", e, tc.path)
		}
	}
}
func TestImageStrictResponse(t *testing.T) {
	bad := []string{`{}`, `{"images":null}`, `{"images":["aGk"]}`, `{"images":["aGk=\n"]}`, `{"images":[],"extra":1}`, `{"images":[],"images":[]}`, `{"images":[]} {}`}
	for _, body := range bad {
		_, e := decodeImageResult("p/m", rawProviderResponse{statusCode: 200, headers: http.Header{}, body: []byte(body)})
		var v *ResponseValidationError
		if !errors.As(e, &v) {
			t.Fatalf("body=%s err=%v", body, e)
		}
	}
	result, e := decodeImageResult("p/m", rawProviderResponse{statusCode: 200, headers: http.Header{}, body: []byte(`{"images":[],"usage":null}`)})
	if e != nil || result.Images == nil || len(result.Images) != 0 || result.Usage != nil {
		t.Fatalf("result=%#v err=%v", result, e)
	}
}
func TestImageResponsePresenceAndCountIndependence(t *testing.T) {
	values := make([]string, 16)
	for i := range values {
		values[i] = ""
	}
	body, _ := json.Marshal(map[string]any{"images": values, "isRetryable": true, "usage": map[string]any{"inputTokens": nil}})
	result, e := decodeImageResult("p/m", rawProviderResponse{body: body, headers: http.Header{}})
	if e != nil || len(result.Images) != 16 || result.Retryable == nil || !*result.Retryable || result.Usage == nil || result.Usage.InputTokens != nil {
		t.Fatalf("result=%#v err=%v", result, e)
	}
	for _, raw := range []string{`{"images":[]}`, `{"images":[],"usage":null}`, `{"images":[],"usage":{}}`} {
		result, e = decodeImageResult("p/m", rawProviderResponse{body: []byte(raw), headers: http.Header{}})
		if e != nil {
			t.Fatal(e)
		}
		if raw == `{"images":[],"usage":{}}` && result.Usage == nil {
			t.Fatal("empty usage lost presence")
		}
		if raw != `{"images":[],"usage":{}}` && result.Usage != nil {
			t.Fatal("absent/null usage became present")
		}
	}
}
func TestImageOutputBoundaries(t *testing.T) {
	data := bytes.Repeat([]byte{'x'}, maxImageDecodedBytes)
	encoded := base64.StdEncoding.EncodeToString(data)
	body, _ := json.Marshal(map[string]any{"images": []string{encoded}})
	result, e := decodeImageResult("p/m", rawProviderResponse{body: body, headers: http.Header{}})
	if e != nil || !reflect.DeepEqual(result.Images[0], data) {
		t.Fatalf("len=%d err=%v", len(result.Images[0]), e)
	}
	too := append(data, 'x')
	body, _ = json.Marshal(map[string]any{"images": []string{base64.StdEncoding.EncodeToString(too)}})
	if _, e = decodeImageResult("p/m", rawProviderResponse{body: body}); e == nil {
		t.Fatal("accepted oversized image")
	}
}
func TestGenerateImageOneAttemptAndNon200(t *testing.T) {
	var calls atomic.Int32
	client := imageClient(t, func(r *http.Request) *http.Response {
		calls.Add(1)
		return &http.Response{StatusCode: 429, Header: http.Header{"Retry-After": {"1"}}, Body: io.NopCloser(bytes.NewBufferString(`{"error":{"message":"slow"}}`)), Request: r}
	})
	client.config.retryPolicy = RetryPolicy{MaxAttempts: 4}
	_, e := client.GenerateImage(context.Background(), "p/m", ImageRequest{Count: 1})
	var re *ResponseError
	if !errors.As(e, &re) || calls.Load() != 1 || !re.Retryable() {
		t.Fatalf("err=%v calls=%d", e, calls.Load())
	}
}
