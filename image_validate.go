package gateway

import (
	"encoding/base64"
	"encoding/json"
	"math"
	"strconv"
	"unicode/utf8"
)

type imageInputWire struct {
	Type            string         `json:"type"`
	URL             *string        `json:"url,omitempty"`
	MediaType       *string        `json:"mediaType,omitempty"`
	Data            *string        `json:"data,omitempty"`
	ProviderOptions map[string]any `json:"providerOptions,omitempty"`
}
type imageRequestWire struct {
	Prompt          *string           `json:"prompt,omitempty"`
	N               int               `json:"n"`
	Size            *ImageSize        `json:"size,omitempty"`
	AspectRatio     *ImageAspectRatio `json:"aspectRatio,omitempty"`
	Seed            *int64            `json:"seed,omitempty"`
	Files           *[]imageInputWire `json:"files,omitempty"`
	Mask            *imageInputWire   `json:"mask,omitempty"`
	ProviderOptions map[string]any    `json:"providerOptions,omitempty"`
}

func cloneImageRequest(r ImageRequest) ImageRequest {
	if r.Prompt != nil {
		v := *r.Prompt
		r.Prompt = &v
	}
	if r.Size != nil {
		v := *r.Size
		r.Size = &v
	}
	if r.AspectRatio != nil {
		v := *r.AspectRatio
		r.AspectRatio = &v
	}
	if r.Seed != nil {
		v := *r.Seed
		r.Seed = &v
	}
	if r.Files != nil {
		f := make([]ImageInput, len(r.Files))
		for i, v := range r.Files {
			f[i] = cloneImageInput(v)
		}
		r.Files = f
	}
	if r.Mask != nil {
		v := cloneImageInput(*r.Mask)
		r.Mask = &v
	}
	r.ProviderOptions = append([]ProviderOption(nil), r.ProviderOptions...)
	return r
}
func cloneImageInput(v ImageInput) ImageInput {
	switch x := v.(type) {
	case ImageURL:
		x.ProviderOptions = append([]ProviderOption(nil), x.ProviderOptions...)
		return x
	case *ImageURL:
		if x == nil {
			return x
		}
		y := *x
		y.ProviderOptions = append([]ProviderOption(nil), x.ProviderOptions...)
		return &y
	case ImageBase64:
		x.ProviderOptions = append([]ProviderOption(nil), x.ProviderOptions...)
		return x
	case *ImageBase64:
		if x == nil {
			return x
		}
		y := *x
		y.ProviderOptions = append([]ProviderOption(nil), x.ProviderOptions...)
		return &y
	case ImageBytes:
		x.Data = append([]byte(nil), x.Data...)
		x.ProviderOptions = append([]ProviderOption(nil), x.ProviderOptions...)
		return x
	case *ImageBytes:
		if x == nil {
			return x
		}
		y := *x
		y.Data = append([]byte(nil), x.Data...)
		y.ProviderOptions = append([]ProviderOption(nil), x.ProviderOptions...)
		return &y
	default:
		return v
	}
}

func prepareImageRequest(modelID string, r ImageRequest) ([]byte, error) {
	if _, err := preflightImageRequest(modelID, r); err != nil {
		return nil, err
	}
	if r.Prompt != nil && len(*r.Prompt) > maxStringBytes {
		return nil, validationError(memberPath("$", "prompt"), "must not exceed 1048576 bytes")
	}
	if r.Size != nil && *r.Size != "" && len(*r.Size) > 255 {
		return nil, validationError(memberPath("$", "size"), "must not exceed 255 bytes")
	}
	if r.AspectRatio != nil && *r.AspectRatio != "" && len(*r.AspectRatio) > 255 {
		return nil, validationError(memberPath("$", "aspectRatio"), "must not exceed 255 bytes")
	}
	w := imageRequestWire{Prompt: r.Prompt, N: r.Count}
	if r.Size != nil && *r.Size != "" {
		w.Size = r.Size
	}
	if r.AspectRatio != nil && *r.AspectRatio != "" {
		w.AspectRatio = r.AspectRatio
	}
	if r.Seed != nil && *r.Seed != 0 {
		w.Seed = r.Seed
	}
	if r.Files != nil {
		files := make([]imageInputWire, len(r.Files))
		for i, v := range r.Files {
			x, e := encodeImageInput(v, indexPath(memberPath("$", "files"), i))
			if e != nil {
				return nil, e
			}
			files[i] = x
		}
		w.Files = &files
	}
	if r.Mask != nil {
		x, e := encodeImageInput(*r.Mask, memberPath("$", "mask"))
		if e != nil {
			return nil, e
		}
		w.Mask = &x
	}
	o, e := encodeImageOptions(r.ProviderOptions, memberPath("$", "providerOptions"))
	if e != nil {
		return nil, e
	}
	w.ProviderOptions = o
	b, e2 := json.Marshal(w)
	if e2 != nil {
		return nil, &TransportError{operation: "encode request", cause: e2}
	}
	if len(b) > maxRequestBodyBytes {
		return nil, validationError("$", "encoded request exceeds 16777216 bytes")
	}
	return b, nil
}

