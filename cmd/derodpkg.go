package derodpkg

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	"github.com/chzyer/readline"
	"github.com/docopt/docopt-go"

	"github.com/DEROFDN/derohe/block"
	"github.com/DEROFDN/derohe/blockchain"
	derodrpc "github.com/DEROFDN/derohe/cmd/derod/rpc"
	"github.com/DEROFDN/derohe/config"
	"github.com/DEROFDN/derohe/globals"
	"github.com/DEROFDN/derohe/p2p"

	"github.com/go-logr/logr"
	"gopkg.in/natefinch/lumberjack.v2"
)

var commandLine string = `derod 
DERO : A secure, private blockchain with smart-contracts

Usage:
  derod [--help] [--version] [--testnet] [--debug]  [--sync-node] [--timeisinsync] [--fastsync] [--socks-proxy=<socks_ip:port>] [--data-dir=<directory>] [--p2p-bind=<0.0.0.0:18089>] [--add-exclusive-node=<ip:port>]... [--add-priority-node=<ip:port>]... 	 [--min-peers=<11>] [--rpc-bind=<127.0.0.1:9999>] [--getwork-bind=<0.0.0.0:18089>] [--node-tag=<unique name>] [--prune-history=<50>] [--integrator-address=<address>] [--clog-level=1] [--flog-level=1]
  derod -h | --help
  derod --version

Options:
  -h --help     Show this screen.
  --version     Show version.
  --testnet  	Run in testnet mode.
  --debug       Debug mode enabled, print more log messages
  --clog-level=1	Set console log level (0 to 127) 
  --flog-level=1	Set file log level (0 to 127)
  --fastsync      Fast sync mode (this option has effect only while bootstrapping)
  --timeisinsync  Confirms to daemon that time is in sync, so daemon doesn't try to sync
  --socks-proxy=<socks_ip:port>  Use a proxy to connect to network.
  --data-dir=<directory>    Store blockchain data at this location
  --rpc-bind=<127.0.0.1:9999>    RPC listens on this ip:port
  --p2p-bind=<0.0.0.0:18089>    p2p server listens on this ip:port, specify port 0 to disable listening server
  --getwork-bind=<0.0.0.0:10100>    getwork server listens on this ip:port, specify port 0 to disable listening server
  --add-exclusive-node=<ip:port>	Connect to specific peer only 
  --add-priority-node=<ip:port>	Maintain persistant connection to specified peer
  --sync-node       Sync node automatically with the seeds nodes. This option is for rare use.
  --node-tag=<unique name>	Unique name of node, visible to everyone
  --integrator-address	if this node mines a block,Integrator rewards will be given to address.default is dev's address.
  --prune-history=<50>	prunes blockchain history until the specific topo_height
`

func filterInput(r rune) (rune, bool) {
	switch r {
	case readline.CharCtrlZ:
		return r, false
	}
	return r, true
}

// Daemon wraps the DERO daemon as a service instance.
type Daemon struct {
	params      map[string]interface{}
	chain       *blockchain.Blockchain
	rpcserver   *derodrpc.RPCServer
	logger      logr.Logger
	rl          *readline.Instance
	initialized bool
	started     bool
}

// NewDaemon creates a new Daemon instance with the provided parameters.
func NewDaemon(initparams map[string]interface{}) (*Daemon, error) {
	if initparams == nil {
		initparams = make(map[string]interface{})
	}
	return &Daemon{
		params: initparams,
		logger: logr.Discard(),
	}, nil
}

