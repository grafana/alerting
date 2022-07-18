package awsds

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/gtime"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/ec2rolecreds"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
)

type envelope struct {
	session    *session.Session
	expiration time.Time
}

// SessionCache cache sessions for a while
type SessionCache struct {
	sessCache     map[string]envelope
	sessCacheLock sync.RWMutex
	authSettings  *AuthSettings
}

// NewSessionCache creates a new session cache using the default settings loaded from environment variables
func NewSessionCache() *SessionCache {
	return &SessionCache{
		sessCache:    map[string]envelope{},
		authSettings: ReadAuthSettingsFromEnvironmentVariables(),
	}
}

// AllowedAuthProvidersEnvVarKeyName is the string literal for the aws allowed auth providers environment variable key name
const AllowedAuthProvidersEnvVarKeyName = "AWS_AUTH_AllowedAuthProviders"

// AssumeRoleEnabledEnvVarKeyName is the string literal for the aws assume role enabled environment variable key name
const AssumeRoleEnabledEnvVarKeyName = "AWS_AUTH_AssumeRoleEnabled"

// SessionDurationEnvVarKeyName is the string literal for the session duration variable key name
const SessionDurationEnvVarKeyName = "AWS_AUTH_SESSION_DURATION"

func ReadAuthSettingsFromEnvironmentVariables() *AuthSettings {
	authSettings := &AuthSettings{}
	allowedAuthProviders := []string{}
	providers := os.Getenv(AllowedAuthProvidersEnvVarKeyName)
	for _, authProvider := range strings.Split(providers, ",") {
		authProvider = strings.TrimSpace(authProvider)
		if authProvider != "" {
			allowedAuthProviders = append(allowedAuthProviders, authProvider)
		}
	}

	if len(allowedAuthProviders) == 0 {
		allowedAuthProviders = []string{"default", "keys", "credentials"}
		backend.Logger.Warn("could not find allowed auth providers. falling back to 'default, keys, credentials'")
	}
	authSettings.AllowedAuthProviders = allowedAuthProviders

	assumeRoleEnabledString := os.Getenv(AssumeRoleEnabledEnvVarKeyName)
	if len(assumeRoleEnabledString) == 0 {
		backend.Logger.Warn("environment variable missing. falling back to enable assume role", "var", AssumeRoleEnabledEnvVarKeyName)
		assumeRoleEnabledString = "true"
	}

	var err error
	authSettings.AssumeRoleEnabled, err = strconv.ParseBool(assumeRoleEnabledString)
	if err != nil {
		backend.Logger.Error("could not parse env variable", "var", AssumeRoleEnabledEnvVarKeyName)
		authSettings.AssumeRoleEnabled = true
	}

	sessionDurationString := os.Getenv(SessionDurationEnvVarKeyName)
	if sessionDurationString != "" {
		sessionDuration, err := gtime.ParseDuration(sessionDurationString)
		if err != nil {
			backend.Logger.Error("could not parse env variable", "var", SessionDurationEnvVarKeyName)
		} else {
			authSettings.SessionDuration = &sessionDuration
		}
	}

	return authSettings
}

// Session factory.
// Stubbable by tests.
//nolint:gocritic
var newSession = func(cfgs ...*aws.Config) (*session.Session, error) {
	return session.NewSession(cfgs...)
}

// STS credentials factory.
// Stubbable by tests.
//nolint:gocritic
var newSTSCredentials = stscreds.NewCredentials

// EC2Metadata service factory.
// Stubbable by tests.
//nolint:gocritic
var newEC2Metadata = ec2metadata.New

// EC2 role credentials factory.
// Stubbable by tests.
var newEC2RoleCredentials = func(sess *session.Session) *credentials.Credentials {
	return credentials.NewCredentials(&ec2rolecreds.EC2RoleProvider{Client: ec2metadata.New(sess), ExpiryWindow: stscreds.DefaultDuration})
}

type SessionConfig struct {
	Settings      AWSDatasourceSettings
	HTTPClient    *http.Client
	UserAgentName *string
}

func isOptInRegion(region string) bool {
	// Opt-in region from https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/using-regions-availability-zones.html#concepts-available-regions
	regions := map[string]bool{
		"af-south-1":     true,
		"ap-east-1":      true,
		"ap-southeast-3": true,
		"eu-south-1":     true,
		"me-south-1":     true,
		// The rest of regions will return false
	}
	return regions[region]
}

