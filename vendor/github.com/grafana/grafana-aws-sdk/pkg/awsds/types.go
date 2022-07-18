package awsds

import (
	"time"

	"github.com/aws/aws-sdk-go/aws/session"
)

// AmazonSessionProvider will return a session (perhaps cached) for given region and settings
type AmazonSessionProvider func(region string, s AWSDatasourceSettings) (*session.Session, error)

// AuthSettings defines whether certain auth types and provider can be used or not
type AuthSettings struct {
	AllowedAuthProviders []string
	AssumeRoleEnabled    bool
	SessionDuration      *time.Duration
}
