package gateway

import (
	"context"
	"errors"
	"testing"
)

type testTokenSource struct {
	token  string
	err    error
	called int
}

func (source *testTokenSource) Token(context.Context) (string, error) {
	source.called++
	return source.token, source.err
}

func clearCredentialEnvironment(t *testing.T) {
	t.Helper()
	t.Setenv(apiKeyEnvironment, "")
	t.Setenv(oidcTokenEnvironment, "")
}

func requireCredential(t *testing.T, client *Client, kind credentialKind, token string, source TokenSource) {
	t.Helper()
	credential := client.config.credential
	if credential.kind != kind || credential.token != token || credential.source != source {
		t.Fatalf("credential = %#v, want kind %d, token %q, source %#v", credential, kind, token, source)
	}
}

func TestNewClientCredentialSelection(t *testing.T) {
	t.Run("explicit API key", func(t *testing.T) {
		clearCredentialEnvironment(t)
		client, err := NewClient(WithAPIKey(" explicit-key "))
		if err != nil {
			t.Fatal(err)
		}
		requireCredential(t, client, credentialAPIKey, " explicit-key ", nil)
	})

	t.Run("environment API key", func(t *testing.T) {
		clearCredentialEnvironment(t)
		t.Setenv(apiKeyEnvironment, " environment-key ")
		client, err := NewClient()
		if err != nil {
			t.Fatal(err)
		}
		requireCredential(t, client, credentialAPIKey, " environment-key ", nil)
	})

	t.Run("explicit OIDC token", func(t *testing.T) {
		clearCredentialEnvironment(t)
		client, err := NewClient(WithOIDCToken(" explicit-oidc "))
		if err != nil {
			t.Fatal(err)
		}
		requireCredential(t, client, credentialOIDCToken, " explicit-oidc ", nil)
	})

	t.Run("explicit OIDC source", func(t *testing.T) {
		clearCredentialEnvironment(t)
		source := &testTokenSource{}
		client, err := NewClient(WithOIDCTokenSource(source))
		if err != nil {
			t.Fatal(err)
		}
		requireCredential(t, client, credentialOIDCSource, "", source)
		if source.called != 0 {
			t.Fatalf("source called %d times during construction", source.called)
		}
	})

	t.Run("environment OIDC", func(t *testing.T) {
		clearCredentialEnvironment(t)
		t.Setenv(oidcTokenEnvironment, " environment-oidc ")
		client, err := NewClient()
		if err != nil {
			t.Fatal(err)
		}
		requireCredential(t, client, credentialOIDCToken, " environment-oidc ", nil)
	})
}

func TestNewClientCredentialPrecedence(t *testing.T) {
	t.Run("explicit API key wins all", func(t *testing.T) {
		clearCredentialEnvironment(t)
		t.Setenv(apiKeyEnvironment, "environment-key")
		t.Setenv(oidcTokenEnvironment, "environment-oidc")
		source := &testTokenSource{}
		client, err := NewClient(WithOIDCTokenSource(source), WithAPIKey("explicit-key"), WithOIDCToken("explicit-oidc"))
		if err != nil {
			t.Fatal(err)
		}
		requireCredential(t, client, credentialAPIKey, "explicit-key", nil)
		if source.called != 0 {
			t.Fatalf("source called %d times", source.called)
		}
	})

	t.Run("environment API key wins explicit OIDC token", func(t *testing.T) {
		clearCredentialEnvironment(t)
		t.Setenv(apiKeyEnvironment, "environment-key")
		client, err := NewClient(WithOIDCToken("explicit-oidc"))
		if err != nil {
			t.Fatal(err)
		}
		requireCredential(t, client, credentialAPIKey, "environment-key", nil)
	})

	t.Run("environment API key wins explicit OIDC source without invocation", func(t *testing.T) {
		clearCredentialEnvironment(t)
		t.Setenv(apiKeyEnvironment, "environment-key")
		source := &testTokenSource{}
		client, err := NewClient(WithOIDCTokenSource(source))
		if err != nil {
			t.Fatal(err)
		}
		requireCredential(t, client, credentialAPIKey, "environment-key", nil)
		if source.called != 0 {
			t.Fatalf("source called %d times", source.called)
		}
	})

	t.Run("explicit OIDC beats environment OIDC", func(t *testing.T) {
		clearCredentialEnvironment(t)
		t.Setenv(oidcTokenEnvironment, "environment-oidc")
		client, err := NewClient(WithOIDCToken("explicit-oidc"))
		if err != nil {
			t.Fatal(err)
		}
		requireCredential(t, client, credentialOIDCToken, "explicit-oidc", nil)
	})

	t.Run("last explicit OIDC token wins", func(t *testing.T) {
		clearCredentialEnvironment(t)
		source := &testTokenSource{}
		client, err := NewClient(WithOIDCTokenSource(source), WithOIDCToken("token"))
		if err != nil {
			t.Fatal(err)
		}
		requireCredential(t, client, credentialOIDCToken, "token", nil)
	})

	t.Run("last explicit OIDC source wins", func(t *testing.T) {
		clearCredentialEnvironment(t)
		source := &testTokenSource{}
		client, err := NewClient(WithOIDCToken("token"), WithOIDCTokenSource(source))
		if err != nil {
			t.Fatal(err)
		}
		requireCredential(t, client, credentialOIDCSource, "", source)
	})
}

func TestNewClientCredentialConfigurationErrors(t *testing.T) {
	var typedNil *testTokenSource

	tests := []struct {
		name       string
		option     Option
		wantOption string
		wantReason string
	}{
		{name: "blank API key", option: WithAPIKey("\u2003\t"), wantOption: "WithAPIKey", wantReason: "must not be empty or whitespace"},
		{name: "blank OIDC token", option: WithOIDCToken(" \n"), wantOption: "WithOIDCToken", wantReason: "must not be empty or whitespace"},
		{name: "nil OIDC source", option: WithOIDCTokenSource(nil), wantOption: "WithOIDCTokenSource", wantReason: "must not be nil"},
		{name: "typed nil OIDC source", option: WithOIDCTokenSource(typedNil), wantOption: "WithOIDCTokenSource", wantReason: "must not be nil"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clearCredentialEnvironment(t)
			_, err := NewClient(test.option)
			assertConfigurationError(t, err, test.wantOption, test.wantReason)
		})
	}

	t.Run("blank environments are absent", func(t *testing.T) {
		clearCredentialEnvironment(t)
		t.Setenv(apiKeyEnvironment, " \t")
		t.Setenv(oidcTokenEnvironment, "\u2003")
		_, err := NewClient()
		assertConfigurationError(t, err, "credentials", "no credential configured")
	})
}

func assertConfigurationError(t *testing.T, err error, option, reason string) {
	t.Helper()
	var configurationError *ConfigurationError
	if !errors.As(err, &configurationError) {
		t.Fatalf("error = %v, want ConfigurationError", err)
	}
	if configurationError.Option() != option || configurationError.Reason() != reason {
		t.Fatalf("ConfigurationError = (%q, %q), want (%q, %q)", configurationError.Option(), configurationError.Reason(), option, reason)
	}
}
