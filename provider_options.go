package gateway

// ProviderOption is the closed set of provider-specific request options.
// This release intentionally provides no concrete implementations.
type ProviderOption interface {
	providerOption()
}

// encodeProviderOptions returns nil for both nil and empty inputs so callers
// omit providerOptions. Concrete option encodings are added alongside their
// sealed implementations.
func encodeProviderOptions(options []ProviderOption) map[string]any {
	if len(options) == 0 {
		return nil
	}

	// No concrete ProviderOption exists in this release, so a nonempty slice
	// cannot be constructed by an external caller. Keep this closed switch at
	// the encoder boundary for future in-package implementations.
	encoded := make(map[string]any)
	for range options {
		panic("gateway: unsupported provider option implementation")
	}
	return encoded
}
