package cypher

import (
	"context"
	"crypto/tls"
	"fmt"
	vault "github.com/hashicorp/vault/api"
	auth "github.com/hashicorp/vault/api/auth/approle"
	"gitlab.com/duel-duck/duel-duck-api/config"
	"net/http"
)

func CreateCypherDBClient(c *config.Config) (*vault.Client, error) {
	cfg := vault.DefaultConfig()
	cfg.Address = c.Vault.VaultAddress // Change to HTTPS if using SSL

	// Skipping TLS verification for self-signed certificates
	cfg.HttpClient.Transport = &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	client, err := vault.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("unable to initialize Vault client: %w", err)
	}

	if client == nil {
		return nil, fmt.Errorf("vault client is not initialized")
	}

	roleID := c.Vault.RoleID
	secretID := &auth.SecretID{FromString: c.Vault.SecretID}

	appRoleAuth, err := auth.NewAppRoleAuth(
		roleID,
		secretID,
	)
	if err != nil {
		return nil, fmt.Errorf("unable to initialize AppRole auth method: %w", err)
	}

	cAuth := client.Auth()
	if cAuth == nil {
		return nil, fmt.Errorf("unable to initialize vault auth")
	}

	authInfo, err := cAuth.Login(context.Background(), appRoleAuth)
	if err != nil {
		return nil, fmt.Errorf("unable to login to AppRole auth method: %w", err)
	}

	if authInfo == nil {
		return nil, fmt.Errorf("no auth info was returned after login")
	}

	return client, nil
}