// Initialize sets up logging, network, and starts the blockchain.
// Must be called before Start.
func (d *Daemon) Initialize() error {
	runtime.MemProfileRate = 0

	if d.params["--testnet"] == nil {
		d.params["--testnet"] = false
	}

	args, err := docopt.Parse(commandLine, nil, true, config.Version.String(), false) //nolint:staticcheck // SA1019 - keeping for globals.Arguments compatibility
	if err != nil {
		return fmt.Errorf("error parsing command line: %w", err)
	}
	globals.Arguments = args

	for k, v := range d.params {
		globals.Arguments[k] = v
	}

	d.rl, err = readline.NewEx(&readline.Config{
		Prompt:              "\033[92mDERO:\033[32m>>>",
		HistoryFile:         filepath.Join(os.TempDir(), "derod_readline.tmp"),
		AutoComplete:        readline.NewPrefixCompleter(),
		InterruptPrompt:     "^C",
		EOFPrompt:           "exit",
		HistorySearchFold:   true,
		FuncFilterInputRune: filterInput,
	})
	if err != nil {
		return fmt.Errorf("error starting readline: %w", err)
	}

	network := "mainnet"
	if !globals.IsMainnet() {
		network = "testnet"
	}

	exename, err := os.Executable()
	if err != nil {
		return fmt.Errorf("error getting executable path: %w", err)
	}

	globals.InitializeLog(d.rl.Stdout(), &lumberjack.Logger{
		Filename:   exename + "_daemon_" + network + ".log",
		MaxSize:    100,
		MaxBackups: 2,
	})

	d.logger = d.logger.WithName("derod")
	d.logger.Info("DERO HE daemon: It is an alpha version, use it for testing/evaluations purpose only.")
	d.logger.Info("Copyright 2017-2021 DERO Project. All rights reserved.")
	d.logger.Info("", "OS", runtime.GOOS, "ARCH", runtime.GOARCH, "GOMAXPROCS", runtime.GOMAXPROCS(0))
	d.logger.Info("", "Version", config.Version.String())
	d.logger.V(1).Info("", "Arguments", globals.Arguments)

	globals.Initialize()
	d.logger.V(0).Info("", "MODE", globals.Config.Name)
	d.logger.V(0).Info("", "Daemon data directory", globals.GetDataDirectory())

	if pruneHistoryStr, ok := globals.Arguments["--prune-history"].(string); ok && pruneHistoryStr != "" {
		pruneTopo, err := strconv.ParseInt(pruneHistoryStr, 10, 64)
		if err != nil {
			d.logger.Error(err, "error parsing --prune-history")
			return fmt.Errorf("invalid --prune-history: %w", err)
		}
		if pruneTopo <= 1 {
			return fmt.Errorf("--prune-history should be positive and more than 1")
		}
		d.logger.Info("will prune history till", "topo_height", pruneTopo)

		if err := blockchain.Prune_Blockchain(pruneTopo); err != nil {
			d.logger.Error(err, "error pruning blockchain")
			return fmt.Errorf("error pruning blockchain: %w", err)
		}
		d.logger.Info("blockchain pruning successful")
	}

	if timeInSync, ok := globals.Arguments["--timeisinsync"].(bool); ok {
		globals.TimeIsInSync = timeInSync
	}

	if _, ok := globals.Arguments["--integrator-address"]; ok {
		d.params["--integrator-address"] = globals.Arguments["--integrator-address"]
	}

	d.chain, err = blockchain.Blockchain_Start(d.params)
	if err != nil {
		d.logger.Error(err, "error starting blockchain")
		return fmt.Errorf("error starting blockchain: %w", err)
	}
	d.params["chain"] = d.chain

	if globals.Arguments["--socks-proxy"] != nil {
		globals.Arguments["--p2p-bind"] = ":0"
		d.logger.Info("Disabling P2P server since we are using socks proxy")
	}

	d.initialized = true
	return nil
}

