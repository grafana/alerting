package templates

import (
	"testing"

	"github.com/prometheus/alertmanager/template"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alerting/logging"
	"github.com/grafana/alerting/models"
)

func TestParseAlertingListURL(t *testing.T) {
	testcases := []struct {
		name        string
		data        *Data
		expectedURI string
	}{
		{
			name: "without org annotation",
			data: &Data{
				ExternalURL: "http://localhost:3000",
			},
			expectedURI: "http://localhost:3000/alerting/list",
		},
		{
			name: "with org annotation",
			data: &Data{
				ExternalURL: "http://localhost:3000",
				CommonAnnotations: template.KV{
					models.OrgIDAnnotation: "1234",
				},
			},
			expectedURI: "http://localhost:3000/alerting/list?orgId=1234",
		},
		{
			name: "with invalid external URL",
			data: &Data{
				ExternalURL: `http%//invalid@url.com`,
			},
			expectedURI: "",
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			actualURI := parseAlertingListURL(tc.data, &logging.FakeLogger{})

			require.Equal(t, tc.expectedURI, actualURI)
		})
	}
}
