package gateway

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
)

type preparedRerankRequest struct {
	payload []byte
	texts   []string
	objects []json.RawMessage
	topN    *int
}

type rerankRequestWire struct {
	Query     string `json:"query"`
	Documents struct {
		Type   string   `json:"type"`
		Values []string `json:"values"`
	} `json:"documents"`
	TopN            *int           `json:"topN,omitempty"`
	ProviderOptions map[string]any `json:"providerOptions,omitempty"`
}

func prepareRerankRequest(modelID string, request RerankRequest) (*preparedRerankRequest, error) {
	if modelID == "" || !validModelID(modelID) {
		return nil, validationError(memberPath("$", "modelID"), "must be a nonempty provider/model string")
	}
	p := &preparedRerankRequest{}
	wire := rerankRequestWire{Query: request.Query, TopN: request.TopN}
	count := 0
	switch docs := request.Documents.(type) {
	case RerankTexts:
		count = len(docs.Values)
		p.texts = append([]string(nil), docs.Values...)
	case *RerankTexts:
		if docs == nil {
			return nil, validationError(memberPath("$", "documents"), "must be a non-nil supported document envelope")
		}
		count = len(docs.Values)
		p.texts = append([]string(nil), docs.Values...)
	case RerankObjects:
		count = len(docs.Values)
		p.objects = make([]json.RawMessage, count)
		copy(p.objects, docs.Values)
	case *RerankObjects:
		if docs == nil {
			return nil, validationError(memberPath("$", "documents"), "must be a non-nil supported document envelope")
		}
		count = len(docs.Values)
		p.objects = make([]json.RawMessage, count)
		copy(p.objects, docs.Values)
	default:
		return nil, validationError(memberPath("$", "documents"), "must be a non-nil supported document envelope")
	}
	if count < 1 || count > maxCollectionItems {
		return nil, validationError(memberPath(memberPath("$", "documents"), "values"), "must contain 1..4096 values")
	}
	if len(request.Query) > maxStringBytes {
		return nil, validationError(memberPath("$", "query"), "must not exceed 1048576 bytes")
	}
	valuesPath := memberPath(memberPath("$", "documents"), "values")
	if p.texts != nil {
		for i, value := range p.texts {
			if len(value) > maxStringBytes {
				return nil, validationError(indexPath(valuesPath, i), "must not exceed 1048576 bytes")
			}
		}
		wire.Documents.Type, wire.Documents.Values = "text", p.texts
	} else {
		for i, value := range p.objects {
			path := indexPath(valuesPath, i)
			if len(value) > maxStringBytes {
				return nil, validationError(path, "must not exceed 1048576 bytes")
			}
			compact, err := validateAndCompactRerankObject(value, path)
			if err != nil {
				return nil, err
			}
			p.objects[i] = compact
		}
	}
	if request.TopN != nil {
		if *request.TopN < 1 || *request.TopN > count {
			return nil, validationError(memberPath("$", "topN"), "must be in 1..len(documents)")
		}
		v := *request.TopN
		p.topN = &v
	}
	options, err := encodeProviderOptions(request.ProviderOptions)
	if err != nil {
		return nil, err
	}
	wire.ProviderOptions = options
	var payload []byte
	if p.objects == nil {
		var marshalErr error
		payload, marshalErr = json.Marshal(wire)
		if marshalErr != nil {
			return nil, &TransportError{operation: "encode request", cause: marshalErr}
		}
	} else {
		query, marshalErr := json.Marshal(request.Query)
		if marshalErr != nil {
			return nil, &TransportError{operation: "encode request", cause: marshalErr}
		}
		payload = append(payload, `{"query":`...)
		payload = append(payload, query...)
		payload = append(payload, `,"documents":{"type":"object","values":[`...)
		for i, object := range p.objects {
			if i != 0 {
				payload = append(payload, ',')
			}
			payload = append(payload, object...)
		}
		payload = append(payload, ']', '}')
		if request.TopN != nil {
			top, _ := json.Marshal(*request.TopN)
			payload = append(payload, `,"topN":`...)
			payload = append(payload, top...)
		}
		if options != nil {
			encoded, marshalErr := json.Marshal(options)
			if marshalErr != nil {
				return nil, &TransportError{operation: "encode request", cause: marshalErr}
			}
			payload = append(payload, `,"providerOptions":`...)
			payload = append(payload, encoded...)
		}
		payload = append(payload, '}')
	}
	if len(payload) > maxRequestBodyBytes {
		return nil, validationError("$", "encoded request exceeds 16777216 bytes")
	}
	p.payload = append([]byte(nil), payload...)
	return p, nil
}

func validateAndCompactRerankObject(raw []byte, path string) ([]byte, error) {
	if len(raw) == 0 {
		return nil, validationError(path, "must be a non-null JSON object")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	tok, err := d.Token()
	if err != nil || tok != json.Delim('{') {
		return nil, validationError(path, "must be a non-null JSON object")
	}
	// Re-scan from the beginning with shared recursive resource and duplicate-key rules.
	d = json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	if f := scanEmbeddingValue(d, path, 1, true, true); f != nil {
		return nil, validationError(f.path, f.reason)
	}
	if _, err = d.Token(); err == nil {
		return nil, validationError(path, "trailing JSON value")
	} else if !errors.Is(err, io.EOF) {
		return nil, validationError(path, "malformed JSON")
	}
	var out bytes.Buffer
	if err = json.Compact(&out, raw); err != nil {
		return nil, validationError(path, "malformed JSON")
	}
	return append([]byte(nil), out.Bytes()...), nil
}
