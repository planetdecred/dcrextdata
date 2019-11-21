package netsnapshot

import (
	"context"

	"github.com/decred/dcrd/rpcclient"
)

type SnapShot struct {
	Timestamp int64 `json:"timestamp"`
	Height    int64 `json:"height"`
	Nodes     int   `json:"nodes"`
}

type NetworkPeer struct {
	Timestamp 		int64 `json:"timestamp"`
	Address         string `json:"address"`
	LastReceiveTime int64  `json:"last_receive_time"`
	LastSendTime    int64  `json:"last_send_time"`
	ConnectionTime  int64  `json:"connection_time"`
	ProtocolVersion uint32 `json:"protocol_version"`
	UserAgent       string `json:"user_agent"`
	StartingHeight  int64  `json:"starting_height"`
	CurrentHeight   int64  `json:"current_height"`
}

type DataStore interface {
	LastSnapshotTime(ctx context.Context) (timestamp int64)
	DeleteSnapshot(ctx context.Context, timestamp int64)
	SaveSnapshot(ctx context.Context, snapShot SnapShot) error
	SaveNetworkPeer(ctx context.Context, peer NetworkPeer) error
}

type taker struct {
	dcrClient *rpcclient.Client
	dataStore DataStore
	period    int
}
