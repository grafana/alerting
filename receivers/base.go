package receivers

import "github.com/go-kit/log"

// Base is the base implementation of a notifier. It contains the common fields across all notifier types.
type Base struct {
	Name                  string
	Type                  string
	UID                   string
	DisableResolveMessage bool
	logger                log.Logger
}

func (n *Base) GetDisableResolveMessage() bool {
	return n.DisableResolveMessage
}

func (n *Base) GetLogger() log.Logger {
	return log.With(n.logger, "receiver", n.Name, "integration", n.Type, "integration_uid", n.UID)
}

// Metadata contains the metadata of the notifier.
type Metadata struct {
	UID                   string
	Name                  string
	Type                  string
	DisableResolveMessage bool
}

func NewBase(cfg Metadata, logger log.Logger) *Base {
	return &Base{
		UID:                   cfg.UID,
		Name:                  cfg.Name,
		Type:                  cfg.Type,
		DisableResolveMessage: cfg.DisableResolveMessage,
		logger:                logger,
	}
}
