package channels

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/alertmanager/template"
	"github.com/prometheus/alertmanager/types"
	"golang.org/x/sync/singleflight"

	"github.com/grafana/alerting/alerting/log"
	"github.com/grafana/alerting/alerting/notifier/config"
	"github.com/grafana/alerting/alerting/notifier/sender"
	template2 "github.com/grafana/alerting/alerting/notifier/template"
)

func WeComFactory(fc config.FactoryConfig) (NotificationChannel, error) {
	ch, err := buildWecomNotifier(fc)
	if err != nil {
		return nil, receiverInitError{
			Reason: err.Error(),
			Cfg:    *fc.Config,
		}
	}
	return ch, nil
}

func buildWecomNotifier(factoryConfig config.FactoryConfig) (*WeComNotifier, error) {
	settings, err := config.BuildWecomConfig(factoryConfig)
	if err != nil {
		return nil, err
	}
	return &WeComNotifier{
		Base:     NewBase(factoryConfig.Config),
		tmpl:     factoryConfig.Template,
		log:      factoryConfig.Logger,
		ns:       factoryConfig.NotificationService,
		settings: settings,
	}, nil
}

// WeComNotifier is responsible for sending alert notifications to WeCom.
type WeComNotifier struct {
	*Base
	tmpl        *template.Template
	log         log.Logger
	ns          sender.WebhookSender
	settings    config.WecomConfig
	tok         *WeComAccessToken
	tokExpireAt time.Time
	group       singleflight.Group
}

// Notify send an alert notification to WeCom.
func (w *WeComNotifier) Notify(ctx context.Context, as ...*types.Alert) (bool, error) {
	w.log.Info("executing WeCom notification", "notification", w.Name)

	var tmplErr error
	tmpl, _ := template2.TmplText(ctx, w.tmpl, as, w.log, &tmplErr)

	bodyMsg := map[string]interface{}{
		"msgtype": w.settings.MsgType,
	}
	content := fmt.Sprintf("# %s\n%s\n",
		tmpl(w.settings.Title),
		tmpl(w.settings.Message),
	)
	if w.settings.MsgType != config.DefaultWeComMsgType {
		content = fmt.Sprintf("%s\n%s\n",
			tmpl(w.settings.Title),
			tmpl(w.settings.Message),
		)
	}

	msgType := string(w.settings.MsgType)
	bodyMsg[msgType] = map[string]interface{}{
		"content": content,
	}

	url := w.settings.URL
	if w.settings.Channel != config.DefaultWeComChannelType {
		bodyMsg["agentid"] = w.settings.AgentID
		bodyMsg["touser"] = w.settings.ToUser
		token, err := w.GetAccessToken(ctx)
		if err != nil {
			return false, err
		}
		url = fmt.Sprintf(w.settings.EndpointURL+"/cgi-bin/message/send?access_token=%s", token)
	}

	body, err := json.Marshal(bodyMsg)
	if err != nil {
		return false, err
	}

	if tmplErr != nil {
		w.log.Warn("failed to template WeCom message", "error", tmplErr.Error())
	}

	cmd := &sender.SendWebhookSettings{
		URL:  url,
		Body: string(body),
	}

	if err = w.ns.SendWebhook(ctx, cmd); err != nil {
		w.log.Error("failed to send WeCom webhook", "error", err, "notification", w.Name)
		return false, err
	}

	return true, nil
}

// GetAccessToken returns the access token for apiapp
func (w *WeComNotifier) GetAccessToken(ctx context.Context) (string, error) {
	t := w.tok
	if w.tokExpireAt.Before(time.Now()) || w.tok == nil {
		// avoid multiple calls when there are multiple alarms
		tok, err, _ := w.group.Do("GetAccessToken", func() (interface{}, error) {
			return w.getAccessToken(ctx)
		})
		if err != nil {
			return "", err
		}
		t = tok.(*WeComAccessToken)
		// expire five minutes in advance to avoid using it when it is about to expire
		w.tokExpireAt = time.Now().Add(time.Second * time.Duration(t.ExpireIn-300))
		w.tok = t
	}
	return t.AccessToken, nil
}

type WeComAccessToken struct {
	AccessToken string `json:"access_token"`
	ErrMsg      string `json:"errmsg"`
	ErrCode     int    `json:"errcode"`
	ExpireIn    int    `json:"expire_in"`
}

func (w *WeComNotifier) getAccessToken(ctx context.Context) (*WeComAccessToken, error) {
	geTokenURL := fmt.Sprintf(w.settings.EndpointURL+"/cgi-bin/gettoken?corpid=%s&corpsecret=%s", w.settings.CorpID, w.settings.Secret)

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, geTokenURL, nil)
	if err != nil {
		return nil, err
	}

	request.Header.Add("Content-Type", "application/json")
	request.Header.Add("User-Agent", "Grafana")

	resp, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("WeCom returned statuscode invalid status code: %v", resp.Status)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	var accessToken WeComAccessToken
	err = json.NewDecoder(resp.Body).Decode(&accessToken)
	if err != nil {
		return nil, err
	}

	if accessToken.ErrCode != 0 {
		return nil, fmt.Errorf("WeCom returned errmsg: %s", accessToken.ErrMsg)
	}
	return &accessToken, nil
}

func (w *WeComNotifier) SendResolved() bool {
	return !w.GetDisableResolveMessage()
}