// GetSession returns a session from the config and possible region overrides -- implements AmazonSessionProvider
func (sc *SessionCache) GetSession(c SessionConfig) (*session.Session, error) {
	if c.Settings.Region == "" && c.Settings.DefaultRegion != "" {
		// DefaultRegion is deprecated, Region should be used instead
		c.Settings.Region = c.Settings.DefaultRegion
	}
	authTypeAllowed := false
	for _, provider := range sc.authSettings.AllowedAuthProviders {
		if provider == c.Settings.AuthType.String() {
			authTypeAllowed = true
			break
		}
	}
	if !authTypeAllowed {
		return nil, fmt.Errorf("attempting to use an auth type that is not allowed: %q", c.Settings.AuthType.String())
	}

	if c.Settings.AssumeRoleARN != "" && !sc.authSettings.AssumeRoleEnabled {
		return nil, fmt.Errorf("attempting to use assume role (ARN) which is disabled in grafana.ini")
	}

	bldr := strings.Builder{}
	for i, s := range []string{
		c.Settings.AuthType.String(), c.Settings.AccessKey, c.Settings.Profile, c.Settings.AssumeRoleARN, c.Settings.Region, c.Settings.Endpoint,
	} {
		if i != 0 {
			bldr.WriteString(":")
		}
		bldr.WriteString(strings.ReplaceAll(s, ":", `\:`))
	}
	cacheKey := bldr.String()

	sc.sessCacheLock.RLock()
	if env, ok := sc.sessCache[cacheKey]; ok {
		if env.expiration.After(time.Now().UTC()) {
			sc.sessCacheLock.RUnlock()
			return env.session, nil
		}
	}
	sc.sessCacheLock.RUnlock()

	cfgs := []*aws.Config{
		{
			CredentialsChainVerboseErrors: aws.Bool(true),
			HTTPClient:                    c.HTTPClient,
		},
	}

	var regionCfg *aws.Config
	if c.Settings.Region == defaultRegion {
		backend.Logger.Warn("Region is set to \"default\", which is unsupported")
		c.Settings.Region = ""
	}
	if c.Settings.Region != "" {
		if c.Settings.AssumeRoleARN != "" && sc.authSettings.AssumeRoleEnabled && isOptInRegion(c.Settings.Region) {
			// When assuming a role, the real region is set later in a new session
			// so we use a well-known region here (not opt-in) to obtain valid credentials
			regionCfg = &aws.Config{Region: aws.String("us-east-1")}
			cfgs = append(cfgs, regionCfg)
		} else {
			regionCfg = &aws.Config{Region: aws.String(c.Settings.Region)}
			cfgs = append(cfgs, regionCfg)
		}
	}

	switch c.Settings.AuthType {
	case AuthTypeSharedCreds:
		backend.Logger.Debug("Authenticating towards AWS with shared credentials", "profile", c.Settings.Profile,
			"region", c.Settings.Region)
		cfgs = append(cfgs, &aws.Config{
			Credentials: credentials.NewSharedCredentials("", c.Settings.Profile),
		})
	case AuthTypeKeys:
		backend.Logger.Debug("Authenticating towards AWS with an access key pair", "region", c.Settings.Region)
		cfgs = append(cfgs, &aws.Config{
			Credentials: credentials.NewStaticCredentials(c.Settings.AccessKey, c.Settings.SecretKey, c.Settings.SessionToken),
		})
	case AuthTypeDefault:
		backend.Logger.Debug("Authenticating towards AWS with default SDK method", "region", c.Settings.Region)
	case AuthTypeEC2IAMRole:
		backend.Logger.Debug("Authenticating towards AWS with IAM Role", "region", c.Settings.Region)
		sess, err := newSession(cfgs...)
		if err != nil {
			return nil, err
		}
		cfgs = append(cfgs, &aws.Config{Credentials: newEC2RoleCredentials(sess)})
	default:
		panic(fmt.Sprintf("Unrecognized authType: %d", c.Settings.AuthType))
	}

	if c.Settings.Endpoint != "" {
		cfgs = append(cfgs, &aws.Config{Endpoint: aws.String(c.Settings.Endpoint)})
	}

	sess, err := newSession(cfgs...)
	if err != nil {
		return nil, err
	}

	duration := stscreds.DefaultDuration
	if sc.authSettings.SessionDuration != nil {
		duration = *sc.authSettings.SessionDuration
	}
	expiration := time.Now().UTC().Add(duration)
	if c.Settings.AssumeRoleARN != "" && sc.authSettings.AssumeRoleEnabled {
		// We should assume a role in AWS
		backend.Logger.Debug("Trying to assume role in AWS", "arn", c.Settings.AssumeRoleARN)

		cfgs := []*aws.Config{
			{
				CredentialsChainVerboseErrors: aws.Bool(true),
			},
			{
				// The previous session is used to obtain STS Credentials
				Credentials: newSTSCredentials(sess, c.Settings.AssumeRoleARN, func(p *stscreds.AssumeRoleProvider) {
					// Not sure if this is necessary, overlaps with p.Duration and is undocumented
					p.Expiry.SetExpiration(expiration, 0)
					p.Duration = duration
					if c.Settings.ExternalID != "" {
						p.ExternalID = aws.String(c.Settings.ExternalID)
					}
				}),
			},
		}

		if c.Settings.Region != "" {
			regionCfg = &aws.Config{Region: aws.String(c.Settings.Region)}
			cfgs = append(cfgs, regionCfg)
		}
		sess, err = newSession(cfgs...)
		if err != nil {
			return nil, err
		}
	}

	if c.UserAgentName != nil {
		sess.Handlers.Send.PushFront(func(r *request.Request) {
			r.HTTPRequest.Header.Set("User-Agent", GetUserAgentString(*c.UserAgentName))
		})
	}

	backend.Logger.Debug("Successfully created AWS session")

	sc.sessCacheLock.Lock()
	sc.sessCache[cacheKey] = envelope{
		session:    sess,
		expiration: expiration,
	}
	sc.sessCacheLock.Unlock()

	return sess, nil
}