func preflightImageRequest(modelID string, r ImageRequest) (int, error) {
	if modelID == "" || !validModelID(modelID) {
		return 0, validationError(memberPath("$", "modelID"), "must be a nonempty provider/model string")
	}
	if r.Count < 1 || r.Count > 16 {
		return 0, validationError(memberPath("$", "count"), "must be between 1 and 16")
	}
	if r.Prompt != nil && len(*r.Prompt) > maxStringBytes {
		return 0, validationError(memberPath("$", "prompt"), "must not exceed 1048576 bytes")
	}
	if r.Size != nil && *r.Size != "" && len(*r.Size) > 255 {
		return 0, validationError(memberPath("$", "size"), "must not exceed 255 bytes")
	}
	if r.AspectRatio != nil && *r.AspectRatio != "" && len(*r.AspectRatio) > 255 {
		return 0, validationError(memberPath("$", "aspectRatio"), "must not exceed 255 bytes")
	}

	total := 2
	fields := 0
	addField := func(name string, valueLength int) bool {
		addition := jsonQuotedLength(name) + 1 + valueLength
		if fields != 0 {
			addition++
		}
		var ok bool
		total, ok = checkedImageLengthAdd(total, addition)
		fields++
		return ok && total <= maxRequestBodyBytes
	}
	if r.Prompt != nil && !addField("prompt", jsonQuotedLength(*r.Prompt)) {
		return 0, imageRequestTooLargeError()
	}
	if !addField("n", len(strconv.Itoa(r.Count))) {
		return 0, imageRequestTooLargeError()
	}
	if r.Size != nil && *r.Size != "" && !addField("size", jsonQuotedLength(string(*r.Size))) {
		return 0, imageRequestTooLargeError()
	}
	if r.AspectRatio != nil && *r.AspectRatio != "" && !addField("aspectRatio", jsonQuotedLength(string(*r.AspectRatio))) {
		return 0, imageRequestTooLargeError()
	}
	if r.Seed != nil && *r.Seed != 0 && !addField("seed", len(strconv.FormatInt(*r.Seed, 10))) {
		return 0, imageRequestTooLargeError()
	}
	if r.Files != nil {
		arrayLength := 2
		for i, input := range r.Files {
			inputLength, err := preflightImageInput(input, indexPath(memberPath("$", "files"), i))
			if err != nil {
				return 0, err
			}
			if i != 0 {
				arrayLength, _ = checkedImageLengthAdd(arrayLength, 1)
			}
			var ok bool
			arrayLength, ok = checkedImageLengthAdd(arrayLength, inputLength)
			if !ok || arrayLength > maxRequestBodyBytes {
				return 0, imageRequestTooLargeError()
			}
		}
		if !addField("files", arrayLength) {
			return 0, imageRequestTooLargeError()
		}
	}
	if r.Mask != nil {
		inputLength, err := preflightImageInput(*r.Mask, memberPath("$", "mask"))
		if err != nil {
			return 0, err
		}
		if !addField("mask", inputLength) {
			return 0, imageRequestTooLargeError()
		}
	}
	optionsLength, err := preflightImageOptions(r.ProviderOptions, memberPath("$", "providerOptions"))
	if err != nil {
		return 0, err
	}
	if optionsLength != 0 && !addField("providerOptions", optionsLength) {
		return 0, imageRequestTooLargeError()
	}
	return total, nil
}

