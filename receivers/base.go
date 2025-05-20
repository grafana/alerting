package receivers

import (
	"fmt"

	"github.com/go-kit/log"
)

// Base is the base implementation of a notifier. It contains the common fields across all notifier types.
type Base struct {
	Index                 int
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
	return log.With(n.logger, "receiver", n.Name, "integration", fmt.Sprintf("%s[%d]", n.Type, n.Index))
}

// Metadata contains the metadata of the notifier.
type Metadata struct {
	Index                 int
	UID                   string
	Name                  string
	Type                  string
	DisableResolveMessage bool
}

func NewBase(cfg Metadata, logger log.Logger) *Base {
	return &Base{
		Index:                 cfg.Index,
		UID:                   cfg.UID,
		Name:                  cfg.Name,
		Type:                  cfg.Type,
		DisableResolveMessage: cfg.DisableResolveMessage,
		logger:                logger,
	}
}
