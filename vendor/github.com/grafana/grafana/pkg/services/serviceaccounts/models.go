package serviceaccounts

import (
	"time"

	"github.com/grafana/grafana/pkg/models"
	"github.com/grafana/grafana/pkg/services/accesscontrol"
)

var (
	ScopeAll = "serviceaccounts:*"
	ScopeID  = accesscontrol.Scope("serviceaccounts", "id", accesscontrol.Parameter(":serviceAccountId"))
)

const (
	ActionRead             = "serviceaccounts:read"
	ActionWrite            = "serviceaccounts:write"
	ActionCreate           = "serviceaccounts:create"
	ActionDelete           = "serviceaccounts:delete"
	ActionPermissionsRead  = "serviceaccounts.permissions:read"
	ActionPermissionsWrite = "serviceaccounts.permissions:write"
)

type ServiceAccount struct {
	Id int64
}

type CreateServiceAccountForm struct {
	Name       string           `json:"name" binding:"Required"`
	Role       *models.RoleType `json:"role"`
	IsDisabled *bool            `json:"isDisabled"`
}

type UpdateServiceAccountForm struct {
	Name       *string          `json:"name"`
	Role       *models.RoleType `json:"role"`
	IsDisabled *bool            `json:"isDisabled"`
}

type ServiceAccountDTO struct {
	Id            int64           `json:"id" xorm:"user_id"`
	Name          string          `json:"name" xorm:"name"`
	Login         string          `json:"login" xorm:"login"`
	OrgId         int64           `json:"orgId" xorm:"org_id"`
	IsDisabled    bool            `json:"isDisabled" xorm:"is_disabled"`
	Role          string          `json:"role" xorm:"role"`
	Tokens        int64           `json:"tokens"`
	AvatarUrl     string          `json:"avatarUrl"`
	AccessControl map[string]bool `json:"accessControl,omitempty"`
}

type AddServiceAccountTokenCommand struct {
	Name          string         `json:"name" binding:"Required"`
	OrgId         int64          `json:"-"`
	Key           string         `json:"-"`
	SecondsToLive int64          `json:"secondsToLive"`
	Result        *models.ApiKey `json:"-"`
}

type SearchServiceAccountsResult struct {
	TotalCount      int64                `json:"totalCount"`
	ServiceAccounts []*ServiceAccountDTO `json:"serviceAccounts"`
	Page            int                  `json:"page"`
	PerPage         int                  `json:"perPage"`
}

type ServiceAccountProfileDTO struct {
	Id            int64           `json:"id" xorm:"user_id"`
	Name          string          `json:"name" xorm:"name"`
	Login         string          `json:"login" xorm:"login"`
	OrgId         int64           `json:"orgId" xorm:"org_id"`
	IsDisabled    bool            `json:"isDisabled" xorm:"is_disabled"`
	Created       time.Time       `json:"createdAt" xorm:"created"`
	Updated       time.Time       `json:"updatedAt" xorm:"updated"`
	AvatarUrl     string          `json:"avatarUrl" xorm:"-"`
	Role          string          `json:"role" xorm:"role"`
	Teams         []string        `json:"teams" xorm:"-"`
	Tokens        int64           `json:"tokens,omitempty"`
	AccessControl map[string]bool `json:"accessControl,omitempty" xorm:"-"`
}

type ServiceAccountFilter string // used for filtering

type APIKeysMigrationStatus struct {
	Migrated bool `json:"migrated"`
}

const (
	FilterOnlyExpiredTokens ServiceAccountFilter = "expiredTokens"
	FilterOnlyDisabled      ServiceAccountFilter = "disabled"
	FilterIncludeAll        ServiceAccountFilter = "all"
)