// Start initializes P2P, starts the RPC server, and begins background goroutines.
// Requires Initialize() to be called first.
func (d *Daemon) Start() error {
	if !d.initialized {
		return fmt.Errorf("daemon must be initialized before starting")
	}
	if d.started {
		return fmt.Errorf("daemon already started")
	}

	if err := p2p.P2P_Init(d.params); err != nil {
		return fmt.Errorf("error initializing P2P: %w", err)
	}

	var err error
	d.rpcserver, err = derodrpc.RPCServer_Start(d.params)
	if err != nil {
		return fmt.Errorf("error starting RPC server: %w", err)
	}

	go derodrpc.Getwork_server()

	d.chain.P2P_Block_Relayer = func(cbl *block.Complete_Block, peerid uint64) {
		p2p.Broadcast_Block(cbl, peerid)
	}
	d.chain.P2P_MiniBlock_Relayer = func(mbl block.MiniBlock, peerid uint64) {
		p2p.Broadcast_MiniBlock(mbl, peerid)
	}

	// Corruption fix loop
	{
		currentBlid, err := d.chain.Load_Block_Topological_order_at_index(17600)
		if err == nil {
			for {
				height := d.chain.Load_Height_for_BL_ID(currentBlid)
				if height < 17500 {
					break
				}

				r, err := d.chain.Store.Topo_store.Read(int64(height))
				if err != nil {
					return fmt.Errorf("error reading topo store: %w", err)
				}
				if r.BLOCK_ID != currentBlid {
					d.logger.Info("fixing corruption", "r", r, "current_blid", currentBlid, "height", height)

					fixCommitVersion, err := d.chain.ReadBlockSnapshotVersion(currentBlid)
					if err != nil {
						return fmt.Errorf("error reading block snapshot version: %w", err)
					}

					if err := d.chain.Store.Topo_store.Write(int64(height), currentBlid, fixCommitVersion, int64(height)); err != nil {
						return fmt.Errorf("error writing topo store: %w", err)
					}
				}

				fixBl, err := d.chain.Load_BL_FROM_ID(currentBlid)
				if err != nil {
					return fmt.Errorf("error loading block from ID: %w", err)
				}
				currentBlid = fixBl.Tips[0]
			}
		}
	}

	globals.Cron.Start()

	go func() {
		for {
			derodrpc.CountMiners()
			time.Sleep(1 * time.Second)
		}
	}()

	setPasswordCfg := d.rl.GenPasswordConfig()
	setPasswordCfg.SetListener(func(line []rune, pos int, key rune) (newLine []rune, newPos int, ok bool) {
		d.rl.SetPrompt(fmt.Sprintf("Enter password(%v): ", len(line)))
		d.rl.Refresh()
		return nil, 0, false
	})
	d.rl.Refresh()

	d.started = true
	return nil
}

// Stop shuts down RPC, P2P, and blockchain subsystems.
// Safe to call multiple times and on nil receiver.
func (d *Daemon) Stop() error {
	if d == nil {
		return nil
	}
	if !d.started && !d.initialized {
		return nil
	}

	d.logger.Info("Exit in Progress, Please wait")
	time.Sleep(100 * time.Millisecond)

	if d.rpcserver != nil {
		d.rpcserver.RPCServer_Stop()
	}
	p2p.P2P_Shutdown()
	if d.chain != nil {
		d.chain.Shutdown()
	}

	for globals.Subsystem_Active > 0 {
		d.logger.Info("Exit in Progress, Please wait.", "active subsystems", globals.Subsystem_Active)
		time.Sleep(1000 * time.Millisecond)
	}

	if d.rl != nil {
		if err := d.rl.Close(); err != nil {
			d.logger.Error(err, "error closing readline")
		}
	}

	d.started = false
	d.initialized = false
	return nil
}

// Chain returns the blockchain instance.
func (d *Daemon) Chain() *blockchain.Blockchain {
	return d.chain
}

// RPCServer returns the RPC server instance.
func (d *Daemon) RPCServer() *derodrpc.RPCServer {
	return d.rpcserver
}

// Initialized returns whether the daemon has been initialized.
func (d *Daemon) Initialized() bool {
	return d.initialized
}

// Started returns whether the daemon has been started.
func (d *Daemon) Started() bool {
	return d.started
}

// Params returns the daemon's parameters map.
func (d *Daemon) Params() map[string]interface{} {
	return d.params
}

// GetParam retrieves a parameter value by key.
func (d *Daemon) GetParam(key string) (interface{}, bool) {
	v, ok := d.params[key]
	return v, ok
}

// SetParam sets a parameter value by key.
func (d *Daemon) SetParam(key string, value interface{}) {
	d.params[key] = value
}

// RemoveParam removes a parameter by key.
func (d *Daemon) RemoveParam(key string) {
	delete(d.params, key)
}
