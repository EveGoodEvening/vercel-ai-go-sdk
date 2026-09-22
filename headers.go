package gateway

import "net/http"

const (
	headerAuthorization                          = "Authorization"
	headerContentType                            = "Content-Type"
	headerGatewayProtocolVersion                 = "Ai-Gateway-Protocol-Version"
	headerGatewayAuthMethod                      = "Ai-Gateway-Auth-Method"
	headerEvaluationModelSpecificationVersion    = "Ai-Evaluation-Model-Specification-Version"
	headerEmbeddingModelSpecificationVersion     = "Ai-Embedding-Model-Specification-Version"
	headerRerankingModelSpecificationVersion     = "Ai-Reranking-Model-Specification-Version"
	headerImageModelSpecificationVersion         = "Ai-Image-Model-Specification-Version"
	headerSpeechModelSpecificationVersion        = "Ai-Speech-Model-Specification-Version"
	headerTranscriptionModelSpecificationVersion = "Ai-Transcription-Model-Specification-Version"
	headerLanguageModelSpecificationVersion      = "Ai-Language-Model-Specification-Version"
	headerModelID                                = "Ai-Model-Id"
	headerTeam                                   = "X-Vercel-Ai-Gateway-Team"

	gatewayProtocolVersion              = "0.0.1"
	evaluationModelSpecificationVersion = "4"
	providerModelSpecificationVersion   = "4"
)

var protectedHeaderNames = map[string]struct{}{
	headerAuthorization:                       {},
	headerContentType:                         {},
	headerGatewayProtocolVersion:              {},
	headerGatewayAuthMethod:                   {},
	headerEvaluationModelSpecificationVersion: {},
	headerModelID:                             {},
	headerTeam:                                {},
}

var providerProtectedHeaderNames = map[string]struct{}{
	headerEmbeddingModelSpecificationVersion:     {},
	headerRerankingModelSpecificationVersion:     {},
	headerImageModelSpecificationVersion:         {},
	headerSpeechModelSpecificationVersion:        {},
	headerTranscriptionModelSpecificationVersion: {},
	headerLanguageModelSpecificationVersion:      {},
}

func containsProtectedHeader(headers http.Header) bool {
	for name := range headers {
		canonicalName := http.CanonicalHeaderKey(name)
		if _, protected := protectedHeaderNames[canonicalName]; protected {
			return true
		}
		if _, protected := providerProtectedHeaderNames[canonicalName]; protected {
			return true
		}
	}
	return false
}

func (config *clientConfig) buildHeaders(authorization, authMethod, modelID string) http.Header {
	headers := config.buildOwnedHeaders(authorization)
	headers.Set(headerGatewayProtocolVersion, gatewayProtocolVersion)
	headers.Set(headerGatewayAuthMethod, authMethod)
	headers.Set(headerEvaluationModelSpecificationVersion, evaluationModelSpecificationVersion)
	headers.Set(headerModelID, modelID)
	return headers
}

func (config *clientConfig) buildProviderHeaders(authorization, authMethod, modelID, specificationHeader string) http.Header {
	headers := config.buildOwnedHeaders(authorization)
	headers.Set(headerGatewayProtocolVersion, gatewayProtocolVersion)
	headers.Set(headerGatewayAuthMethod, authMethod)
	headers.Set(specificationHeader, providerModelSpecificationVersion)
	headers.Set(headerModelID, modelID)
	return headers
}

func (config *clientConfig) buildPublicHeaders(authorization string) http.Header {
	return config.buildOwnedHeaders(authorization)
}

func (config *clientConfig) buildOwnedHeaders(authorization string) http.Header {
	headers := config.headers.Clone()
	if headers == nil {
		headers = make(http.Header, len(protectedHeaderNames))
	}
	headers.Set(headerAuthorization, authorization)
	headers.Set(headerContentType, "application/json")
	if config.team != "" {
		headers.Set(headerTeam, config.team)
	}
	return headers
}
