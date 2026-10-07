package v1

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	snstypes "github.com/aws/aws-sdk-go-v2/service/sns/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/go-kit/log/level"
	"github.com/prometheus/alertmanager/notify"
	"github.com/prometheus/alertmanager/types"

	"github.com/go-kit/log"

	"github.com/grafana/alerting/receivers"
	"github.com/grafana/alerting/templates"
)

const subjectSizeLimit = 100

// snsClient is the subset of the Amazon SNS client used by the Notifier. It is
// implemented by *sns.Client and allows the client to be mocked in tests.
type snsClient interface {
	Publish(ctx context.Context, input *sns.PublishInput, optFns ...func(*sns.Options)) (*sns.PublishOutput, error)
}

// Notifier is responsible for sending
// alert notifications to Amazon SNS.
type Notifier struct {
	*receivers.Base
	tmpl     *templates.Template
	settings Config
	// newSNSClient builds the SNS client used to publish the notification. It is
	// a field so it can be overridden in tests.
	newSNSClient func(ctx context.Context) (snsClient, error)
}

func New(cfg Config, meta receivers.Metadata, template *templates.Template, logger log.Logger) *Notifier {
	n := &Notifier{
		Base:     receivers.NewBase(meta, logger),
		tmpl:     template,
		settings: cfg,
	}
	n.newSNSClient = func(ctx context.Context) (snsClient, error) {
		return n.createSNSClient(ctx)
	}
	return n
}

// Notify sends the alert notification to sns.
func (s *Notifier) Notify(ctx context.Context, as ...*types.Alert) (bool, error) {
	l := s.GetLogger(ctx)
	var tmplErr error
	tmpl, data := templates.TmplText(ctx, s.tmpl, as, l, &tmplErr)

	receivers.ApplyExtraData(ctx, data.Alerts)

	level.Info(l).Log("msg", "Sending notification")

	publishInput, err := s.createPublishInput(ctx, tmpl)
	if err != nil {
		return false, err
	}

	snsClient, err := s.newSNSClient(ctx)
	if err != nil {
		return true, err
	}

	// check template error after we use them
	if tmplErr != nil {
		level.Warn(l).Log("msg", "failed to template message", "err", tmplErr.Error())
	}

	publishOutput, err := snsClient.Publish(ctx, publishInput)
	if err != nil {
		level.Error(l).Log("msg", "Failed to publish to Amazon SNS. ", "err", err)
		return true, err
	}

	level.Debug(l).Log("msg", "Message successfully published", "messageId", aws.ToString(publishOutput.MessageId), "sequenceNumber", aws.ToString(publishOutput.SequenceNumber))
	return true, nil
}

func (s *Notifier) SendResolved() bool {
	return !s.GetDisableResolveMessage()
}

func (s *Notifier) createMessageAttributes(tmpl func(string) string) map[string]snstypes.MessageAttributeValue {
	// Convert the given attributes map into the AWS Message Attributes Format.
	attributes := make(map[string]snstypes.MessageAttributeValue, len(s.settings.Attributes))
	for k, v := range s.settings.Attributes {
		attributes[tmpl(k)] = snstypes.MessageAttributeValue{DataType: aws.String("String"), StringValue: aws.String(tmpl(v))}
	}
	return attributes
}

func (s *Notifier) createSNSClient(ctx context.Context) (*sns.Client, error) {
	var staticCreds aws.CredentialsProvider
	// If there are provided sigV4 credentials we want to use those for the SNS client.
	if s.settings.Sigv4.AccessKey != "" && s.settings.Sigv4.SecretKey != "" {
		staticCreds = credentials.NewStaticCredentialsProvider(s.settings.Sigv4.AccessKey, s.settings.Sigv4.SecretKey, "")
	}
	opts := []func(*awsconfig.LoadOptions) error{awsconfig.WithRegion(s.settings.Sigv4.Region)}
	if s.settings.Sigv4.Profile != "" {
		opts = append(opts, awsconfig.WithSharedConfigProfile(s.settings.Sigv4.Profile))
	}
	cfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, err
	}
	// We will always need a region to be set by either the local config or the environment.
	if cfg.Region == "" {
		return nil, fmt.Errorf("region not configured in sns.sigv4.region or in default credentials chain")
	}

	creds := staticCreds
	if s.settings.Sigv4.RoleARN != "" {
		// Matches the aws-sdk-go v1 implementation: the STS client only uses the
		// static credentials when the API URL is set, otherwise the default chain.
		stsCfg := cfg.Copy()
		if s.settings.APIUrl != "" && staticCreds != nil {
			stsCfg.Credentials = aws.NewCredentialsCache(staticCreds)
		}
		roleSessionName := strconv.FormatInt(time.Now().UTC().UnixNano(), 10)
		creds = aws.NewCredentialsCache(stscreds.NewAssumeRoleProvider(sts.NewFromConfig(stsCfg), s.settings.Sigv4.RoleARN, func(o *stscreds.AssumeRoleOptions) {
			o.RoleSessionName = roleSessionName
		}))
	}

	return sns.NewFromConfig(cfg, func(o *sns.Options) {
		if creds != nil {
			o.Credentials = creds
		}
		if s.settings.APIUrl != "" {
			o.BaseEndpoint = aws.String(withDefaultScheme(s.settings.APIUrl))
		}
	}), nil
}

