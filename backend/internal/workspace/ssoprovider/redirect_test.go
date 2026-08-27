package ssoprovider

import "testing"

func TestValidateRedirectURI(t *testing.T) {
	t.Setenv("OAUTH_REDIRECT_HOSTS", "client.example, other.example")
	for _, uri := range []string{"https://client.example/callback", "https://other.example/oauth/cb", "http://localhost:3000/callback"} {
		if err := ValidateRedirectURI(uri); err != nil {
			t.Errorf("%s: %v", uri, err)
		}
	}
	for _, uri := range []string{"http://client.example/callback", "https://evil.example/callback", "https://sub.client.example/callback", "https://client.example/cb#fragment"} {
		if err := ValidateRedirectURI(uri); err == nil {
			t.Errorf("%s unexpectedly accepted", uri)
		}
	}
}
