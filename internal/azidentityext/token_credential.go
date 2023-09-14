package azidentityext

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/golang-jwt/jwt/v4"
)

type AzureAccessTokenCredential struct {
	token    []byte
	expireOn time.Time
}

// NewAzureAccessTokenCredential returns a TokenCredential that simply use the specified token when called with GetToken.
// When the token get expired, both this function and the GetToken method on the returned TokenCredential fails.
// Especially, there will be no handling on the options for the GetToken method (e.g. the "scopes" will not be validated).
func NewAzureAccessTokenCredential(token []byte) (*AzureAccessTokenCredential, error) {
	tk, _, err := jwt.NewParser().ParseUnverified(string(token), jwt.MapClaims{})
	if err != nil {
		return nil, fmt.Errorf("parsing JWT token: %v", err)
	}
	claims := tk.Claims.(jwt.MapClaims)
	exp, ok := claims["exp"]
	if !ok {
		return nil, fmt.Errorf(`no "exp" found in claim`)
	}
	expireOn := time.Unix(int64(exp.(float64)), 0)
	if time.Now().After(expireOn) {
		return nil, fmt.Errorf("token has already expired")
	}
	return &AzureAccessTokenCredential{
		token:    token,
		expireOn: expireOn,
	}, nil
}

func (c *AzureAccessTokenCredential) GetToken(ctx context.Context, _ policy.TokenRequestOptions) (azcore.AccessToken, error) {
	if time.Now().After(c.expireOn) {
		return azcore.AccessToken{}, fmt.Errorf("token has already expired")
	}
	return azcore.AccessToken{
		Token:     string(c.token),
		ExpiresOn: c.expireOn,
	}, nil
}
