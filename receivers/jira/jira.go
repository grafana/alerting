package jira

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/prometheus/common/model"

	"github.com/prometheus/alertmanager/notify"
	"github.com/prometheus/alertmanager/types"

	"github.com/grafana/alerting/logging"
	"github.com/grafana/alerting/receivers"
	"github.com/grafana/alerting/templates"
)

const (
	MaxSummaryLenRunes     = 255
	MaxDescriptionLenRunes = 32767
)

// Notifier implements a Notifier for JIRA notifications. Can use V2 and V3 API to create issues, depending on Config.URL
// https://developer.atlassian.com/cloud/jira/platform/rest/v3/api-group-issues/#api-rest-api-3-issue-post
type Notifier struct {
	*receivers.Base
	tmpl    *templates.Template
	log     logging.Logger
	ns      receivers.WebhookSender
	conf    Config
	retrier *notify.Retrier
}

func New(cfg Config, meta receivers.Metadata, template *templates.Template, sender receivers.WebhookSender, logger logging.Logger) *Notifier {
	return &Notifier{
		Base:    receivers.NewBase(meta),
		log:     logger,
		ns:      sender,
		tmpl:    template,
		conf:    cfg,
		retrier: &notify.Retrier{RetryCodes: []int{http.StatusTooManyRequests}},
	}
}

// Notify implements the Notifier interface.
func (n *Notifier) Notify(ctx context.Context, as ...*types.Alert) (bool, error) {
	var (
		alerts = types.Alerts(as...)
	)

	key, err := notify.ExtractGroupKey(ctx)
	if err != nil {
		return false, err
	}
	logger := n.log.New("group_key", key.Hash())
	logger.Debug("executing Jira notification")

	existingIssue, shouldRetry, err := n.searchExistingIssue(ctx, logger, key.Hash(), alerts.HasFiring())
	if err != nil {
		return shouldRetry, fmt.Errorf("failed to look up existing issues: %w", err)
	}

	var method string
	var path string
	if existingIssue == nil {
		// Do not create new issues for resolved alerts
		if alerts.Status() == model.AlertResolved {
			return false, nil
		}
		path = "issue"
		method = http.MethodPost
		logger.Debug("create new issue")
	} else {
		path = "issue/" + existingIssue.Key
		method = http.MethodPut
		logger.Debug("updating existing issue", "issue_key", existingIssue.Key)
	}

	requestBody, err := n.prepareIssueRequestBody(ctx, logger, key.Hash(), as...)
	if err != nil {
		return false, err
	}

	_, shouldRetry, err = n.doAPIRequest(ctx, method, path, requestBody)
	if err != nil {
		return shouldRetry, fmt.Errorf("failed to %s request to %q: %w", method, path, err)
	}

	return n.transitionIssue(ctx, logger, existingIssue, alerts.HasFiring())
}

