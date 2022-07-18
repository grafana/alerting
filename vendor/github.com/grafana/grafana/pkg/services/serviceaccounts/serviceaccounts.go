package serviceaccounts

import (
	"context"

	"github.com/grafana/grafana/pkg/models"
)

// this should reflect the api
type Service interface {
	CreateServiceAccount(ctx context.Context, orgID int64, saForm *CreateServiceAccountForm) (*ServiceAccountDTO, error)
	DeleteServiceAccount(ctx context.Context, orgID, serviceAccountID int64) error
	RetrieveServiceAccountIdByName(ctx context.Context, orgID int64, name string) (int64, error)
}

type Store interface {
	CreateServiceAccount(ctx context.Context, orgID int64, saForm *CreateServiceAccountForm) (*ServiceAccountDTO, error)
	SearchOrgServiceAccounts(ctx context.Context, orgID int64, query string, filter ServiceAccountFilter, page int, limit int,
		signedInUser *models.SignedInUser) (*SearchServiceAccountsResult, error)
	UpdateServiceAccount(ctx context.Context, orgID, serviceAccountID int64,
		saForm *UpdateServiceAccountForm) (*ServiceAccountProfileDTO, error)
	RetrieveServiceAccount(ctx context.Context, orgID, serviceAccountID int64) (*ServiceAccountProfileDTO, error)
	RetrieveServiceAccountIdByName(ctx context.Context, orgID int64, name string) (int64, error)
	DeleteServiceAccount(ctx context.Context, orgID, serviceAccountID int64) error
	GetAPIKeysMigrationStatus(ctx context.Context, orgID int64) (*APIKeysMigrationStatus, error)
	HideApiKeysTab(ctx context.Context, orgID int64) error
	MigrateApiKeysToServiceAccounts(ctx context.Context, orgID int64) error
	MigrateApiKey(ctx context.Context, orgID int64, keyId int64) error
	RevertApiKey(ctx context.Context, keyId int64) error
	ListTokens(ctx context.Context, orgID int64, serviceAccount int64) ([]*models.ApiKey, error)
	DeleteServiceAccountToken(ctx context.Context, orgID, serviceAccountID, tokenID int64) error
	AddServiceAccountToken(ctx context.Context, serviceAccountID int64, cmd *AddServiceAccountTokenCommand) error
	GetUsageMetrics(ctx context.Context) (map[string]interface{}, error)
	RunMetricsCollection(ctx context.Context) error
}