func preflightImageInput(v ImageInput, path string) (int, error) {
	if v == nil {
		return 0, validationError(path, "must be a non-nil image input")
	}
	var typ, url, media, data string
	var byteLength int
	var opts []ProviderOption
	isURL, isBytes := false, false
	switch x := v.(type) {
	case ImageURL:
		typ, url, opts, isURL = "url", x.URL, x.ProviderOptions, true
	case *ImageURL:
		if x == nil {
			return 0, validationError(path, "must be a non-nil image input")
		}
		typ, url, opts, isURL = "url", x.URL, x.ProviderOptions, true
	case ImageBase64:
		typ, media, data, opts = "file", x.MediaType, x.Data, x.ProviderOptions
	case *ImageBase64:
		if x == nil {
			return 0, validationError(path, "must be a non-nil image input")
		}
		typ, media, data, opts = "file", x.MediaType, x.Data, x.ProviderOptions
	case ImageBytes:
		typ, media, byteLength, opts, isBytes = "file", x.MediaType, len(x.Data), x.ProviderOptions, true
	case *ImageBytes:
		if x == nil {
			return 0, validationError(path, "must be a non-nil image input")
		}
		typ, media, byteLength, opts, isBytes = "file", x.MediaType, len(x.Data), x.ProviderOptions, true
	default:
		return 0, validationError(path, "image input type is unsupported")
	}
	if isURL {
		if len(url) > maxStringBytes {
			return 0, validationError(memberPath(path, "url"), "must not exceed 1048576 bytes")
		}
	} else {
		if len(media) > maxStringBytes {
			return 0, validationError(memberPath(path, "mediaType"), "must not exceed 1048576 bytes")
		}
		if !isBytes && len(data) > maxStringBytes {
			return 0, validationError(memberPath(path, "data"), "must not exceed 1048576 bytes")
		}
	}
	optionsLength, err := preflightImageOptions(opts, memberPath(path, "providerOptions"))
	if err != nil {
		return 0, err
	}
	length := 2
	fields := 0
	add := func(name string, valueLength int) bool {
		addition := jsonQuotedLength(name) + 1 + valueLength
		if fields != 0 {
			addition++
		}
		var ok bool
		length, ok = checkedImageLengthAdd(length, addition)
		fields++
		return ok && length <= maxRequestBodyBytes
	}
	if !add("type", jsonQuotedLength(typ)) {
		return 0, imageRequestTooLargeError()
	}
	if isURL {
		if !add("url", jsonQuotedLength(url)) {
			return 0, imageRequestTooLargeError()
		}
	} else {
		if !add("mediaType", jsonQuotedLength(media)) {
			return 0, imageRequestTooLargeError()
		}
		dataLength := jsonQuotedLength(data)
		if isBytes {
			encodedLength, ok := checkedBase64EncodedLen(byteLength)
			if !ok {
				return 0, imageRequestTooLargeError()
			}
			dataLength, ok = checkedImageLengthAdd(encodedLength, 2)
			if !ok {
				return 0, imageRequestTooLargeError()
			}
		}
		if !add("data", dataLength) {
			return 0, imageRequestTooLargeError()
		}
	}
	if optionsLength != 0 && !add("providerOptions", optionsLength) {
		return 0, imageRequestTooLargeError()
	}
	return length, nil
}

func preflightImageOptions(options []ProviderOption, path string) (int, error) {
	if len(options) == 0 {
		return 0, nil
	}
	for i, option := range options {
		if option == nil {
			return 0, validationError(indexPath(path, i), "must be a non-nil provider option")
		}
		switch option.(type) {
		default:
			return 0, validationError(indexPath(path, i), "provider option type is unsupported")
		}
	}
	return 2, nil
}

func checkedBase64EncodedLen(n int) (int, bool) {
	if n < 0 || n > (math.MaxInt/4)*3 {
		return 0, false
	}
	return ((n + 2) / 3) * 4, true
}

