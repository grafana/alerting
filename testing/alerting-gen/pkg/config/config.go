package config

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
)

type Config struct {
	NumAlerting     int
	NumRecording    int
	QueryDS         string
	WriteDS         string
	RulesPerGroup   int
	GroupsPerFolder int
	EvalInterval    int64
	Seed            int64
	UploadOptions
}

type UploadOptions struct {
	GrafanaURL    string
	Username      string
	Password      string
	Token         string
	OrgID         int64
	FolderUIDsCSV string
	NumFolders    int
	Nuke          bool
	Concurrency   int

	FolderUIDs []string
}

// Validate validates the configuration and adds defaults.
func (c *Config) Validate() error {
	// Check for negative values.
	if c.NumAlerting < 0 {
		return errors.New("alert rule count cannot be negative")
	}
	if c.NumRecording < 0 {
		return errors.New("recording rule count cannot be negative")
	}
	if c.RulesPerGroup < 0 {
		return errors.New("rules per group cannot be negative")
	}
	if c.GroupsPerFolder < 0 {
		return errors.New("groups per folder cannot be negative")
	}
	if c.EvalInterval < 0 {
		return errors.New("evaluation interval cannot be negative")
	}
	if c.OrgID < 0 {
		return errors.New("org ID cannot be negative")
	}
	if c.NumFolders < 0 {
		return errors.New("folder count cannot be negative")
	}
	if c.Concurrency < 0 {
		return errors.New("concurrency cannot be negative")
	}

	// Add defaults.
	c.Concurrency = max(c.Concurrency, 1)
	c.OrgID = max(c.OrgID, 1)
	if c.Seed == 0 {
		c.Seed = time.Now().Unix()
	}
	if c.QueryDS == "" {
		c.QueryDS = "grafanacloud-prom"
	}
	if c.WriteDS == "" {
		c.WriteDS = "grafanacloud-prom"
	}

	// Validate Grafana credentials when URL is provided.
	if c.GrafanaURL != "" && c.Token == "" && (c.Username == "" || c.Password == "") {
		return errors.New("no username + password or token provided")
	}

	// Can't nuke without a URL.
	if c.Nuke && c.GrafanaURL == "" {
		return errors.New("can't nuke an instance without a URL")
	}

	if c.NumAlerting <= 0 && c.NumRecording <= 0 {
		// If we're just nuking without creating rules, we're done validating.
		if c.Nuke {
			return nil
		}
		// Otherwise, we need rules to create.
		return errors.New("no alert/recording rules to create")
	}

	if len(c.FolderUIDsCSV) > 0 {
		if c.NumFolders > 0 {
			// TODO: (Optional) Create missing folders.
			// If folderCount > len(FolderUIDs), create folders until we reach the desired folder count.
			return errors.New("can't have folder UIDs and folder count")
		}

		// Extract folder UIDs.
		for uid := range strings.SplitSeq(c.FolderUIDsCSV, ",") {
			if trimmed := strings.TrimSpace(uid); trimmed != "" {
				c.FolderUIDs = append(c.FolderUIDs, trimmed)
			}
		}
		c.FolderUIDsCSV = ""
	}

	folderCount := len(c.FolderUIDs)
	if folderCount == 0 {
		folderCount = c.NumFolders
	}
	ruleCount := c.NumAlerting + c.NumRecording

	if c.GroupsPerFolder <= 0 {
		// No groups per folder specified. Calculate it based on rules per group and folders.
		if folderCount > 0 && c.RulesPerGroup > 0 {
			capacityPerGroup := c.RulesPerGroup * folderCount
			c.GroupsPerFolder = int(math.Ceil(float64(ruleCount) / float64(capacityPerGroup)))
		} else {
			// Default to 1 group per folder if we can't calculate it.
			c.GroupsPerFolder = 1
		}
	}

	// No folder count specified. Calculate it based on rules and groups.
	if folderCount <= 0 && c.RulesPerGroup > 0 {
		capacityPerFolder := c.RulesPerGroup * c.GroupsPerFolder
		folderCount = int(math.Ceil(float64(ruleCount) / float64(capacityPerFolder)))
		c.NumFolders = folderCount
	}

	// At this point, we must have either a desired folder count or a list of folder UIDs.
	if c.NumFolders == 0 && len(c.FolderUIDs) == 0 {
		return errors.New("can't calculate desired folder count with the provided configuration (rule count, rules per group, groups per folder)")
	}

	if c.RulesPerGroup <= 0 {
		// Divide all rules among all groups and folders, round up.
		totalGroups := c.GroupsPerFolder * folderCount
		c.RulesPerGroup = int(math.Ceil(float64(ruleCount) / float64(totalGroups)))
	} else {
		// Validate that explicit RulesPerGroup provides sufficient capacity.
		totalCapacity := c.RulesPerGroup * c.GroupsPerFolder * folderCount
		if totalCapacity < ruleCount {
			return fmt.Errorf("insufficient capacity: need space for %d rules but only have capacity for %d (RulesPerGroup=%d, GroupsPerFolder=%d, folders=%d)",
				ruleCount, totalCapacity, c.RulesPerGroup, c.GroupsPerFolder, folderCount)
		}
	}

	return nil
}
