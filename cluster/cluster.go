package cluster

import (
	"time"

	gokitlog "github.com/go-kit/log"
	"github.com/prometheus/alertmanager/cluster"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/grafana/alerting/logging"
)

const (
	DefaultGossipInterval    = cluster.DefaultGossipInterval
	DefaultPushPullInterval  = cluster.DefaultPushPullInterval
	DefaultProbeInterval     = cluster.DefaultProbeInterval
	DefaultProbeTimeout      = cluster.DefaultProbeTimeout
	DefaultReconnectInterval = cluster.DefaultReconnectInterval
	DefaultReconnectTimeout  = cluster.DefaultReconnectTimeout
	DefaultTCPTimeout        = cluster.DefaultTCPTimeout
)

// Create wraps the fork's cluster.Create, which now takes a *slog.Logger, to
// keep grafana/alerting's own public API on go-kit's log.Logger (grafana/grafana
// calls this with one, e.g. pkg/services/ngalert/notifier/multiorg_alertmanager.go).
func Create(
	l gokitlog.Logger,
	reg prometheus.Registerer,
	bindAddr string,
	advertiseAddr string,
	knownPeers []string,
	waitIfEmpty bool,
	pushPullInterval time.Duration,
	gossipInterval time.Duration,
	tcpTimeout time.Duration,
	probeTimeout time.Duration,
	probeInterval time.Duration,
	tlsTransportConfig *cluster.TLSTransportConfig,
	allowInsecureAdvertise bool,
	label string,
) (*cluster.Peer, error) {
	return cluster.Create(
		logging.NewSlogLogger(l),
		reg,
		bindAddr,
		advertiseAddr,
		knownPeers,
		waitIfEmpty,
		pushPullInterval,
		gossipInterval,
		tcpTimeout,
		probeTimeout,
		probeInterval,
		tlsTransportConfig,
		allowInsecureAdvertise,
		label,
	)
}

type ClusterChannel = cluster.ClusterChannel //nolint:revive
type ChannelOption = cluster.ChannelOption
type ChannelOptions = cluster.ChannelOptions
type Peer = cluster.Peer
type State = cluster.State

var (
	WithReliableDelivery = cluster.WithReliableDelivery
	WithQueueSize        = cluster.WithQueueSize
	ResolveOptions       = cluster.ResolveOptions
)
