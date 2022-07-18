// Package adapters contains plugin SDK adapters.
package adapters

import (
	"encoding/json"
	"fmt"

	"github.com/grafana/grafana-plugin-sdk-go/backend"

	"github.com/grafana/grafana/pkg/models"
	"github.com/grafana/grafana/pkg/services/datasources"
)

// ModelToInstanceSettings converts a datasources.DataSource to a backend.DataSourceInstanceSettings.
func ModelToInstanceSettings(ds *datasources.DataSource, decryptFn func(ds *datasources.DataSource) (map[string]string, error),
) (*backend.DataSourceInstanceSettings, error) {
	var jsonDataBytes json.RawMessage
	if ds.JsonData != nil {
		var err error
		jsonDataBytes, err = ds.JsonData.MarshalJSON()
		if err != nil {
			return nil, fmt.Errorf("failed to convert data source to instance settings: %w", err)
		}
	}
	decrypted, err := decryptFn(ds)
	if err != nil {
		return nil, err
	}

	return &backend.DataSourceInstanceSettings{
		ID:                      ds.Id,
		Name:                    ds.Name,
		URL:                     ds.Url,
		UID:                     ds.Uid,
		Database:                ds.Database,
		User:                    ds.User,
		BasicAuthEnabled:        ds.BasicAuth,
		BasicAuthUser:           ds.BasicAuthUser,
		JSONData:                jsonDataBytes,
		DecryptedSecureJSONData: decrypted,
		Updated:                 ds.Updated,
	}, err
}

// BackendUserFromSignedInUser converts Grafana's SignedInUser model
// to the backend plugin's model.
func BackendUserFromSignedInUser(su *models.SignedInUser) *backend.User {
	if su == nil {
		return nil
	}
	return &backend.User{
		Login: su.Login,
		Name:  su.Name,
		Email: su.Email,
		Role:  string(su.OrgRole),
	}
}
