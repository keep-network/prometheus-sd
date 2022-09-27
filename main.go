// Copyright 2018 The Prometheus Authors
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
//
// This implementation is based on an [example] from the Prometheus documentation.
//
// example: https://github.com/prometheus/prometheus/blob/734772f82824db11344ea3c39a166449d0e7e468/documentation/examples/custom-sd/adapter-usage/main.go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"

	"os"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"golang.org/x/exp/slices"

	"github.com/keep-network/keep-core/pkg/clientinfo"

	"github.com/prometheus/common/model"
	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/prometheus/prometheus/discovery/targetgroup"
	"github.com/prometheus/prometheus/documentation/examples/custom-sd/adapter"

	"github.com/keep-network/prometheus-sd/internal/utils"
)

var (
	app = kingpin.New(
		"Keep Network Nodes Discovery for Prometheus",
		"Tool to discover Keep Network Nodes and export them to a file_sd target file for Prometheus.",
	)
	config = &sdConfig{}
	logger log.Logger

	scanPortRangeFlagValue string

	excludedAddresses = []string{"127.0.0.1"}

	labelChainAddress = model.MetaLabelPrefix + "chain_address"
	labelNetworkID    = model.MetaLabelPrefix + "network_id"
)

type sdConfig struct {
	outputFile      string
	listenAddresses []string

	refreshInterval time.Duration

	diagnosticsPortRange utils.Range
	scanPortTimeout      time.Duration

	getDiagnosticsTimeout time.Duration

	logJson bool
}

type peerData struct {
	clientinfo.Peer
	ClientInfoEndpoint string
}

type discovery struct {
	peers map[string]*peerData

	oldSourceList map[string]bool
}

func init() {
	app.Flag(
		"output.file",
		"Output file for file_sd compatible file.",
	).Default("keep_sd.json").StringVar(&config.outputFile)

	app.Flag(
		"source.address",
		"The address of Keep Network Bootstrap Node to discover the list of peers from.",
	).Default("localhost:9701").StringsVar(&config.listenAddresses)

	app.Flag(
		"refresh.interval",
		"Frequency for running the discovery.",
	).Default("5m").DurationVar(&config.refreshInterval)

	app.Flag(
		"scan.range",
		"A port range for diagnostics endpoint port scan.",
	).Default("9701-9799").StringVar(&scanPortRangeFlagValue)

	app.Flag(
		"scan.timeout",
		"Timeout for single port scan.",
	).Default("1s").DurationVar(&config.scanPortTimeout)

	app.Flag(
		"diagnostics.timeout",
		"Timeout for diagnostics endpoint call.",
	).Default("5s").DurationVar(&config.getDiagnosticsTimeout)

	app.Flag(
		"log.json",
		"Output logs in JSON format.",
	).Default("false").BoolVar(&config.logJson)
}

func newDiscovery() (*discovery, error) {
	var err error
	config.diagnosticsPortRange, err = utils.NewRange(scanPortRangeFlagValue)
	if err != nil {
		return nil, fmt.Errorf("invalid port range value provided %s: %v", scanPortRangeFlagValue, err)
	}

	cd := &discovery{
		peers:         make(map[string]*peerData),
		oldSourceList: make(map[string]bool),
	}
	return cd, nil
}

func (d *discovery) collectDiagnostics(addresses []string) []clientinfo.Diagnostics {
	var allDiagnostics = make([]clientinfo.Diagnostics, 0)

	for _, address := range addresses {
		level.Info(logger).Log(
			"msg", fmt.Sprintf("collecting diagnostics from source %s", address),
		)

		diagnostics, err := getDiagnostics(address)
		if err != nil {
			level.Error(logger).Log(
				"msg", "failed to get diagnostics",
				"from", address,
				"err", err,
			)
			continue
		}

		allDiagnostics = append(allDiagnostics, diagnostics)
	}

	return allDiagnostics
}

