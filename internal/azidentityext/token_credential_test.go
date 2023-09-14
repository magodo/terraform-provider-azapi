package azidentityext

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/golang-jwt/jwt/v4"
	"github.com/stretchr/testify/require"
)

func TestAzureAccessTokenCredential_GetToken(t *testing.T) {
	tk := jwt.Token{
		Method: jwt.SigningMethodRS256,
		Header: map[string]interface{}{
			"typ": "JWT",
			"alg": "RS256",
		},
		Claims: jwt.MapClaims{
			"foo": "bar",
			"exp": time.Now().Add(time.Hour).Unix(),
		},
	}
	private, _ := rsa.GenerateKey(rand.Reader, 512)
	tkStr, err := tk.SignedString(private)
	require.NoError(t, err)

	cred, err := NewAzureAccessTokenCredential([]byte(tkStr))
	require.NoError(t, err)

	// successful
	_, err = cred.GetToken(context.TODO(), policy.TokenRequestOptions{})
	require.NoError(t, err)

	// expired
	cred.expireOn = time.Now().Add(-time.Second)
	_, err = cred.GetToken(context.TODO(), policy.TokenRequestOptions{})
	require.Error(t, err)
}
