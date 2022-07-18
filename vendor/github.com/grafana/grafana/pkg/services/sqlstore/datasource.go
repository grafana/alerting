package sqlstore

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"xorm.io/xorm"

	"github.com/grafana/grafana/pkg/components/simplejson"
	"github.com/grafana/grafana/pkg/events"
	"github.com/grafana/grafana/pkg/infra/metrics"
	ac "github.com/grafana/grafana/pkg/services/accesscontrol"
	"github.com/grafana/grafana/pkg/services/datasources"
	"github.com/grafana/grafana/pkg/util"
)

// GetDataSource adds a datasource to the query model by querying by org_id as well as
// either uid (preferred), id, or name and is added to the bus.
func (ss *SQLStore) GetDataSource(ctx context.Context, query *datasources.GetDataSourceQuery) error {
	metrics.MDBDataSourceQueryByID.Inc()

	return ss.WithDbSession(ctx, func(sess *DBSession) error {
		return ss.getDataSource(ctx, query, sess)
	})
}

func (ss *SQLStore) getDataSource(ctx context.Context, query *datasources.GetDataSourceQuery, sess *DBSession) error {
	if query.OrgId == 0 || (query.Id == 0 && len(query.Name) == 0 && len(query.Uid) == 0) {
		return datasources.ErrDataSourceIdentifierNotSet
	}

	datasource := &datasources.DataSource{Name: query.Name, OrgId: query.OrgId, Id: query.Id, Uid: query.Uid}
	has, err := sess.Get(datasource)

	if err != nil {
		sqlog.Error("Failed getting data source", "err", err, "uid", query.Uid, "id", query.Id, "name", query.Name, "orgId", query.OrgId)
		return err
	} else if !has {
		return datasources.ErrDataSourceNotFound
	}

	query.Result = datasource

	return nil
}

func (ss *SQLStore) GetDataSources(ctx context.Context, query *datasources.GetDataSourcesQuery) error {
	var sess *xorm.Session
	return ss.WithDbSession(ctx, func(dbSess *DBSession) error {
		if query.DataSourceLimit <= 0 {
			sess = dbSess.Where("org_id=?", query.OrgId).Asc("name")
		} else {
			sess = dbSess.Limit(query.DataSourceLimit, 0).Where("org_id=?", query.OrgId).Asc("name")
		}

		query.Result = make([]*datasources.DataSource, 0)
		return sess.Find(&query.Result)
	})
}

func (ss *SQLStore) GetAllDataSources(ctx context.Context, query *datasources.GetAllDataSourcesQuery) error {
	return ss.WithDbSession(ctx, func(sess *DBSession) error {
		query.Result = make([]*datasources.DataSource, 0)
		return sess.Asc("name").Find(&query.Result)
	})
}

// GetDataSourcesByType returns all datasources for a given type or an error if the specified type is an empty string
func (ss *SQLStore) GetDataSourcesByType(ctx context.Context, query *datasources.GetDataSourcesByTypeQuery) error {
	if query.Type == "" {
		return fmt.Errorf("datasource type cannot be empty")
	}

	query.Result = make([]*datasources.DataSource, 0)
	return ss.WithDbSession(ctx, func(sess *DBSession) error {
		return sess.Where("type=?", query.Type).Asc("id").Find(&query.Result)
	})
}

// GetDefaultDataSource is used to get the default datasource of organization
func (ss *SQLStore) GetDefaultDataSource(ctx context.Context, query *datasources.GetDefaultDataSourceQuery) error {
	datasource := datasources.DataSource{}
	return ss.WithDbSession(ctx, func(sess *DBSession) error {
		exists, err := sess.Where("org_id=? AND is_default=?", query.OrgId, true).Get(&datasource)

		if !exists {
			return datasources.ErrDataSourceNotFound
		}

		query.Result = &datasource
		return err
	})
}

