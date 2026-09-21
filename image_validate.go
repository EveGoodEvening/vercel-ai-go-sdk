package gateway

import (
	"encoding/base64"
	"encoding/json"
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
	if modelID == "" || !validModelID(modelID) {
		return nil, validationError(memberPath("$", "modelID"), "must be a nonempty provider/model string")
	}
	if r.Count < 1 || r.Count > 16 {
		return nil, validationError(memberPath("$", "count"), "must be between 1 and 16")
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
