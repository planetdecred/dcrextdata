// Copyright (c) 2018 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package netsnapshot

import (
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/decred/dcrd/chaincfg/v2"
	"github.com/decred/dcrd/dcrutil/v2"
	"github.com/decred/dcrd/peer/v2"
	"github.com/decred/dcrd/wire"
	"github.com/planetdecred/dcrextdata/app/config"
)

const (
	// defaultAddressTimeout defines the duration to wait
	// for new addresses.
	defaultAddressTimeout = time.Minute * 10

	// defaultNodeTimeout defines the timeout time waiting for
	// a response from a node.
	defaultNodeTimeout = time.Second * 10
)

var (
	defaultHomeDir = dcrutil.AppDataDir("dcrextdata", false)

	amgr *Manager
	wg   sync.WaitGroup
)

func creep(netParams *chaincfg.Params) {
	defer wg.Done()

	onaddr := make(chan struct{})
	verack := make(chan struct{})
	peerConfig := peer.Config{
		UserAgentName:    "dcrextdata",
		UserAgentVersion: "0.0.1",
		Net:              netParams.Net,
		DisableRelayTx:   true,

		Listeners: peer.MessageListeners{
			OnAddr: func(p *peer.Peer, msg *wire.MsgAddr) {
				n := make([]peerAddress, 0, len(msg.AddrList))
				for _, addr := range msg.AddrList {
					n = append(n, peerAddress{addr.IP, addr.Port})
				}
				added := amgr.AddAddresses(n)
				log.Debugf("Peer %v sent %v addresses, %d new", p.Addr(), len(msg.AddrList), added)
				onaddr <- struct{}{}
			},
			OnVerAck: func(p *peer.Peer, msg *wire.MsgVerAck) {
				log.Debugf("Adding peer %v with services %v", p.NA().IP.String(), p.Services())
				verack <- struct{}{}
			},
		},
	}

	var wg sync.WaitGroup
	for {
		peerAddrs := amgr.Addresses()
		if len(peerAddrs) == 0 {
			log.Infof("No stale addresses -- sleeping for %v", defaultAddressTimeout)
			time.Sleep(defaultAddressTimeout)
			continue
		}

		wg.Add(len(peerAddrs))

		for _, addr := range peerAddrs {
			go func(addr peerAddress) {
				defer wg.Done()

				port := strconv.Itoa(int(addr.Port))
				if addr.Port == 0 {
					port = netParams.DefaultPort
				}
				host := net.JoinHostPort(addr.IP.String(), port)

				p, err := peer.NewOutboundPeer(&peerConfig, host)
				if err != nil {
					log.Debugf("NewOutboundPeer on %v: %v", host, err)
					amgr.notifyFailedAttempt(addr.IP)
					return
				}

				t := time.Now()
				amgr.Attempt(addr.IP)

				conn, err := net.DialTimeout("tcp", p.Addr(),
					defaultNodeTimeout)
				if err != nil {
					log.Debugf("DialTimeout failed for %s, %s", p.Addr(), err.Error())
					amgr.notifyFailedAttempt(addr.IP)
					return
				}
				latency := time.Since(t).Seconds() * 1000
				p.AssociateConnection(conn)

				// Wait for the verack message or timeout in case of
				// failure.
				select {
				case <-verack:
					// Mark this peer as a good node.
					amgr.Good(p)
					amgr.peerNtfn <- &Node{
						IP:              addr.IP,
						Port:            addr.Port,
						Services:        p.Services(),
						LastAttempt:     time.Now().UTC(),
						LastSuccess:     time.Now().UTC(),
						LastSeen:        time.Now().UTC(),
						Latency:         int64(latency),
						ConnectionTime:  p.TimeConnected().Unix(),
						ProtocolVersion: p.ProtocolVersion(),
						UserAgent:       p.UserAgent(),
						StartingHeight:  p.StartingHeight(),
						CurrentHeight:   p.LastBlock(),
					}

					// Ask peer for some addresses.
					p.QueueMessage(wire.NewMsgGetAddr(), nil)

				case <-time.After(defaultNodeTimeout):
					log.Debugf("verack timeout on peer %v", p.Addr())
					p.Disconnect()
					return
				}

				select {
				case <-onaddr:
				case <-time.After(defaultNodeTimeout):
					log.Debugf("getaddr timeout on peer %v", p.Addr())
					p.Disconnect()
					return
				}
				p.Disconnect()
			}(addr)
		}
		wg.Wait()
	}
}

func runSeeder(cfg config.NetworkSnapshotOptions, netParams *chaincfg.Params) {
	amgr.AddAddresses([]peerAddress{{net.ParseIP(cfg.Seeder), cfg.SeederPort}})

	wg.Add(1)
	go creep(netParams)

	wg.Wait()
}
