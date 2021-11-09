package acceptance

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/hashicorp/go-azure-helpers/authentication"
	"github.com/ms-henglu/terraform-provider-azurermg/internal/clients"
	"github.com/ms-henglu/terraform-provider-azurermg/internal/features"
)

var (
	_client    *clients.Client
	clientLock = &sync.Mutex{}
)

func BuildTestClient() (*clients.Client, error) {
	clientLock.Lock()
	defer clientLock.Unlock()

	if _client == nil {
		environment, exists := os.LookupEnv("ARM_ENVIRONMENT")
		if !exists {
			environment = "public"
		}

		builder := authentication.Builder{
			SubscriptionID: os.Getenv("ARM_SUBSCRIPTION_ID"),
			ClientID:       os.Getenv("ARM_CLIENT_ID"),
			TenantID:       os.Getenv("ARM_TENANT_ID"),
			ClientSecret:   os.Getenv("ARM_CLIENT_SECRET"),
			Environment:    environment,
			MetadataHost:   os.Getenv("ARM_METADATA_HOST"),

			// we intentionally only support Client Secret auth for tests (since those variables are used all over)
			SupportsClientSecretAuth: true,
		}
		config, err := builder.Build()
		if err != nil {
			return nil, fmt.Errorf("building ARM Client: %+v", err)
		}

		clientBuilder := clients.ClientBuilder{
			AuthConfig:               config,
			SkipProviderRegistration: true,
			TerraformVersion:         os.Getenv("TERRAFORM_CORE_VERSION"),
			StorageUseAzureAD:        false,
			Features:                 features.Default(),
		}
		client, err := clients.Build(context.TODO(), clientBuilder)
		if err != nil {
			return nil, err
		}
		_client = client
	}

	return _client, nil
}
