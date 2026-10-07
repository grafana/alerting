// Copyright 2021 Prometheus Team
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v0mimir1

import (
	"context"
	"errors"
	"fmt"
	"net/http"
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
	"github.com/aws/smithy-go"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	commoncfg "github.com/prometheus/common/config"

	httpcfg "github.com/grafana/alerting/http/v0mimir"
	"github.com/grafana/alerting/logging"
	"github.com/prometheus/alertmanager/notify"
	"github.com/prometheus/alertmanager/template"
	"github.com/prometheus/alertmanager/types"
)

// Notifier implements a Notifier for SNS notifications.
type Notifier struct {
	conf    *Config
	tmpl    *template.Template
	logger  log.Logger
	client  *http.Client
	retrier *notify.Retrier
}

// New returns a new SNS notification handler.
func New(c *Config, t *template.Template, l log.Logger, httpOpts ...commoncfg.HTTPClientOption) (*Notifier, error) {
	client, err := httpcfg.NewClientFromConfig(c.HTTPConfig, "sns", httpOpts...)
	if err != nil {
		return nil, err
	}
	return &Notifier{
		conf:    c,
		tmpl:    t,
		logger:  l,
		client:  client,
		retrier: &notify.Retrier{},
	}, nil
}

func (n *Notifier) SendResolved() bool { return n.conf.SendResolved() }

func (n *Notifier) Notify(ctx context.Context, alert ...*types.Alert) (bool, error) {
	var (
		err  error
		data = notify.GetTemplateData(ctx, n.tmpl, alert, logging.NewSlogLogger(n.logger))
		tmpl = notify.TmplText(n.tmpl, data, &err)
	)

	client, err := n.createSNSClient(ctx, tmpl)
	if err != nil {
		return true, err
	}

	publishInput, err := n.createPublishInput(ctx, tmpl)
	if err != nil {
		return true, err
	}

	publishOutput, err := client.Publish(ctx, publishInput)
	if err != nil {
		var e *smithyhttp.ResponseError
		if errors.As(err, &e) {
			message := e.Error()
			var apiErr smithy.APIError
			if errors.As(err, &apiErr) {
				message = apiErr.ErrorMessage()
			}
			retryable, error := n.retrier.Check(e.HTTPStatusCode(), strings.NewReader(message))

			reasonErr := notify.NewErrorWithReason(notify.GetFailureReasonFromStatusCode(e.HTTPStatusCode()), error)
			return retryable, reasonErr
		}
		return true, err
	}

	level.Debug(n.logger).Log("msg", "SNS message successfully published", "message_id", aws.ToString(publishOutput.MessageId), "sequence number", aws.ToString(publishOutput.SequenceNumber))

	return false, nil
}

func (n *Notifier) createSNSClient(ctx context.Context, tmpl func(string) string) (*sns.Client, error) {
	var staticCreds aws.CredentialsProvider
	// If there are provided sigV4 credentials we want to use those for the SNS client.
	if n.conf.Sigv4.AccessKey != "" && n.conf.Sigv4.SecretKey != "" {
		staticCreds = credentials.NewStaticCredentialsProvider(n.conf.Sigv4.AccessKey, string(n.conf.Sigv4.SecretKey), "")
	}
	opts := []func(*awsconfig.LoadOptions) error{awsconfig.WithRegion(n.conf.Sigv4.Region)}
	if n.conf.Sigv4.Profile != "" {
		opts = append(opts, awsconfig.WithSharedConfigProfile(n.conf.Sigv4.Profile))
	}
	// The HTTP client is set on the SNS client only (as in aws-sdk-go v1), so
	// LoadDefaultConfig can still apply AWS_CA_BUNDLE to the default client used by STS.
	cfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, err
	}
	// We will always need a region to be set by either the local config or the environment.
	if cfg.Region == "" {
		return nil, fmt.Errorf("region not configured in sns.sigv4.region or in default credentials chain")
	}

	creds := staticCreds
	if n.conf.Sigv4.RoleARN != "" {
		// Matches the aws-sdk-go v1 implementation: the STS client only uses the
		// static credentials when the API URL is set, otherwise the default chain.
		stsCfg := cfg.Copy()
		if n.conf.APIUrl != "" && staticCreds != nil {
			stsCfg.Credentials = aws.NewCredentialsCache(staticCreds)
		}
		roleSessionName := strconv.FormatInt(time.Now().UTC().UnixNano(), 10)
		creds = aws.NewCredentialsCache(stscreds.NewAssumeRoleProvider(sts.NewFromConfig(stsCfg), n.conf.Sigv4.RoleARN, func(o *stscreds.AssumeRoleOptions) {
			o.RoleSessionName = roleSessionName
		}))
	}

	apiURL := tmpl(n.conf.APIUrl)
	return sns.NewFromConfig(cfg, func(o *sns.Options) {
		o.HTTPClient = n.client
		if creds != nil {
			o.Credentials = creds
		}
		if apiURL != "" {
			o.BaseEndpoint = aws.String(withDefaultScheme(apiURL))
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

func (n *Notifier) createPublishInput(ctx context.Context, tmpl func(string) string) (*sns.PublishInput, error) {
	publishInput := &sns.PublishInput{}
	messageAttributes := n.createMessageAttributes(tmpl)
	// Max message size for a message in a SNS publish request is 256KB, except for SMS messages where the limit is 1600 characters/runes.
	messageSizeLimit := 256 * 1024
	if n.conf.TopicARN != "" {
		topicARN := tmpl(n.conf.TopicARN)
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
	if n.conf.PhoneNumber != "" {
		publishInput.PhoneNumber = aws.String(tmpl(n.conf.PhoneNumber))
		// If we have an SMS message, we need to truncate to 1600 characters/runes.
		messageSizeLimit = 1600
	}
	if n.conf.TargetARN != "" {
		publishInput.TargetArn = aws.String(tmpl(n.conf.TargetARN))
	}

	messageToSend, isTrunc, err := validateAndTruncateMessage(tmpl(n.conf.Message), messageSizeLimit)
	if err != nil {
		return nil, err
	}
	if isTrunc {
		// If we truncated the message we need to add a message attribute showing that it was truncated.
		messageAttributes["truncated"] = snstypes.MessageAttributeValue{DataType: aws.String("String"), StringValue: aws.String("true")}
	}

	publishInput.Message = aws.String(messageToSend)
	publishInput.MessageAttributes = messageAttributes

	if n.conf.Subject != "" {
		publishInput.Subject = aws.String(tmpl(n.conf.Subject))
	}

	return publishInput, nil
}

func validateAndTruncateMessage(message string, maxMessageSizeInBytes int) (string, bool, error) {
	if !utf8.ValidString(message) {
		return "", false, fmt.Errorf("non utf8 encoded message string")
	}
	if len(message) <= maxMessageSizeInBytes {
		return message, false, nil
	}
	// If the message is larger than our specified size we have to truncate.
	truncated := make([]byte, maxMessageSizeInBytes)
	copy(truncated, message)
	return string(truncated), true, nil
}

func (n *Notifier) createMessageAttributes(tmpl func(string) string) map[string]snstypes.MessageAttributeValue {
	// Convert the given attributes map into the AWS Message Attributes Format.
	attributes := make(map[string]snstypes.MessageAttributeValue, len(n.conf.Attributes))
	for k, v := range n.conf.Attributes {
		attributes[tmpl(k)] = snstypes.MessageAttributeValue{DataType: aws.String("String"), StringValue: aws.String(tmpl(v))}
	}
	return attributes
}
