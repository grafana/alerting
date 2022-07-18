package accesscontrol

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/grafana/grafana/pkg/models"
	"github.com/grafana/grafana/pkg/setting"
	"github.com/grafana/grafana/pkg/util"
	"github.com/grafana/grafana/pkg/web"
)

func Middleware(ac AccessControl) func(web.Handler, Evaluator) web.Handler {
	return func(fallback web.Handler, evaluator Evaluator) web.Handler {
		if ac.IsDisabled() {
			return fallback
		}

		return func(c *models.ReqContext) {
			authorize(c, ac, c.SignedInUser, evaluator)
		}
	}
}

func authorize(c *models.ReqContext, ac AccessControl, user *models.SignedInUser, evaluator Evaluator) {
	injected, err := evaluator.MutateScopes(c.Req.Context(), ScopeInjector(ScopeParams{
		OrgID:     c.OrgId,
		URLParams: web.Params(c.Req),
	}))
	if err != nil {
		c.JsonApiErr(http.StatusInternalServerError, "Internal server error", err)
		return
	}

	hasAccess, err := ac.Evaluate(c.Req.Context(), user, injected)
	if !hasAccess || err != nil {
		deny(c, injected, err)
		return
	}
}

func deny(c *models.ReqContext, evaluator Evaluator, err error) {
	id := newID()
	if err != nil {
		c.Logger.Error("Error from access control system", "error", err, "accessErrorID", id)
	} else {
		c.Logger.Info(
			"Access denied",
			"userID", c.UserId,
			"accessErrorID", id,
			"permissions", evaluator.GoString(),
		)
	}

	if !c.IsApiRequest() {
		// TODO(emil): I'd like to show a message after this redirect, not sure how that can be done?
		c.Redirect(setting.AppSubUrl + "/")
		return
	}

	message := ""
	if evaluator != nil {
		message = evaluator.String()
	}

	// If the user triggers an error in the access control system, we
	// don't want the user to be aware of that, so the user gets the
	// same information from the system regardless of if it's an
	// internal server error or access denied.
	c.JSON(http.StatusForbidden, map[string]string{
		"title":         "Access denied", // the component needs to pick this up
		"message":       fmt.Sprintf("You'll need additional permissions to perform this action. Permissions needed: %s", message),
		"accessErrorId": id,
	})
}

func newID() string {
	// Less ambiguity than alphanumerical.
	numerical := []byte("0123456789")
	id, err := util.GetRandomString(10, numerical...)
	if err != nil {
		// this should not happen, but if it does, a timestamp is as
		// useful as anything.
		id = fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return "ACE" + id
}

type OrgIDGetter func(c *models.ReqContext) (int64, error)
type userCache interface {
	GetSignedInUserWithCacheCtx(ctx context.Context, query *models.GetSignedInUserQuery) error
}

func AuthorizeInOrgMiddleware(ac AccessControl, cache userCache) func(web.Handler, OrgIDGetter, Evaluator) web.Handler {
	return func(fallback web.Handler, getTargetOrg OrgIDGetter, evaluator Evaluator) web.Handler {
		if ac.IsDisabled() {
			return fallback
		}

		return func(c *models.ReqContext) {
			// using a copy of the user not to modify the signedInUser, yet perform the permission evaluation in another org
			userCopy := *(c.SignedInUser)
			orgID, err := getTargetOrg(c)
			if err != nil {
				deny(c, nil, fmt.Errorf("failed to get target org: %w", err))
				return
			}
			if orgID == GlobalOrgID {
				userCopy.OrgId = orgID
				userCopy.OrgName = ""
				userCopy.OrgRole = ""
			} else {
				query := models.GetSignedInUserQuery{UserId: c.UserId, OrgId: orgID}
				if err := cache.GetSignedInUserWithCacheCtx(c.Req.Context(), &query); err != nil {
					deny(c, nil, fmt.Errorf("failed to authenticate user in target org: %w", err))
					return
				}
				userCopy.OrgId = query.Result.OrgId
				userCopy.OrgName = query.Result.OrgName
				userCopy.OrgRole = query.Result.OrgRole
			}

			authorize(c, ac, &userCopy, evaluator)

			// Set the sign-ed in user permissions in that org
			c.SignedInUser.Permissions = userCopy.Permissions
		}
	}
}

func UseOrgFromContextParams(c *models.ReqContext) (int64, error) {
	orgID, err := strconv.ParseInt(web.Params(c.Req)[":orgId"], 10, 64)

	// Special case of macaron handling invalid params
	if orgID == 0 || err != nil {
		return 0, models.ErrOrgNotFound
	}

	return orgID, nil
}

func UseGlobalOrg(c *models.ReqContext) (int64, error) {
	return GlobalOrgID, nil
}

func LoadPermissionsMiddleware(ac AccessControl) web.Handler {
	return func(c *models.ReqContext) {
		if ac.IsDisabled() {
			return
		}

		permissions, err := ac.GetUserPermissions(c.Req.Context(), c.SignedInUser,
			Options{ReloadCache: false})
		if err != nil {
			c.JsonApiErr(http.StatusForbidden, "", err)
			return
		}

		if c.SignedInUser.Permissions == nil {
			c.SignedInUser.Permissions = make(map[int64]map[string][]string)
		}
		c.SignedInUser.Permissions[c.OrgId] = GroupScopesByAction(permissions)
	}
}