func checkedImageLengthAdd(a, b int) (int, bool) {
	if a < 0 || b < 0 || a > math.MaxInt-b {
		return 0, false
	}
	return a + b, true
}

// encoding/json can emit invalid UTF-8 as either an escaped or literal U+FFFD.
// Measure once so preflight matches the active encoder without allocating per call.
var jsonInvalidUTF8Length = func() int {
	encoded, _ := json.Marshal("\xff")
	return len(encoded) - 2 // Exclude the surrounding quotes.
}()

func jsonQuotedLength(value string) int {
	length := 2
	for index := 0; index < len(value); {
		character := value[index]
		if character < utf8.RuneSelf {
			index++
			switch {
			case character == '"' || character == '\\' || character == '\b' || character == '\f' || character == '\n' || character == '\r' || character == '\t':
				length += 2
			case character < 0x20:
				length += 6
			case character == '<' || character == '>' || character == '&':
				length += 6
			default:
				length++
			}
			continue
		}
		r, size := utf8.DecodeRuneInString(value[index:])
		if r == utf8.RuneError && size == 1 {
			length += jsonInvalidUTF8Length
			index++
		} else if r == '\u2028' || r == '\u2029' {
			length += 6
			index += size
		} else {
			length += size
			index += size
		}
	}
	return length
}

func imageRequestTooLargeError() error {
	return validationError("$", "encoded request exceeds 16777216 bytes")
}
func encodeImageOptions(options []ProviderOption, path string) (map[string]any, error) {
	if len(options) == 0 {
		return nil, nil
	}
	for i, o := range options {
		if o == nil {
			return nil, validationError(indexPath(path, i), "must be a non-nil provider option")
		}
		switch o.(type) {
		default:
			return nil, validationError(indexPath(path, i), "provider option type is unsupported")
		}
	}
	return map[string]any{}, nil
}
func encodeImageInput(v ImageInput, path string) (imageInputWire, error) {
	if v == nil {
		return imageInputWire{}, validationError(path, "must be a non-nil image input")
	}
	var typ, url, media, data string
	var opts []ProviderOption
	validateDataString := false
	switch x := v.(type) {
	case ImageURL:
		typ = "url"
		url = x.URL
		opts = x.ProviderOptions
	case *ImageURL:
		if x == nil {
			return imageInputWire{}, validationError(path, "must be a non-nil image input")
		}
		typ = "url"
		url = x.URL
		opts = x.ProviderOptions
	case ImageBase64:
		typ = "file"
		media = x.MediaType
		data = x.Data
		opts = x.ProviderOptions
		validateDataString = true
	case *ImageBase64:
		if x == nil {
			return imageInputWire{}, validationError(path, "must be a non-nil image input")
		}
		typ = "file"
		media = x.MediaType
		data = x.Data
		opts = x.ProviderOptions
		validateDataString = true
	case ImageBytes:
		typ = "file"
		media = x.MediaType
		data = base64.StdEncoding.EncodeToString(x.Data)
		opts = x.ProviderOptions
	case *ImageBytes:
		if x == nil {
			return imageInputWire{}, validationError(path, "must be a non-nil image input")
		}
		typ = "file"
		media = x.MediaType
		data = base64.StdEncoding.EncodeToString(x.Data)
		opts = x.ProviderOptions
	default:
		return imageInputWire{}, validationError(path, "image input type is unsupported")
	}
	if typ == "url" {
		if len(url) > maxStringBytes {
			return imageInputWire{}, validationError(memberPath(path, "url"), "must not exceed 1048576 bytes")
		}
	} else {
		if len(media) > maxStringBytes {
			return imageInputWire{}, validationError(memberPath(path, "mediaType"), "must not exceed 1048576 bytes")
		}
		if validateDataString && len(data) > maxStringBytes {
			return imageInputWire{}, validationError(memberPath(path, "data"), "must not exceed 1048576 bytes")
		}
	}
	o, e := encodeImageOptions(opts, memberPath(path, "providerOptions"))
	if e != nil {
		return imageInputWire{}, e
	}
	out := imageInputWire{Type: typ, ProviderOptions: o}
	if typ == "url" {
		out.URL = &url
	} else {
		out.MediaType = &media
		out.Data = &data
	}
	return out, nil
}
