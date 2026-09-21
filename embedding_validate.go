package gateway

import "encoding/json"

func prepareEmbeddingRequest(modelID string, request EmbeddingRequest) ([]byte, error) {
	if modelID == "" || !validModelID(modelID) {
		return nil, validationError(memberPath("$", "modelID"), "must be a nonempty provider/model string")
	}
	if len(request.Values) < 1 || len(request.Values) > maxCollectionItems {
		return nil, validationError(memberPath("$", "values"), "must contain 1..4096 values")
	}
	for i, value := range request.Values {
		if len(value) > maxStringBytes {
			return nil, validationError(indexPath(memberPath("$", "values"), i), "must not exceed 1048576 bytes")
		}
	}
	options, err := encodeProviderOptions(request.ProviderOptions)
	if err != nil {
		return nil, err
	}
	wire := embeddingRequestWire{Values: append([]string(nil), request.Values...)}
	if options != nil {
		wire.ProviderOptions = options
	}
	payload, marshalErr := json.Marshal(wire)
	if marshalErr != nil {
		return nil, &TransportError{operation: "encode request", cause: marshalErr}
	}
	if len(payload) > maxRequestBodyBytes {
		return nil, validationError("$", "encoded request exceeds 16777216 bytes")
	}
	return payload, nil
}
