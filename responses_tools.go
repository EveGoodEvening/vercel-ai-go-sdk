package gateway

// ResponseXSearchTool declares Gateway-native SpaceXAI x_search.
// It emits only {"type":"x_search"}.
type ResponseXSearchTool struct{}

func (ResponseXSearchTool) responseBuiltInTool() {}

// ResponseXSearchOptionsTool declares Gateway-native SpaceXAI x_search with
// request options. Nil fields are omitted; non-nil empty slices are emitted.
type ResponseXSearchOptionsTool struct {
	AllowedXHandles          []string
	ExcludedXHandles         []string
	FromDate                 *string
	ToDate                   *string
	EnableImageUnderstanding *bool
	EnableVideoUnderstanding *bool
}

func (ResponseXSearchOptionsTool) responseBuiltInTool() {}