func (n *Notifier) prepareIssueRequestBody(ctx context.Context, logger logging.Logger, groupID string, as ...*types.Alert) (issue, error) {
	var tmplErr error
	tmpl, _ := templates.TmplText(ctx, n.tmpl, as, n.log, &tmplErr)

	renderOrDefault := func(fieldName, template, fallback string) string {
		defer func() {
			tmplErr = nil
		}()
		result := tmpl(template)
		if tmplErr == nil {
			return result
		}
		if fallback == "" {
			logger.Error("failed to render template", "error", tmplErr, "configField", fieldName, "template", template)
			return ""
		}
		logger.Error("failed to render template, use default template", "error", tmplErr, "configField", fieldName, "template", template)
		tmplErr = nil
		result = tmpl(fallback)
		if tmplErr == nil {
			return result
		}
		logger.Error("failed to render default template", "error", tmplErr, "configField", fieldName, "template", fallback)
		return ""
	}

	summary := renderOrDefault("summary", n.conf.Summary, DefaultSummary)
	summary, truncated := notify.TruncateInRunes(summary, MaxSummaryLenRunes)
	if truncated {
		logger.Warn("Truncated summary", "max_runes", MaxSummaryLenRunes)
	}

	fields := &issueFields{
		Summary: summary,
		Labels:  make([]string, 0, len(n.conf.Labels)+1),
		Fields:  n.conf.Fields,
	}

	projectKey := strings.TrimSpace(renderOrDefault("project", n.conf.Project, ""))
	if projectKey != "" {
		fields.Project = &issueProject{Key: projectKey}
	}
	issueType := strings.TrimSpace(renderOrDefault("issue_type", n.conf.IssueType, ""))
	if issueType != "" {
		fields.Issuetype = &idNameValue{Name: issueType}
	}

	// // Recursively convert any maps to map[string]interface{}, filtering out all non-string keys, so the json encoder
	// // doesn't blow up when marshaling JIRA requests.
	// fieldsWithStringKeys, err := tcontainer.ConvertToMarshalMap(n.conf.Fields, func(v string) string { return v })
	// if err != nil {
	// 	return issue{}, fmt.Errorf("convertToMarshalMap: %w", err)
	// }

	issueDescriptionString := renderOrDefault("description", n.conf.Description, DefaultDescription)
	if len([]rune(issueDescriptionString)) > MaxDescriptionLenRunes {
		logger.Warn("Description exceeds the max length, falling back to default description", "max_runes", MaxDescriptionLenRunes, "description_length", len(issueDescriptionString))
		issueDescriptionString = DefaultDescription
	}

	fields.Description = issueDescriptionString
	if strings.HasSuffix(n.conf.URL.Path, "/3") {
		var issueDescription any
		// TODO what to do if truncated and not unmarshalled?
		if err := json.Unmarshal([]byte(issueDescriptionString), &issueDescription); err != nil {
			return issue{}, fmt.Errorf("description unmarshaling: %w", err)
		}
		fields.Description = issueDescription
	}

	for i, label := range n.conf.Labels {
		label = strings.TrimSpace(renderOrDefault(fmt.Sprintf("labels[%d]", i), label, ""))
		if label == "" {
			continue
		}
		fields.Labels = append(fields.Labels, label)
	}
	fields.Labels = append(fields.Labels, fmt.Sprintf("ALERT{%s}", groupID))
	sort.Strings(fields.Labels)

	priority := strings.TrimSpace(renderOrDefault("priority", n.conf.Priority, ""))
	if priority != "" {
		fields.Priority = &idNameValue{Name: priority}
	}

	return issue{Fields: fields}, nil
}

func (n *Notifier) searchExistingIssue(ctx context.Context, logger logging.Logger, groupID string, firing bool) (*issue, bool, error) {
	jql := strings.Builder{}

	if n.conf.WontFixResolution != "" {
		jql.WriteString(fmt.Sprintf(`resolution != %q and `, n.conf.WontFixResolution))
	}

	// If the group is firing, do not search for closed issues unless a reopen transition is defined.
	if firing {
		if n.conf.ReopenTransition == "" {
			jql.WriteString(`statusCategory != Done and `)
		}
	} else {
		reopenDuration := int64(time.Duration(n.conf.ReopenDuration).Minutes())
		if reopenDuration != 0 {
			jql.WriteString(fmt.Sprintf(`(resolutiondate is EMPTY OR resolutiondate >= -%dm) and `, reopenDuration))
		}
	}

	alertLabel := fmt.Sprintf("ALERT{%s}", groupID)
	jql.WriteString(fmt.Sprintf(`project=%q and labels=%q order by status ASC,resolutiondate DESC`, n.conf.Project, alertLabel))

	requestBody := issueSearch{
		JQL:        jql.String(),
		MaxResults: 2,
		Fields:     []string{"status"},
		Expand:     []string{},
	}

	logger.Debug("search for recent issues", "jql", requestBody.JQL)

	responseBody, shouldRetry, err := n.doAPIRequest(ctx, http.MethodPost, "search", requestBody)
	if err != nil {
		return nil, shouldRetry, fmt.Errorf("HTTP request to JIRA API: %w", err)
	}

	var issueSearchResult issueSearchResult
	err = json.Unmarshal(responseBody, &issueSearchResult)
	if err != nil {
		return nil, false, err
	}

	if issueSearchResult.Total == 0 {
		logger.Debug("found no existing issue")
		return nil, false, nil
	}

	if issueSearchResult.Total > 1 {
		logger.Warn("more than one issue matched, selecting the most recently resolved", "selected_issue", issueSearchResult.Issues[0].Key)
	}

	return &issueSearchResult.Issues[0], false, nil
}

