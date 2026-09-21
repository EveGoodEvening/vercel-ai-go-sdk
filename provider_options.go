package gateway

// ProviderOption is the closed set of provider-specific request options.
// This release intentionally provides no concrete implementations.
type ProviderOption interface {
	providerOption()
}

// encodeProviderOptions returns nil for both nil and empty inputs so callers
// omit providerOptions. Concrete option encodings are added alongside their
// sealed implementations.
func encodeProviderOptions(options []ProviderOption) (map[string]any, *ValidationError) {
	if len(options) == 0 {
		return nil, nil
	}

	// Keep validation and the closed encoding switch at this boundary so each
	// future sealed implementation can add its encoding without opening the API.
	encoded := make(map[string]any)
	for index, option := range options {
		path := indexPath(memberPath("$", "providerOptions"), index)
		if option == nil {
			return nil, validationError(path, "must be a non-nil provider option")
		}
		switch option.(type) {
		default:
			return nil, validationError(path, "provider option type is unsupported")
		}
	}
	return encoded, nil
}