// DeleteDataSource removes a datasource by org_id as well as either uid (preferred), id, or name
// and is added to the bus. It also removes permissions related to the datasource.
func (ss *SQLStore) DeleteDataSource(ctx context.Context, cmd *datasources.DeleteDataSourceCommand) error {
	return ss.WithTransactionalDbSession(ctx, func(sess *DBSession) error {
		dsQuery := &datasources.GetDataSourceQuery{Id: cmd.ID, Uid: cmd.UID, Name: cmd.Name, OrgId: cmd.OrgID}
		errGettingDS := ss.getDataSource(ctx, dsQuery, sess)

		if errGettingDS != nil && !errors.Is(errGettingDS, datasources.ErrDataSourceNotFound) {
			return errGettingDS
		}

		ds := dsQuery.Result
		if ds != nil {
			// Delete the data source
			result, err := sess.Exec("DELETE FROM data_source WHERE org_id=? AND id=?", ds.OrgId, ds.Id)
			if err != nil {
				return err
			}

			cmd.DeletedDatasourcesCount, _ = result.RowsAffected()

			// Remove associated AccessControl permissions
			if _, errDeletingPerms := sess.Exec("DELETE FROM permission WHERE scope=?",
				ac.Scope("datasources", "id", fmt.Sprint(dsQuery.Result.Id))); errDeletingPerms != nil {
				return errDeletingPerms
			}
		}

		if cmd.UpdateSecretFn != nil {
			if err := cmd.UpdateSecretFn(); err != nil {
				sqlog.Error("Failed to update datasource secrets -- rolling back update", "UID", cmd.UID, "name", cmd.Name, "orgId", cmd.OrgID)
				return err
			}
		}

		// Publish data source deletion event
		sess.publishAfterCommit(&events.DataSourceDeleted{
			Timestamp: time.Now(),
			Name:      cmd.Name,
			ID:        cmd.ID,
			UID:       cmd.UID,
			OrgID:     cmd.OrgID,
		})

		return nil
	})
}

func (ss *SQLStore) AddDataSource(ctx context.Context, cmd *datasources.AddDataSourceCommand) error {
	return ss.WithTransactionalDbSession(ctx, func(sess *DBSession) error {
		existing := datasources.DataSource{OrgId: cmd.OrgId, Name: cmd.Name}
		has, _ := sess.Get(&existing)

		if has {
			return datasources.ErrDataSourceNameExists
		}

		if cmd.JsonData == nil {
			cmd.JsonData = simplejson.New()
		}

		if cmd.Uid == "" {
			uid, err := generateNewDatasourceUid(sess, cmd.OrgId)
			if err != nil {
				return fmt.Errorf("failed to generate UID for datasource %q: %w", cmd.Name, err)
			}
			cmd.Uid = uid
		}

		ds := &datasources.DataSource{
			OrgId:           cmd.OrgId,
			Name:            cmd.Name,
			Type:            cmd.Type,
			Access:          cmd.Access,
			Url:             cmd.Url,
			User:            cmd.User,
			Database:        cmd.Database,
			IsDefault:       cmd.IsDefault,
			BasicAuth:       cmd.BasicAuth,
			BasicAuthUser:   cmd.BasicAuthUser,
			WithCredentials: cmd.WithCredentials,
			JsonData:        cmd.JsonData,
			SecureJsonData:  cmd.EncryptedSecureJsonData,
			Created:         time.Now(),
			Updated:         time.Now(),
			Version:         1,
			ReadOnly:        cmd.ReadOnly,
			Uid:             cmd.Uid,
		}

		if _, err := sess.Insert(ds); err != nil {
			if dialect.IsUniqueConstraintViolation(err) && strings.Contains(strings.ToLower(dialect.ErrorMessage(err)), "uid") {
				return datasources.ErrDataSourceUidExists
			}
			return err
		}
		if err := updateIsDefaultFlag(ds, sess); err != nil {
			return err
		}

		if cmd.UpdateSecretFn != nil {
			if err := cmd.UpdateSecretFn(); err != nil {
				sqlog.Error("Failed to update datasource secrets -- rolling back update", "name", cmd.Name, "type", cmd.Type, "orgId", cmd.OrgId)
				return err
			}
		}

		cmd.Result = ds

		sess.publishAfterCommit(&events.DataSourceCreated{
			Timestamp: time.Now(),
			Name:      cmd.Name,
			ID:        ds.Id,
			UID:       cmd.Uid,
			OrgID:     cmd.OrgId,
		})
		return nil
	})
}