func (d *discovery) combineDiscoveredPeers(
	allDiagnostics []clientinfo.Diagnostics,
) []clientinfo.Peer {
	var peersNetworkIDs = make(map[string]string, 0)                // chain address -> network id
	var peersAddressesSet = make(map[string]map[string]struct{}, 0) // chain address -> []network addresses set
	var peersSet = make([]clientinfo.Peer, 0)

	for _, diagnostics := range allDiagnostics {
		for _, peer := range diagnostics.ConnectedPeers {
			// Check for chain address vs network id mismatch for peer resolved from
			// previous diagnostics source - this should never be true.
			if peersNetworkIDs[peer.ChainAddress] != "" &&
				peersNetworkIDs[peer.ChainAddress] != peer.NetworkID {
				level.Warn(logger).Log(
					"msg", "previously resolved network ID for the peer doesn't match",
					"peer", peer.ChainAddress,
					"previous", peersNetworkIDs[peer.ChainAddress],
					"current", peer.NetworkID,
				)
				continue
			} else {
				peersNetworkIDs[peer.ChainAddress] = peer.NetworkID
			}

			// In case diagnostics sources know different addresses for the peer
			// we want to combine them in a set.
			for _, peerMultiAddress := range peer.NetworkMultiAddresses {
				peerAddress, err := utils.ExtractAddressFromMultiAddress(peerMultiAddress)
				if err != nil {
					level.Error(logger).Log(
						"msg", "failed to extract peer address from multi address",
						"peer", peer.ChainAddress,
						"multiaddress", peerMultiAddress,
					)
					continue
				}

				if _, ok := peersAddressesSet[peer.ChainAddress]; !ok {
					peersAddressesSet[peer.ChainAddress] = make(map[string]struct{})
				}

				peersAddressesSet[peer.ChainAddress][peerAddress] = struct{}{}
			}
		}
	}

	// Go doesn't support sets directly so we need to use intermediate mapping
	// to gather the results. Here we convert the mapping to a slice that will
	// be considered a set.
	for chainAddress, networkAddresses := range peersAddressesSet {
		networkAddressesSet := make([]string, 0, len(networkAddresses))
		for k := range networkAddresses {
			networkAddressesSet = append(networkAddressesSet, k)
		}
		// TODO: Sort addresses to start endpoint resolving with `dns` addresses
		// and move addresses looking like internal to the end of the list.

		peersSet = append(peersSet, clientinfo.Peer{
			ChainAddress:          chainAddress,
			NetworkID:             peersNetworkIDs[chainAddress],
			NetworkMultiAddresses: networkAddressesSet,
		})
	}

	return peersSet
}

// Convert a peer details to a Prometheus' target.
func (p *peerData) createPeerTarget() (targetGroup targetgroup.Group) {
	targetGroup.Source = p.ChainAddress // TODO: Maybe we should use endpoint here?

	targetGroup.Targets = []model.LabelSet{
		{
			model.AddressLabel: model.LabelValue(p.ClientInfoEndpoint),
		},
	}
	targetGroup.Labels = model.LabelSet{
		model.AddressLabel:                 model.LabelValue(p.ClientInfoEndpoint),
		model.LabelName(labelChainAddress): model.LabelValue(p.ChainAddress),
		model.LabelName(labelNetworkID):    model.LabelValue(p.NetworkID),
	}
	return
}

func getDiagnostics(addressWithPort string) (clientinfo.Diagnostics, error) {
	var diagnostics clientinfo.Diagnostics
	client := http.Client{
		Timeout: config.getDiagnosticsTimeout,
	}

	if addressWithPort == "" {
		return diagnostics, fmt.Errorf("address is empty")
	}

	resp, err := client.Get(fmt.Sprintf("http://%s/diagnostics", addressWithPort))
	if err != nil {
		return diagnostics, fmt.Errorf("failed to get diagnostics: %v", err)
	}

	if err := json.NewDecoder(resp.Body).Decode(&diagnostics); err != nil {
		return diagnostics, fmt.Errorf("failed to decode diagnostics: %v", err)
	}

	return diagnostics, nil
}

