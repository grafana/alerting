package nfstatus

import (
	"github.com/prometheus/alertmanager/notify"
)

// Receiver wraps a notify.Receiver, but additionally holds onto nfstatus.Integration.
type Receiver struct {
	receiver     *notify.Receiver
	integrations []*Integration
}

func (r *Receiver) Receiver() *notify.Receiver {
	return r.receiver
}

func (r *Receiver) Name() string {
	return r.receiver.Name()
}

func (r *Receiver) Active() bool {
	return r.receiver.Active()
}

func (r *Receiver) Integrations() []*Integration {
	return r.integrations
}

func NewReceiver(name string, active bool, integrations []*Integration) *Receiver {
	receiver := notify.NewReceiver(name, active, GetIntegrations(integrations))

	return &Receiver{
		receiver:     receiver,
		integrations: integrations,
	}
}