func updateIsDefaultFlag(ds *datasources.DataSource, sess *DBSession) error {
	// Handle is default flag
	if ds.IsDefault {
		rawSQL := "UPDATE data_source SET is_default=? WHERE org_id=? AND id <> ?"
		if _, err := sess.Exec(rawSQL, false, ds.OrgId, ds.Id); err != nil {
			return err
		}
	}
	return nil
}

func (ss *SQLStore) UpdateDataSource(ctx context.Context, cmd *datasources.UpdateDataSourceCommand) error {
	return ss.WithTransactionalDbSession(ctx, func(sess *DBSession) error {
		if cmd.JsonData == nil {
			cmd.JsonData = simplejson.New()
		}

		ds := &datasources.DataSource{
			Id:              cmd.Id,
			OrgId:           cmd.OrgId,
			Name:            cmd.Name,
			Type:            cmd.Type,
			Access:          cmd.Access,
			Url:             cmd.Url,
			User:            cmd.User,
			Database:        cmd.Database,
			IsDefault:       cmd.IsDefault,
			BasicAuth:       cmd.BasicAuth,
			BasicAuthUser:   cmd.BasicAuthUser,
			WithCredentials: cmd.WithCredentials,
			JsonData:        cmd.JsonData,
			SecureJsonData:  cmd.EncryptedSecureJsonData,
			Updated:         time.Now(),
			ReadOnly:        cmd.ReadOnly,
			Version:         cmd.Version + 1,
			Uid:             cmd.Uid,
		}

		sess.UseBool("is_default")
		sess.UseBool("basic_auth")
		sess.UseBool("with_credentials")
		sess.UseBool("read_only")
		// Make sure password are zeroed out if empty. We do this as we want to migrate passwords from
		// plain text fields to SecureJsonData.
		sess.MustCols("password")
		sess.MustCols("basic_auth_password")
		sess.MustCols("user")
		// Make sure secure json data is zeroed out if empty. We do this as we want to migrate secrets from
		// secure json data to the unified secrets table.
		sess.MustCols("secure_json_data")

		var updateSession *xorm.Session
		if cmd.Version != 0 {
			// the reason we allow cmd.version > db.version is make it possible for people to force
			// updates to datasources using the datasource.yaml file without knowing exactly what version
			// a datasource have in the db.
			updateSession = sess.Where("id=? and org_id=? and version < ?", ds.Id, ds.OrgId, ds.Version)
		} else {
			updateSession = sess.Where("id=? and org_id=?", ds.Id, ds.OrgId)
		}

		affected, err := updateSession.Update(ds)
		if err != nil {
			return err
		}

		if affected == 0 {
			return datasources.ErrDataSourceUpdatingOldVersion
		}

		err = updateIsDefaultFlag(ds, sess)

		if cmd.UpdateSecretFn != nil {
			if err := cmd.UpdateSecretFn(); err != nil {
				sqlog.Error("Failed to update datasource secrets -- rolling back update", "UID", cmd.Uid, "name", cmd.Name, "type", cmd.Type, "orgId", cmd.OrgId)
				return err
			}
		}

		cmd.Result = ds
		return err
	})
}

func generateNewDatasourceUid(sess *DBSession, orgId int64) (string, error) {
	for i := 0; i < 3; i++ {
		uid := generateNewUid()

		exists, err := sess.Where("org_id=? AND uid=?", orgId, uid).Get(&datasources.DataSource{})
		if err != nil {
			return "", err
		}

		if !exists {
			return uid, nil
		}
	}

	return "", datasources.ErrDataSourceFailedGenerateUniqueUid
}

var generateNewUid func() string = util.GenerateShortUID
