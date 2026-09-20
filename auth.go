package gateway

import (
	"context"
	"errors"
	"os"
)

const (
	apiKeyEnvironment    = "AI_GATEWAY_API_KEY"
	oidcTokenEnvironment = "VERCEL_OIDC_TOKEN"
)

var errBlankOIDCToken = errors.New("OIDC token source returned an empty or whitespace token")

type resolvedCredential struct {
	kind   credentialKind
	token  string
	source TokenSource
}

func resolveCredential(config *clientConfig) error {
	apiKey := os.Getenv(apiKeyEnvironment)
	oidcToken := os.Getenv(oidcTokenEnvironment)
	switch {
	case config.explicitAPIKey:
		config.credential = resolvedCredential{kind: credentialAPIKey, token: config.apiKey}
	case !blank(apiKey):
		config.credential = resolvedCredential{kind: credentialAPIKey, token: apiKey}
	case config.explicitOIDC == credentialOIDCToken:
		config.credential = resolvedCredential{kind: credentialOIDCToken, token: config.oidcToken}
	case config.explicitOIDC == credentialOIDCSource:
		config.credential = resolvedCredential{kind: credentialOIDCSource, source: config.oidcTokenSource}
	case !blank(oidcToken):
		config.credential = resolvedCredential{kind: credentialOIDCToken, token: oidcToken}
	default:
		return newConfigurationError("credentials", "no credential configured")
	}
	return nil
}

func (credential resolvedCredential) authorization(ctx context.Context) (string, string, error) {
	token := credential.token
	if credential.kind == credentialOIDCSource {
		var err error
		token, err = credential.source.Token(ctx)
		if err != nil {
			return "", "", &TransportError{operation: "resolve OIDC token", cause: err}
		}
		if blank(token) {
			return "", "", &TransportError{operation: "resolve OIDC token", cause: errBlankOIDCToken}
		}
	}

	switch credential.kind {
	case credentialAPIKey:
		return "Bearer " + token, "api-key", nil
	case credentialOIDCToken, credentialOIDCSource:
		return "Bearer " + token, "oidc", nil
	default:
		panic("gateway: unresolved credential")
	}
}