// withDefaultScheme prefixes endpoints without a scheme with https://, as
// aws-sdk-go v1 did. aws-sdk-go-v2 requires BaseEndpoint to be a full URL.
func withDefaultScheme(endpoint string) string {
	if strings.Contains(endpoint, "://") {
		return endpoint
	}
	return "https://" + endpoint
}

func (s *Notifier) createPublishInput(ctx context.Context, tmpl func(string) string) (*sns.PublishInput, error) {
	publishInput := &sns.PublishInput{}
	messageAttributes := s.createMessageAttributes(tmpl)
	// Max message size for a message in an SNS publish request is 256KB, except for SMS messages where the limit is 1600 characters/runes.
	messageSizeLimit := 256 * 1024
	if s.settings.TopicARN != "" {
		topicARN := tmpl(s.settings.TopicARN)
		publishInput.TopicArn = aws.String(topicARN)
		// If we are using a topic ARN, it could be a FIFO topic specified by the topic's suffix ".fifo".
		if strings.HasSuffix(topicARN, ".fifo") {
			// Deduplication key and Message Group ID are only added if it's a FIFO SNS Topic.
			key, err := notify.ExtractGroupKey(ctx)
			if err != nil {
				return nil, err
			}
			publishInput.MessageDeduplicationId = aws.String(key.Hash())
			publishInput.MessageGroupId = aws.String(key.Hash())
		}
	}

	if s.settings.PhoneNumber != "" {
		publishInput.PhoneNumber = aws.String(tmpl(s.settings.PhoneNumber))
		// If we have an SMS message, we need to truncate to 1600 characters/runes.
		messageSizeLimit = 1600
	}
	if s.settings.TargetARN != "" {
		publishInput.TargetArn = aws.String(tmpl(s.settings.TargetARN))
	}

	messageToSend, isTrunc, err := validateAndTruncateString(tmpl(s.settings.Message), messageSizeLimit)
	if err != nil {
		return nil, fmt.Errorf("message validation failed: %v", err)
	}
	if isTrunc {
		// If we truncated the message we need to add a message attribute showing that it was truncated.
		messageAttributes["truncated"] = snstypes.MessageAttributeValue{DataType: aws.String("String"), StringValue: aws.String("true")}
	}

	subject, subjIsTrunc, err := validateAndTruncateString(tmpl(s.settings.Subject), subjectSizeLimit)
	if err != nil {
		return nil, fmt.Errorf("subject validation failed: %v", err)
	}
	if subjIsTrunc {
		// If we truncated the subject we need to add a message attribute showing that it was truncated.
		messageAttributes["subject_truncated"] = snstypes.MessageAttributeValue{DataType: aws.String("String"), StringValue: aws.String("true")}
	}
	if subject != "" {
		publishInput.Subject = aws.String(subject)
	}

	publishInput.Message = aws.String(messageToSend)
	publishInput.MessageAttributes = messageAttributes

	return publishInput, nil
}

func validateAndTruncateString(message string, maxMessageSizeInBytes int) (string, bool, error) {
	if !utf8.ValidString(message) {
		return "", false, fmt.Errorf("non utf8 encoded string")
	}
	if len(message) <= maxMessageSizeInBytes {
		return message, false, nil
	}
	// If the given string is larger than our specified size we have to truncate.
	truncated := make([]byte, maxMessageSizeInBytes)
	copy(truncated, message)
	return string(truncated), true, nil
}