func (n *Notifier) getIssueTransitionByName(ctx context.Context, issueKey, transitionName string) (string, bool, error) {
	path := fmt.Sprintf("issue/%s/transitions", issueKey)

	responseBody, shouldRetry, err := n.doAPIRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return "", shouldRetry, err
	}

	var issueTransitions issueTransitions
	err = json.Unmarshal(responseBody, &issueTransitions)
	if err != nil {
		return "", false, err
	}

	for _, issueTransition := range issueTransitions.Transitions {
		if issueTransition.Name == transitionName {
			return issueTransition.ID, false, nil
		}
	}

	return "", false, fmt.Errorf("can't find transition %s for issue %s", transitionName, issueKey)
}

func (n *Notifier) transitionIssue(ctx context.Context, logger logging.Logger, i *issue, firing bool) (bool, error) {
	if i == nil || i.Key == "" || i.Fields == nil || i.Fields.Status == nil {
		return false, nil
	}

	var transition string
	if firing {
		if i.Fields.Status.StatusCategory.Key != "done" {
			return false, nil
		}

		transition = n.conf.ReopenTransition
	} else {
		if i.Fields.Status.StatusCategory.Key == "done" {
			return false, nil
		}

		transition = n.conf.ResolveTransition
	}

	transitionID, shouldRetry, err := n.getIssueTransitionByName(ctx, i.Key, transition)
	if err != nil {
		return shouldRetry, err
	}

	requestBody := issue{
		Transition: &idNameValue{
			ID: transitionID,
		},
	}

	path := fmt.Sprintf("issue/%s/transitions", i.Key)

	logger.Debug("transitions jira issue", "issue_key", i.Key, "transition", transition)
	_, shouldRetry, err = n.doAPIRequest(ctx, http.MethodPost, path, requestBody)

	return shouldRetry, err
}

func (n *Notifier) doAPIRequest(ctx context.Context, method, path string, requestBody any) ([]byte, bool, error) {
	body, err := json.Marshal(requestBody)
	if err != nil {
		return nil, false, fmt.Errorf("failed to marshal request body: %w", err)
	}

	headers := make(map[string]string, 3)
	headers["Content-Type"] = "application/json"
	headers["Accept-Language"] = "en"
	if n.conf.Token != "" {
		headers["Authorization"] = fmt.Sprintf("Bearer %s", n.conf.Token)
	}

	var shouldRetry bool
	var responseBody []byte
	err = n.ns.SendWebhook(ctx, &receivers.SendWebhookSettings{
		URL:         n.conf.URL.JoinPath(path).String(),
		User:        n.conf.User,
		Password:    n.conf.Password,
		Body:        string(body),
		HTTPMethod:  method,
		HTTPHeader:  headers,
		ContentType: "application/json",
		Validation: func(body []byte, code int) error {
			responseBody = body
			shouldRetry, err = n.retrier.Check(code, bytes.NewReader(body))
			return err
		},
		TLSConfig: nil, // TODO support TLS?
	})
	if err != nil {
		return nil, shouldRetry, err
	}
	return responseBody, false, nil
}

func (n *Notifier) SendResolved() bool {
	return !n.GetDisableResolveMessage()
}