// Run is an implementation of the Discovery interface.
func (d *discovery) Run(ctx context.Context, ch chan<- []*targetgroup.Group) {
discoveryLoop:
	for c := time.Tick(config.refreshInterval); ; {
		// Get diagnostics from the source nodes (bootstrap nodes) to resolve
		// the list of connected peers.
		sourceDiagnostics := d.collectDiagnostics(config.listenAddresses)

		// Combine results received from the source nodes to resolve a set of unique
		// peers.
		discoveredPeers := d.combineDiscoveredPeers(sourceDiagnostics)

		level.Info(logger).Log(
			"msg", fmt.Sprintf("discovered %d connected peers", len(discoveredPeers)),
		)
		level.Debug(logger).Log(
			"discoveredPeers", fmt.Sprintf("%+v", discoveredPeers),
		)

		// TODO: Try use https://github.com/Ullaakut/nmap for ports scanning

		// network address -> chain address -> port
		discoveredPorts := make(map[string]map[string]int)

	peerLoop:
		for _, discoveredPeer := range discoveredPeers {
			level.Info(logger).Log(
				"msg", "resolving diagnostics endpoint target for peer",
				"peer", discoveredPeer.ChainAddress,
			)

			// Register discovered peer's details.
			if _, ok := d.peers[discoveredPeer.ChainAddress]; !ok {
				d.peers[discoveredPeer.ChainAddress] = &peerData{}
			}
			d.peers[discoveredPeer.ChainAddress].Peer = discoveredPeer

			peer := d.peers[discoveredPeer.ChainAddress]

			// Check if the already known endpoint still works.
			if peer.ClientInfoEndpoint != "" {
				diagnostics, err := getDiagnostics(peer.ClientInfoEndpoint)
				if err == nil {
					if peer.ChainAddress == diagnostics.ClientInfo.ChainAddress {
						level.Info(logger).Log(
							"msg", "already known endpoint still works",
							"peer", discoveredPeer.ChainAddress,
							"endpoint", peer.ClientInfoEndpoint,
						)
						// The endpoint still works, move to the next peer.
						continue peerLoop
					}
				}

				level.Warn(logger).Log(
					"msg", "already known endpoint doesn't work",
					"peer", discoveredPeer.ChainAddress,
					"endpoint", peer.ClientInfoEndpoint,
				)
			}

			level.Info(logger).Log(
				"msg", "starting diagnostics ports scanning",
				"peer", discoveredPeer.ChainAddress,
			)

			// Loop all discovered network addresses of the peer.
		addressLoop:
			for _, networkAddress := range discoveredPeer.NetworkMultiAddresses {
				// Check if the network address is excluded (e.g. it's an internal address).
				if slices.Contains(excludedAddresses, networkAddress) {
					// The address is excluded, continue to the next discovered
					// peer's network address.
					continue addressLoop
				}

				if _, ok := discoveredPorts[networkAddress]; !ok {
					discoveredPorts[networkAddress] = make(map[string]int)
				}

				// Check if the network address is reachable.
				isReachable := true // TODO: Implement check if the address is reachable.
				if !isReachable {
					// The address is not reachable, continue to the next discovered
					// peer's network address.
					continue addressLoop
				}

				checkPort := func(port int) error {
					// Check if the port is open.
					if !utils.IsPortOpen("tcp", networkAddress, port, config.scanPortTimeout) {
						return fmt.Errorf("port %d is not open", port)
					}

					// The port is open, check if this is the correct diagnostics
					// endpoint for the peer.

					endpoint := net.JoinHostPort(networkAddress, fmt.Sprintf("%d", port))
					diagnostics, err := getDiagnostics(endpoint)
					if err != nil {
						return fmt.Errorf("failed to get diagnostics: %v", err)
					}

					// Store discovered port to use for discovery of other peers
					// running at the same address.
					discoveredPorts[networkAddress][diagnostics.ClientInfo.ChainAddress] = port

					// Check if this port serves diagnostics for the peer we're
					// looking for.
					if discoveredPeer.ChainAddress != diagnostics.ClientInfo.ChainAddress {
						return fmt.Errorf(
							"port serves another peer: %s", diagnostics.ClientInfo.ChainAddress,
						)
					}

					// We've got a correct diagnostics target endpoint for the peer.
					peer.ClientInfoEndpoint = endpoint
					return nil
				}

				// TODO: Test this on test environment when multiple nodes are
				// running at the same address.
				// Check if a port has been already discovered when looping ports
				// for another peer. This case is path is meant for peers running
				// sharing the same network address under different ports.
				if port, ok := discoveredPorts[networkAddress][peer.ChainAddress]; ok {
					err := checkPort(port)
					if err == nil {
						level.Info(logger).Log(
							"msg", "found diagnostics port",
							"peer", peer.ChainAddress,
							"address", networkAddress,
							"port", port,
						)
						// We've got correct address and port for the peer; move to another peer.
						continue peerLoop
					}
					level.Warn(logger).Log(
						"msg", "failed to check port",
						"address", networkAddress,
						"port", port, "err", err,
					)
					// The port is not correct; proceed to the ports scanning loop.
				}

				// Scan ports range.
			portLoop:
				for port := config.diagnosticsPortRange.Start; port <= config.diagnosticsPortRange.End; port++ {
					level.Debug(logger).Log("msg", "scanning port", "peer", discoveredPeer.ChainAddress, "address", networkAddress, "port", port)

					err := checkPort(port)
					if err != nil {
						level.Warn(logger).Log("msg", "failed to check port", "address", networkAddress, "port", port, "err", err)
						continue portLoop
					}
					level.Info(logger).Log("msg", "found diagnostics port", "peer", discoveredPeer.ChainAddress, "address", networkAddress, "port", port)

					// We've got correct address and port for the peer, let's resolve another peer.
					continue peerLoop
				}
			}

			level.Error(logger).Log("msg", "failed to find diagnostics port", "peer", discoveredPeer.ChainAddress, "multiaddresses", discoveredPeer.NetworkMultiAddresses)
		}

		// Note that we treat errors when querying specific node as fatal for this
		// iteration of the time.Tick loop. It's better to have some stale targets than an incomplete
		// list of targets simply because there may have been a timeout. If the service is actually
		// gone as far as consul is concerned, that will be picked up during the next iteration of
		// the outer loop.

		newSourceList := make(map[string]bool)

		tgs := make([]*targetgroup.Group, len(d.peers))
		for _, peer := range d.peers {
			target := peer.createPeerTarget()
			tgs = append(tgs, &target)

			newSourceList[target.Source] = true
		}

		// TODO: Test if it works as expected
		// When a target disappears, send an update with empty targetList.
		for key := range d.oldSourceList {
			if !newSourceList[key] {
				tgs = append(tgs, &targetgroup.Group{
					Source: key,
				})
			}
		}
		d.oldSourceList = newSourceList

		level.Info(logger).Log("msg", "discovery round completed")

		// We're returning all peer nodes targets as a single target group.
		ch <- tgs

		// Wait for ticker to start a next discovery round or exit when ctx is closed.
		select {
		case <-c:
			continue discoveryLoop
		case <-ctx.Done():
			return
		}
	}
}

func main() {
	app.HelpFlag.Short('h')

	_, err := app.Parse(os.Args[1:])
	if err != nil {
		fmt.Println("err: ", err)
		return
	}

	var baseLogger log.Logger
	if config.logJson {
		baseLogger = log.NewJSONLogger(os.Stdout)
	} else {
		baseLogger = log.NewLogfmtLogger(os.Stdout)
	}

	logger = log.NewSyncLogger(baseLogger)
	logger = log.With(logger, "ts", log.DefaultTimestampUTC)

	ctx := context.Background()

	disc, err := newDiscovery()
	if err != nil {
		panic(fmt.Errorf("failed to initiate discovery: %v", err))
	}
	sdAdapter := adapter.NewAdapter(ctx, config.outputFile, "keepNetworkPeerSD", disc, logger)
	fmt.Printf("FILE: %s\n", config.outputFile)
	sdAdapter.Run()

	<-ctx.Done()
}

// TODO: Test what happens if bootstraps are down
// TODO: Test what happens when one of the nodes is down
