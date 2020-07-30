package netsnapshot

import (
	"context"
	"fmt"
	"math"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/decred/dcrd/chaincfg/v2"
	"github.com/planetdecred/dcrextdata/app/config"
	"github.com/planetdecred/dcrextdata/app/helpers"
	"github.com/planetdecred/dcrextdata/cache"
)

var snapshotinterval int

func Snapshotinterval() int {
	return snapshotinterval
}

func NewTaker(store DataStore, cfg config.NetworkSnapshotOptions) *taker {
	snapshotinterval = cfg.SnapshotInterval
	return &taker{
		dataStore: store,
		cfg:       cfg,
	}
}

func (t taker) Start(ctx context.Context, cacheManager *cache.Manager) {
	log.Info("Triggering network snapshot taker.")

	var netParams = chaincfg.MainNetParams()
	if t.cfg.TestNet {
		netParams = chaincfg.TestNet3Params()
	}

	// defaultStaleTimeout = time.Minute * time.Duration(t.cfg.SnapshotInterval)
	// pruneExpireTimeout = defaultStaleTimeout * 2

	var err error
	amgr, err = NewManager(filepath.Join(defaultHomeDir,
		netParams.Name), t.cfg.ShowDetailedLog, t.cfg.SnapshotInterval)
	if err != nil {
		fmt.Fprintf(os.Stderr, "NewManager: %v\n", err)
		os.Exit(1)
	}

	// update all reachable nodes
	loadLiveNodes := func() {
		nodes, err := t.dataStore.GetAvailableNodes(ctx)
		if err != nil {
			log.Errorf("Error in taking network snapshot, %s", err.Error())
		}
		amgr.setLiveNodes(nodes)
	}

	// enqueue previous known ips
	loadLiveNodes()

	go runSeeder(t.cfg, netParams)

	var mtx sync.Mutex
	var bestBlockHeight int64

	var count int
	var timestamp = time.Now().UTC().Unix()
	snapshot := SnapShot{
		Timestamp: timestamp,
		Height:    bestBlockHeight,
	}

	lastSnapshot, err := t.dataStore.LastSnapshot(ctx)
	if err == nil {
		minutesPassed := math.Abs(time.Since(time.Unix(lastSnapshot.Timestamp, 0)).Minutes())
		if minutesPassed < float64(t.cfg.SnapshotInterval) {
			snapshot = *lastSnapshot
			timestamp = lastSnapshot.Timestamp
		}
	}

	snapshot.NodeCount = len(amgr.nodes)
	if snapshot.NodeCount > 0 {
		if err = t.dataStore.SaveSnapshot(ctx, snapshot); err != nil {
			t.dataStore.DeleteSnapshot(ctx, timestamp)
			log.Errorf("Error in saving network snapshot, %s", err.Error())
		}
	}

	ticker := time.NewTicker(time.Duration(t.cfg.SnapshotInterval) * time.Minute)
	defer ticker.Stop()

	for {
		// start listening for node heartbeat
		select {
		case <-ticker.C:
			err := t.dataStore.SaveSnapshot(ctx, SnapShot{
				Timestamp: timestamp,
				Height:    bestBlockHeight,
				NodeCount: len(amgr.nodes),
			})

			if err != nil {
				t.dataStore.DeleteSnapshot(ctx, timestamp)
				log.Errorf("Error in saving network snapshot, %s", err.Error())
			}

			if err = cacheManager.Update(ctx, cache.Snapshot, cache.SnapshotTable); err != nil {
				log.Error(err)
			}

			mtx.Lock()
			count = 0
			if t.cfg.ShowDetailedLog {
				log.Infof("Took a new network snapshot, recorded %d discoverable nodes.", count)
			}
			timestamp = time.Now().UTC().Unix()
			mtx.Unlock()
			// update all reachable nodes
			loadLiveNodes()

		case node := <-amgr.peerNtfn:
			if node.IP.String() == "127.0.0.1" { // do not add the local IP
				break
			}

			networkPeer := NetworkPeer{
				Timestamp:       timestamp,
				Address:         node.IP.String(),
				LastAttempt:     node.LastAttempt.UTC().Unix(),
				LastSeen:        node.LastSeen.UTC().Unix(),
				LastSuccess:     node.LastSuccess.UTC().Unix(),
				ConnectionTime:  node.ConnectionTime,
				ProtocolVersion: node.ProtocolVersion,
				UserAgent:       node.UserAgent,
				StartingHeight:  node.StartingHeight,
				CurrentHeight:   node.CurrentHeight,
				Services:        node.Services.String(),
				Latency:         int(node.Latency),
			}

			if exists, _ := t.dataStore.NodeExists(ctx, networkPeer.Address); exists {
				err = t.dataStore.UpdateNode(ctx, networkPeer)
				if err != nil {
					log.Errorf("Error in saving node info, %s.", err.Error())
				}
			} else {
				geoLoc, err := t.geolocation(ctx, node.IP)
				if err == nil {
					networkPeer.IPInfo = *geoLoc
					// networkPeer.Country = geoLoc.CountryName
					if geoLoc.Type == "ipv4" {
						networkPeer.IPVersion = 4
					} else if geoLoc.Type == "ipv6" {
						networkPeer.IPVersion = 6
					}
				} else {
					log.Error(err)
				}

				err = t.dataStore.SaveNode(ctx, networkPeer)
				if err != nil {
					log.Errorf("Error in saving node info, %s.", err.Error())
				}
			}

			err = t.dataStore.SaveHeartbeat(ctx, Heartbeat{
				Timestamp: timestamp,
				Address:   node.IP.String(),
				LastSeen:  node.LastSeen.UTC().Unix(),
				Latency:   int(node.Latency),
			})
			if err != nil {
				log.Errorf("Error in saving node info, %s.", err.Error())
			} else {
				mtx.Lock()
				count++
				if node.CurrentHeight > bestBlockHeight {
					bestBlockHeight = node.CurrentHeight
				}

				snapshot := SnapShot{
					Timestamp: timestamp,
					Height:    bestBlockHeight,
				}

				snapshot.NodeCount = len(amgr.nodes)
				err = t.dataStore.SaveSnapshot(ctx, snapshot)
				if err != nil {
					// todo delete all the related node info
					t.dataStore.DeleteSnapshot(ctx, timestamp)
					log.Errorf("Error in saving network snapshot, %s", err.Error())
				}

				mtx.Unlock()
				if amgr.showDetailedLog {
					log.Infof("New heartbeat recorded for node: %s, %s, %d", node.IP.String(),
						node.UserAgent, node.ProtocolVersion)
				}
			}

		case attemptedPeer := <-amgr.attemptNtfn:
			if err := t.dataStore.AttemptPeer(ctx, attemptedPeer.IP.String(), attemptedPeer.Time); err != nil {
				log.Errorf("Error in saving peer attempt for %s, %s", attemptedPeer.IP.String(), err.Error())
			}

		case ip := <-amgr.connFailNtfn:
			if err := t.dataStore.RecordNodeConnectionFailure(ctx, ip.String(), t.cfg.MaxPeerConnectionFailure); err != nil {
				log.Errorf("Error in failed connection attempt for %s, %s", ip.String(), err.Error())
			}

		case <-ctx.Done():
			log.Info("Shutting down network seeder")
			amgr.quit <- struct{}{}
			return
		}
	}
}

func (t taker) geolocation(ctx context.Context, ip net.IP) (*IPInfo, error) {
	url := fmt.Sprintf("http://api.ipstack.com/%s?access_key=%s&format=1", ip.String(), t.cfg.IpStackAccessKey)
	var geo IPInfo
	err := helpers.GetResponse(ctx, &http.Client{Timeout: 3 * time.Second}, url, &geo)
	return &geo, err
}
