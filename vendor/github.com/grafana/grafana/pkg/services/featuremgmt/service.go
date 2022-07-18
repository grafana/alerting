package featuremgmt

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/grafana/grafana/pkg/infra/log"

	"github.com/grafana/grafana/pkg/models"
	"github.com/grafana/grafana/pkg/setting"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// The values are updated each time
	featureToggleInfo = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name:      "feature_toggles_info",
		Help:      "info metric that exposes what feature toggles are enabled or not",
		Namespace: "grafana",
	}, []string{"name"})
)

func ProvideManagerService(cfg *setting.Cfg, licensing models.Licensing) (*FeatureManager, error) {
	mgmt := &FeatureManager{
		isDevMod:  setting.Env != setting.Prod,
		licensing: licensing,
		flags:     make(map[string]*FeatureFlag, 30),
		enabled:   make(map[string]bool),
		log:       log.New("featuremgmt"),
	}

	// Register the standard flags
	mgmt.registerFlags(standardFeatureFlags...)

	// Load the flags from `custom.ini` files
	flags, err := setting.ReadFeatureTogglesFromInitFile(cfg.Raw.Section("feature_toggles"))
	if err != nil {
		return mgmt, err
	}
	for key, val := range flags {
		flag, ok := mgmt.flags[key]
		if !ok {
			flag = &FeatureFlag{
				Name:  key,
				State: FeatureStateUnknown,
			}
			mgmt.flags[key] = flag
		}
		flag.Expression = fmt.Sprintf("%t", val) // true | false
	}

	// Load config settings
	configfile := filepath.Join(cfg.HomePath, "conf", "features.yaml")
	if _, err := os.Stat(configfile); err == nil {
		mgmt.log.Info("[experimental] loading features from config file", "path", configfile)
		mgmt.config = configfile
		err = mgmt.readFile()
		if err != nil {
			return mgmt, err
		}
	}

	// update the values
	mgmt.update()

	// Minimum approach to avoid circular dependency
	cfg.IsFeatureToggleEnabled = mgmt.IsEnabled
	return mgmt, nil
}

// ProvideToggles allows read-only access to the feature state
func ProvideToggles(mgmt *FeatureManager) FeatureToggles {
	return mgmt
}
