package gateway

// ResponseXSearchTool declares Gateway-native SpaceXAI x_search.
// It emits only {"type":"x_search"}.
type ResponseXSearchTool struct{}

func (ResponseXSearchTool) responseBuiltInTool() {}
