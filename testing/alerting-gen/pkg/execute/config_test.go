package execute

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name           string
		config         Config
		errMsg         string
		expectedConfig Config
	}{
		{
			name: "negative alert count",
			config: Config{
				NumAlerting: -5,
			},
			errMsg: "alert rule count cannot be negative",
		},
		{
			name: "negative recording rule count",
			config: Config{
				NumRecording: -10,
			},
			errMsg: "recording rule count cannot be negative",
		},
		{
			name: "negative rules per group",
			config: Config{
				RulesPerGroup: -5,
			},
			errMsg: "rules per group cannot be negative",
		},
		{
			name: "negative groups per folder",
			config: Config{
				GroupsPerFolder: -3,
			},
			errMsg: "groups per folder cannot be negative",
		},
		{
			name: "negative evaluation interval",
			config: Config{
				EvalInterval: -100,
			},
			errMsg: "evaluation interval cannot be negative",
		},
		{
			name: "negative org ID",
			config: Config{
				UploadOptions: UploadOptions{
					OrgID: -5,
				},
			},
			errMsg: "org ID cannot be negative",
		},
		{
			name: "negative folder count",
			config: Config{
				UploadOptions: UploadOptions{
					NumFolders: -2,
				},
			},
			errMsg: "folder count cannot be negative",
		},
		{
			name: "negative concurrency",
			config: Config{
				UploadOptions: UploadOptions{
					Concurrency: -10,
				},
			},
			errMsg: "concurrency cannot be negative",
		},
		{
			name: "no alert or recording rules",
			config: Config{
				UploadOptions: UploadOptions{
					FolderUIDsCSV: "folder1",
				},
			},
			errMsg: "no alert/recording rules to create",
		},
		{
			name: "no folder UIDs or folder count",
			config: Config{
				NumAlerting: 10,
			},
			errMsg: "can't calculate desired folder count with the provided configuration (rule count, rules per group, groups per folder)",
		},
		{
			name: "both folder UIDs and folder count provided",
			config: Config{
				NumAlerting: 10,
				UploadOptions: UploadOptions{
					FolderUIDsCSV: "folder1,folder2",
					NumFolders:    3,
				},
			},
			errMsg: "can't have folder UIDs and folder count",
		},
		{
			name: "valid config with both rule types",
			config: Config{
				NumAlerting:   25,
				NumRecording:  25,
				RulesPerGroup: 10,
				UploadOptions: UploadOptions{
					FolderUIDsCSV: "default",
				},
			},
			expectedConfig: Config{
				NumAlerting:     25,
				NumRecording:    25,
				QueryDS:         "grafanacloud-prom",
				WriteDS:         "grafanacloud-prom",
				RulesPerGroup:   10,
				GroupsPerFolder: 5,
				UploadOptions: UploadOptions{
					OrgID:       1,
					Concurrency: 1,
					FolderUIDs:  []string{"default"},
				},
			},
		},
		{
			name: "dynamic GroupsPerFolder when zero",
			config: Config{
				NumAlerting:   10,
				RulesPerGroup: 5,
				UploadOptions: UploadOptions{
					FolderUIDsCSV: "folder1",
				},
			},
			expectedConfig: Config{
				NumAlerting:     10,
				QueryDS:         "grafanacloud-prom",
				WriteDS:         "grafanacloud-prom",
				RulesPerGroup:   5,
				GroupsPerFolder: 2,
				UploadOptions: UploadOptions{
					OrgID:       1,
					Concurrency: 1,
					FolderUIDs:  []string{"folder1"},
				},
			},
		},
		{
			name: "dynamic RulesPerGroup when zero",
			config: Config{
				NumAlerting:     20,
				GroupsPerFolder: 2,
				UploadOptions: UploadOptions{
					FolderUIDsCSV: "folder1",
				},
			},
			expectedConfig: Config{
				NumAlerting:     20,
				QueryDS:         "grafanacloud-prom",
				WriteDS:         "grafanacloud-prom",
				RulesPerGroup:   10,
				GroupsPerFolder: 2,
				UploadOptions: UploadOptions{
					OrgID:       1,
					Concurrency: 1,
					FolderUIDs:  []string{"folder1"},
				},
			},
		},
		{
			name: "both RulesPerGroup and GroupsPerFolder calculated",
			config: Config{
				NumAlerting: 15,
				UploadOptions: UploadOptions{
					FolderUIDsCSV: "folder1",
				},
			},
			expectedConfig: Config{
				NumAlerting:     15,
				QueryDS:         "grafanacloud-prom",
				WriteDS:         "grafanacloud-prom",
				RulesPerGroup:   15,
				GroupsPerFolder: 1,
				UploadOptions: UploadOptions{
					OrgID:       1,
					Concurrency: 1,
					FolderUIDs:  []string{"folder1"},
				},
			},
		},
		{
			name: "dynamic NumFolders",
			config: Config{
				NumAlerting:     100,
				RulesPerGroup:   10,
				GroupsPerFolder: 5,
				UploadOptions:   UploadOptions{},
			},
			expectedConfig: Config{
				NumAlerting:     100,
				QueryDS:         "grafanacloud-prom",
				WriteDS:         "grafanacloud-prom",
				RulesPerGroup:   10,
				GroupsPerFolder: 5,
				UploadOptions: UploadOptions{
					OrgID:       1,
					NumFolders:  2,
					Concurrency: 1,
				},
			},
		},
		{
			name: "rounding up folder count",
			config: Config{
				NumAlerting:     50,
				NumRecording:    50,
				RulesPerGroup:   10,
				GroupsPerFolder: 3,
				UploadOptions:   UploadOptions{},
			},
			expectedConfig: Config{
				NumAlerting:     50,
				NumRecording:    50,
				QueryDS:         "grafanacloud-prom",
				WriteDS:         "grafanacloud-prom",
				RulesPerGroup:   10,
				GroupsPerFolder: 3,
				UploadOptions: UploadOptions{
					OrgID:       1,
					NumFolders:  4,
					Concurrency: 1,
				},
			},
		},
		{
			name: "rounding up rules per group",
			config: Config{
				NumAlerting:     100,
				GroupsPerFolder: 5,
				UploadOptions: UploadOptions{
					NumFolders: 4,
				},
			},
			expectedConfig: Config{
				NumAlerting:     100,
				QueryDS:         "grafanacloud-prom",
				WriteDS:         "grafanacloud-prom",
				RulesPerGroup:   5,
				GroupsPerFolder: 5,
				UploadOptions: UploadOptions{
					OrgID:       1,
					NumFolders:  4,
					Concurrency: 1,
				},
			},
		},
		{
			name: "valid config with multiple folder UIDs",
			config: Config{
				NumAlerting:     100,
				RulesPerGroup:   10,
				GroupsPerFolder: 5,
				UploadOptions: UploadOptions{
					FolderUIDsCSV: "folder1,folder2,folder3",
				},
			},
			expectedConfig: Config{
				NumAlerting:     100,
				QueryDS:         "grafanacloud-prom",
				WriteDS:         "grafanacloud-prom",
				RulesPerGroup:   10,
				GroupsPerFolder: 5,
				UploadOptions: UploadOptions{
					OrgID:       1,
					Concurrency: 1,
					FolderUIDs:  []string{"folder1", "folder2", "folder3"},
				},
			},
		},
		{
			name: "valid config with folder count",
			config: Config{
				NumRecording:    50,
				RulesPerGroup:   5,
				GroupsPerFolder: 2,
				UploadOptions: UploadOptions{
					NumFolders: 10,
				},
			},
			expectedConfig: Config{
				NumRecording:    50,
				QueryDS:         "grafanacloud-prom",
				WriteDS:         "grafanacloud-prom",
				RulesPerGroup:   5,
				GroupsPerFolder: 2,
				UploadOptions: UploadOptions{
					OrgID:       1,
					NumFolders:  10,
					Concurrency: 1,
				},
			},
		},
		{
			name: "empty folder UIDs, no folder count",
			config: Config{
				NumAlerting: 10,
			},
			errMsg: "can't calculate desired folder count with the provided configuration (rule count, rules per group, groups per folder)",
		},
		{
			name: "explicit RulesPerGroup too small",
			config: Config{
				NumAlerting:     100,
				RulesPerGroup:   10,
				GroupsPerFolder: 2,
				UploadOptions: UploadOptions{
					NumFolders: 1,
				},
			},
			errMsg: "insufficient capacity: need space for 100 rules but only have capacity for 20 (RulesPerGroup=10, GroupsPerFolder=2, folders=1)",
		},
		{
			name: "folder UIDs with spaces",
			config: Config{
				NumAlerting:     30,
				GroupsPerFolder: 3,
				UploadOptions: UploadOptions{
					FolderUIDsCSV: "folder1, folder2 , folder3",
				},
			},
			expectedConfig: Config{
				NumAlerting:     30,
				QueryDS:         "grafanacloud-prom",
				WriteDS:         "grafanacloud-prom",
				RulesPerGroup:   4,
				GroupsPerFolder: 3,
				UploadOptions: UploadOptions{
					OrgID:       1,
					Concurrency: 1,
					FolderUIDs:  []string{"folder1", "folder2", "folder3"},
				},
			},
		},
		{
			name: "folder UIDs with empty entries",
			config: Config{
				NumAlerting:     20,
				GroupsPerFolder: 2,
				UploadOptions: UploadOptions{
					FolderUIDsCSV: "folder1,,folder2,",
				},
			},
			expectedConfig: Config{
				NumAlerting:     20,
				QueryDS:         "grafanacloud-prom",
				WriteDS:         "grafanacloud-prom",
				RulesPerGroup:   5,
				GroupsPerFolder: 2,
				UploadOptions: UploadOptions{
					OrgID:       1,
					Concurrency: 1,
					FolderUIDs:  []string{"folder1", "folder2"},
				},
			},
		},
		{
			name: "GroupsPerFolder defaults to 1 when RulesPerGroup not set",
			config: Config{
				NumAlerting: 20,
				UploadOptions: UploadOptions{
					NumFolders: 2,
				},
			},
			expectedConfig: Config{
				NumAlerting:     20,
				QueryDS:         "grafanacloud-prom",
				WriteDS:         "grafanacloud-prom",
				RulesPerGroup:   10,
				GroupsPerFolder: 1,
				UploadOptions: UploadOptions{
					OrgID:       1,
					NumFolders:  2,
					Concurrency: 1,
				},
			},
		},
		{
			name: "folder UIDs with trailing comma",
			config: Config{
				NumAlerting:     30,
				GroupsPerFolder: 3,
				UploadOptions: UploadOptions{
					FolderUIDsCSV: "f1,f2,f3,",
				},
			},
			expectedConfig: Config{
				NumAlerting:     30,
				QueryDS:         "grafanacloud-prom",
				WriteDS:         "grafanacloud-prom",
				RulesPerGroup:   4,
				GroupsPerFolder: 3,
				UploadOptions: UploadOptions{
					OrgID:       1,
					Concurrency: 1,
					FolderUIDs:  []string{"f1", "f2", "f3"},
				},
			},
		},
		{
			name: "GrafanaURL set but no credentials",
			config: Config{
				NumAlerting:     10,
				RulesPerGroup:   5,
				GroupsPerFolder: 2,
				UploadOptions: UploadOptions{
					GrafanaURL: "http://localhost:3000",
					NumFolders: 1,
				},
			},
			errMsg: "no username + password or token provided",
		},
		{
			name: "GrafanaURL with username and password",
			config: Config{
				NumAlerting:     10,
				RulesPerGroup:   5,
				GroupsPerFolder: 2,
				UploadOptions: UploadOptions{
					GrafanaURL: "http://localhost:3000",
					Username:   "admin",
					Password:   "admin",
					NumFolders: 1,
				},
			},
			expectedConfig: Config{
				NumAlerting:     10,
				QueryDS:         "grafanacloud-prom",
				WriteDS:         "grafanacloud-prom",
				RulesPerGroup:   5,
				GroupsPerFolder: 2,
				UploadOptions: UploadOptions{
					GrafanaURL:  "http://localhost:3000",
					Username:    "admin",
					Password:    "admin",
					OrgID:       1,
					NumFolders:  1,
					Concurrency: 1,
				},
			},
		},
		{
			name: "GrafanaURL with token only",
			config: Config{
				NumAlerting:     20,
				RulesPerGroup:   10,
				GroupsPerFolder: 1,
				UploadOptions: UploadOptions{
					GrafanaURL: "http://localhost:3000",
					Token:      "test_token",
					NumFolders: 2,
				},
			},
			expectedConfig: Config{
				NumAlerting:     20,
				QueryDS:         "grafanacloud-prom",
				WriteDS:         "grafanacloud-prom",
				RulesPerGroup:   10,
				GroupsPerFolder: 1,
				UploadOptions: UploadOptions{
					GrafanaURL:  "http://localhost:3000",
					Token:       "test_token",
					OrgID:       1,
					NumFolders:  2,
					Concurrency: 1,
				},
			},
		},
		{
			name: "GrafanaURL with password only (no username)",
			config: Config{
				NumAlerting:     10,
				RulesPerGroup:   5,
				GroupsPerFolder: 2,
				UploadOptions: UploadOptions{
					GrafanaURL: "http://localhost:3000",
					Password:   "admin",
					NumFolders: 1,
				},
			},
			errMsg: "no username + password or token provided",
		},
		{
			name: "no GrafanaURL, no credentials needed",
			config: Config{
				NumAlerting:     50,
				RulesPerGroup:   10,
				GroupsPerFolder: 5,
				UploadOptions: UploadOptions{
					NumFolders: 1,
				},
			},
			expectedConfig: Config{
				NumAlerting:     50,
				QueryDS:         "grafanacloud-prom",
				WriteDS:         "grafanacloud-prom",
				RulesPerGroup:   10,
				GroupsPerFolder: 5,
				UploadOptions: UploadOptions{
					OrgID:       1,
					NumFolders:  1,
					Concurrency: 1,
				},
			},
		},
		{
			name: "nuke without GrafanaURL",
			config: Config{
				NumAlerting: 10,
				UploadOptions: UploadOptions{
					Nuke:       true,
					NumFolders: 1,
				},
			},
			errMsg: "can't nuke an instance without a URL",
		},
		{
			name: "nuke with GrafanaURL and credentials, no rules",
			config: Config{
				UploadOptions: UploadOptions{
					Nuke:       true,
					GrafanaURL: "http://localhost:3000",
					Token:      "token123",
				},
			},
			expectedConfig: Config{
				QueryDS: "grafanacloud-prom",
				WriteDS: "grafanacloud-prom",
				UploadOptions: UploadOptions{
					Nuke:        true,
					GrafanaURL:  "http://localhost:3000",
					Token:       "token123",
					OrgID:       1,
					Concurrency: 1,
				},
			},
		},
		{
			name: "nuke with GrafanaURL and rules",
			config: Config{
				NumAlerting:     10,
				RulesPerGroup:   5,
				GroupsPerFolder: 2,
				UploadOptions: UploadOptions{
					Nuke:       true,
					GrafanaURL: "http://localhost:3000",
					Username:   "admin",
					Password:   "admin",
					NumFolders: 1,
				},
			},
			expectedConfig: Config{
				NumAlerting:     10,
				QueryDS:         "grafanacloud-prom",
				WriteDS:         "grafanacloud-prom",
				RulesPerGroup:   5,
				GroupsPerFolder: 2,
				UploadOptions: UploadOptions{
					Nuke:        true,
					GrafanaURL:  "http://localhost:3000",
					Username:    "admin",
					Password:    "admin",
					OrgID:       1,
					NumFolders:  1,
					Concurrency: 1,
				},
			},
		},
		{
			name: "Concurrency defaults to 1 when zero",
			config: Config{
				NumAlerting: 10,
				UploadOptions: UploadOptions{
					NumFolders: 1,
				},
			},
			expectedConfig: Config{
				NumAlerting:     10,
				QueryDS:         "grafanacloud-prom",
				WriteDS:         "grafanacloud-prom",
				RulesPerGroup:   10,
				GroupsPerFolder: 1,
				UploadOptions: UploadOptions{
					OrgID:       1,
					NumFolders:  1,
					Concurrency: 1,
				},
			},
		},
		{
			name: "Concurrency preserved when set to valid value",
			config: Config{
				NumAlerting: 10,
				UploadOptions: UploadOptions{
					NumFolders:  1,
					Concurrency: 20,
				},
			},
			expectedConfig: Config{
				NumAlerting:     10,
				QueryDS:         "grafanacloud-prom",
				WriteDS:         "grafanacloud-prom",
				RulesPerGroup:   10,
				GroupsPerFolder: 1,
				UploadOptions: UploadOptions{
					OrgID:       1,
					NumFolders:  1,
					Concurrency: 20,
				},
			},
		},
		{
			name: "Query data source set, write data source not set",
			config: Config{
				NumAlerting: 10,
				UploadOptions: UploadOptions{
					NumFolders: 1,
				},
				QueryDS: "test-ds",
			},
			expectedConfig: Config{
				NumAlerting:     10,
				QueryDS:         "test-ds",
				WriteDS:         "test-ds",
				RulesPerGroup:   10,
				GroupsPerFolder: 1,
				UploadOptions: UploadOptions{
					OrgID:       1,
					NumFolders:  1,
					Concurrency: 1,
				},
			},
		},
		{
			name: "Write data source set, query data source not set",
			config: Config{
				NumAlerting: 10,
				UploadOptions: UploadOptions{
					NumFolders: 1,
				},
				WriteDS: "test-ds",
			},
			expectedConfig: Config{
				NumAlerting:     10,
				QueryDS:         "grafanacloud-prom",
				WriteDS:         "test-ds",
				RulesPerGroup:   10,
				GroupsPerFolder: 1,
				UploadOptions: UploadOptions{
					OrgID:       1,
					NumFolders:  1,
					Concurrency: 1,
				},
			},
		},
		{
			name: "Write and query data sources set",
			config: Config{
				NumAlerting: 10,
				UploadOptions: UploadOptions{
					NumFolders: 1,
				},
				QueryDS: "test-ds-query",
				WriteDS: "test-ds-write",
			},
			expectedConfig: Config{
				NumAlerting:     10,
				QueryDS:         "test-ds-query",
				WriteDS:         "test-ds-write",
				RulesPerGroup:   10,
				GroupsPerFolder: 1,
				UploadOptions: UploadOptions{
					OrgID:       1,
					NumFolders:  1,
					Concurrency: 1,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.errMsg != "" {
				require.Error(t, err)
				require.Equal(t, tt.errMsg, err.Error())
				return
			}

			require.NoError(t, err)

			if tt.expectedConfig.Seed == 0 {
				// Ignore seed when comparing unless explicitly set.
				require.NotEqual(t, 0, tt.config.Seed)
				tt.config.Seed = 0
			}

			require.Equal(t, tt.expectedConfig, tt.config)
		})
	}
}
