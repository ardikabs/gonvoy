//go:build e2e
// +build e2e

package tests

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/ardikabs/gonvoy/pkg/suite"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func init() {
	TestCases = append(TestCases, JwtValidationTestCase)
}

// JwtValidationTestCase observes Envoy jwt_authn via the x-claim-name header.
//
// Envoy is configured with an inline JWKS (hermetic, no remote fetch) for a
// single HS256 issuer. Tokens below are signed deterministically with the
// fixed test secret so the suite needs no external JWT dependency.
var JwtValidationTestCase = suite.TestCase{
	Name:        "JwtValidationTest",
	FilterName:  "jwt_validation",
	Description: "Validate JWT via Envoy jwt_authn (inline JWKS) and observe the claim_to_headers mapping in the Go filter.",
	Parallel:    true,
	Test: func(t *testing.T, kit *suite.TestSuiteKit) {
		stop := kit.StartEnvoy(t)
		defer stop()

		valid := jwtValidationBuildToken(t, jwtTestAudience, jwtTestClaimName, jwtTestValidExp)
		expired := jwtValidationBuildToken(t, jwtTestAudience, jwtTestClaimName, jwtTestExpiredExp)
		wrongAud := jwtValidationBuildToken(t, "wrong-audience", jwtTestClaimName, jwtTestValidExp)
		noNameClaim := jwtValidationBuildToken(t, jwtTestAudience, "", jwtTestValidExp)
		tampered := valid[:len(valid)-1] + jwtValidationFlipLast(valid[len(valid)-1])

		t.Run("missing token returns 401", func(t *testing.T) {
			resp, err := http.Get(kit.GetEnvoyHost() + "/")
			require.NoError(t, err)
			defer resp.Body.Close()

			_, _ = io.ReadAll(resp.Body)
			require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		})

		t.Run("valid token via header returns 200 and propagates claim", func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, kit.GetEnvoyHost()+"/", nil)
			require.NoError(t, err)

			req.Header.Set("Authorization", "Bearer "+valid)

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			_, _ = io.ReadAll(resp.Body)
			require.Equal(t, http.StatusOK, resp.StatusCode)
			require.Equal(t, jwtTestClaimName, resp.Header.Get("x-claim-name"))
		})

		t.Run("tampered signature returns 401", func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, kit.GetEnvoyHost()+"/", nil)
			require.NoError(t, err)

			req.Header.Set("Authorization", "Bearer "+tampered)

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			_, _ = io.ReadAll(resp.Body)
			require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		})

		t.Run("expired token returns 401", func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, kit.GetEnvoyHost()+"/", nil)
			require.NoError(t, err)

			req.Header.Set("Authorization", "Bearer "+expired)

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			_, _ = io.ReadAll(resp.Body)
			require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		})

		t.Run("wrong audience returns 403", func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, kit.GetEnvoyHost()+"/", nil)
			require.NoError(t, err)

			req.Header.Set("Authorization", "Bearer "+wrongAud)

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			_, _ = io.ReadAll(resp.Body)
			require.Equal(t, http.StatusForbidden, resp.StatusCode)
		})

		t.Run("valid token via query param returns 200", func(t *testing.T) {
			resp, err := http.Get(kit.GetEnvoyHost() + "/?access_token=" + valid)
			require.NoError(t, err)
			defer resp.Body.Close()

			_, _ = io.ReadAll(resp.Body)
			require.Equal(t, http.StatusOK, resp.StatusCode)
			require.Equal(t, jwtTestClaimName, resp.Header.Get("x-claim-name"))
		})

		t.Run("valid token via cookie returns 200", func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, kit.GetEnvoyHost()+"/", nil)
			require.NoError(t, err)

			req.AddCookie(&http.Cookie{Name: "access_token", Value: valid})

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			_, _ = io.ReadAll(resp.Body)
			require.Equal(t, http.StatusOK, resp.StatusCode)
			require.Equal(t, jwtTestClaimName, resp.Header.Get("x-claim-name"))
		})

		t.Run("valid signature without name claim returns 401 JSON from Go filter", func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, kit.GetEnvoyHost()+"/", nil)
			require.NoError(t, err)

			req.Header.Set("Authorization", "Bearer "+noNameClaim)

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			payload := gjson.ParseBytes(body)

			require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
			require.Equal(t, "UNAUTHORIZED", payload.Get("code").Str)
			require.Equal(t, "UNAUTHORIZED", payload.Get("message").Str)
		})
	},
}

// Fixed HS256 test identity. The base64url of jwtValidationTestSecret is
// embedded as the "k" parameter of the inline JWKS in envoy.yaml, so tokens
// signed here verify hermetically without any remote fetch.
const jwtValidationTestSecret = "gonvoy-e2e-jwt-test-secret-0123456789"

const jwtValidationTestIssuer = "https://test-issuer.example.com/"

const jwtTestAudience = "test-audience"

const jwtTestClaimName = "gonvoy-tester"

const jwtTestKid = "test-key"

// jwtTestValidExp is 2100-01-01T00:00:00Z, jwtTestExpiredExp is
// 2001-09-09T01:46:40Z, and jwtTestIat is fixed so tokens are deterministic.
const jwtTestValidExp = 4102444800

const jwtTestExpiredExp = 1000000000

const jwtTestIat = 1700000000

type jwtValidationClaims struct {
	Aud  string `json:"aud"`
	Exp  int64  `json:"exp"`
	Iat  int64  `json:"iat"`
	Iss  string `json:"iss"`
	Name string `json:"name,omitempty"`
	Sub  string `json:"sub"`
}

func jwtValidationBuildToken(t *testing.T, aud, name string, exp int64) string {
	t.Helper()

	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT","kid":"` + jwtTestKid + `"}`))

	payload, err := json.Marshal(jwtValidationClaims{
		Aud:  aud,
		Exp:  exp,
		Iat:  jwtTestIat,
		Iss:  jwtValidationTestIssuer,
		Name: name,
		Sub:  "test-user",
	})
	require.NoError(t, err)

	unsigned := header + "." + base64.RawURLEncoding.EncodeToString(payload)

	mac := hmac.New(sha256.New, []byte(jwtValidationTestSecret))
	_, err = mac.Write([]byte(unsigned))
	require.NoError(t, err)

	return unsigned + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func jwtValidationFlipLast(last byte) string {
	if last == 'A' {
		return "B"
	}

	return "A"
}
