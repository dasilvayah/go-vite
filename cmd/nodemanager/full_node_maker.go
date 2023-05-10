package nodemanager

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/urfave/cli.v1"

	"github.com/vitelabs/go-vite/v2/cmd/utils"
	"github.com/vitelabs/go-vite/v2/common"
	"github.com/vitelabs/go-vite/v2/log15"
	"github.com/vitelabs/go-vite/v2/node"
	nodeconfig "github.com/vitelabs/go-vite/v2/node/config"
)

var defaultNodeConfigFileName = "node_config.json"

type FullNodeMaker struct {
}

func (maker FullNodeMaker) MakeNode(ctx *cli.Context) (*node.Node, error) {
	// 1: Make Node.Config
	nodeConfig, err := maker.MakeNodeConfig(ctx)
	if err != nil {
		return nil, err
	}
	log.Info(fmt.Sprintf("NodeConfig info: %v", nodeConfig))
	// 2: New Node
	node, err := node.New(nodeConfig)
	if err != nil {
		log.Error("Failed to create the node: %v", err)
		return nil, err
	}
	return node, nil
}

func (maker FullNodeMaker) MakeNodeConfig(ctx *cli.Context) (*nodeconfig.Config, error) {
	cfg := nodeconfig.DefaultNodeConfig
	log.Info(fmt.Sprintf("DefaultNodeconfig: %v", cfg))

	// 1: Load config file.
	err := loadNodeConfigFromFile(ctx, &cfg)
	if err != nil {
		return nil, err
	}
	log.Info(fmt.Sprintf("After load config file: %v", cfg))

	// 2: Apply flags, Overwrite the configuration file configuration
	mappingNodeConfig(ctx, &cfg)
	log.Info(fmt.Sprintf("After mapping cmd input: %v", cfg))

	// 3: Override any default configs for hard coded networks.
	overrideNodeConfigs(ctx, &cfg)
	log.Info(fmt.Sprintf("Last override config: %v", cfg))
	log.Info(fmt.Sprintf("NodeServer.DataDir:%v", cfg.DataDir))
	log.Info(fmt.Sprintf("NodeServer.KeyStoreDir:%v", cfg.KeyStoreDir))

	// 4: Config log to file
	makeRunLogFile(&cfg)

	return &cfg, nil
}

// SetNodeConfig applies node-related command line flags to the config.
func mappingNodeConfig(ctx *cli.Context, cfg *nodeconfig.Config) {

	//Global Config
	if dataDir := ctx.GlobalString(utils.DataDirFlag.Name); len(dataDir) > 0 {
		cfg.DataDir = dataDir
	}

	//Wallet
	if keyStoreDir := ctx.GlobalString(utils.KeyStoreDirFlag.Name); len(keyStoreDir) > 0 {
		cfg.KeyStoreDir = keyStoreDir
	}

	//Network Config
	if identity := ctx.GlobalString(utils.IdentityFlag.Name); len(identity) > 0 {
		cfg.Identity = identity
	}

	if ctx.GlobalIsSet(utils.MaxPeersFlag.Name) {
		cfg.MaxPeers = ctx.GlobalInt(utils.MaxPeersFlag.Name)
	}

	if ctx.GlobalIsSet(utils.MaxPendingPeersFlag.Name) {
		cfg.MaxPendingPeers = ctx.GlobalInt(utils.MaxPendingPeersFlag.Name)
	}

	if nodeKeyHex := ctx.GlobalString(utils.NodeKeyHexFlag.Name); len(nodeKeyHex) > 0 {
		cfg.SetPrivateKey(nodeKeyHex)
	}

	//Ipc Config
	if ctx.GlobalIsSet(utils.IPCEnabledFlag.Name) {
		cfg.IPCEnabled = ctx.GlobalBool(utils.IPCEnabledFlag.Name)
	}

	if ipcPath := ctx.GlobalString(utils.IPCPathFlag.Name); len(ipcPath) > 0 {
		cfg.IPCPath = ipcPath
	}

	//Http Config
	if ctx.GlobalIsSet(utils.RPCEnabledFlag.Name) {
		cfg.RPCEnabled = ctx.GlobalBool(utils.RPCEnabledFlag.Name)
	}

	if httpHost := ctx.GlobalString(utils.RPCListenAddrFlag.Name); len(httpHost) > 0 {
		cfg.HttpHost = httpHost
	}

	if ctx.GlobalIsSet(utils.RPCPortFlag.Name) {
		cfg.HttpPort = ctx.GlobalInt(utils.RPCPortFlag.Name)
	}

	//WS Config
	if ctx.GlobalIsSet(utils.WSEnabledFlag.Name) {
		cfg.WSEnabled = ctx.GlobalBool(utils.WSEnabledFlag.Name)
	}

	if wsListenAddr := ctx.GlobalString(utils.WSListenAddrFlag.Name); len(wsListenAddr) > 0 {
		cfg.WSHost = wsListenAddr
	}

	if ctx.GlobalIsSet(utils.WSPortFlag.Name) {
		cfg.WSPort = ctx.GlobalInt(utils.WSPortFlag.Name)
	}

	//Producer Config
	if coinBase := ctx.GlobalString(utils.CoinBaseFlag.Name); len(coinBase) > 0 {
		cfg.CoinBase = coinBase
	}

	if ctx.GlobalIsSet(utils.MinerFlag.Name) {
		cfg.MinerEnabled = ctx.GlobalBool(utils.MinerFlag.Name)
	}

	//Log Level Config
	if logLevel := ctx.GlobalString(utils.LogLvlFlag.Name); len(logLevel) > 0 {
		cfg.LogLevel = logLevel
	}

	//VM
	if ctx.GlobalIsSet(utils.VMTestFlag.Name) {
		cfg.VMTestEnabled = ctx.GlobalBool(utils.VMTestFlag.Name)
	}
	if ctx.GlobalIsSet(utils.VMTestParamFlag.Name) {
		cfg.VMTestParamEnabled = ctx.GlobalBool(utils.VMTestParamFlag.Name)
	}
	if ctx.GlobalIsSet(utils.QuotaTestParamFlag.Name) {
		cfg.QuotaTestParamEnabled = ctx.GlobalBool(utils.QuotaTestParamFlag.Name)
	}
	if ctx.GlobalIsSet(utils.VMDebugFlag.Name) {
		cfg.VMDebug = ctx.GlobalBool(utils.VMDebugFlag.Name)
	}

	// Subscribe
	if ctx.GlobalIsSet(utils.SubscribeFlag.Name) {
		cfg.SubscribeEnabled = ctx.GlobalBool(utils.SubscribeFlag.Name)
	}

	//Net
	if ctx.GlobalIsSet(utils.SingleFlag.Name) {
		cfg.Single = ctx.GlobalBool(utils.SingleFlag.Name)
	}
}

func overrideNodeConfigs(ctx *cli.Context, cfg *nodeconfig.Config) {

	if len(cfg.DataDir) == 0 || cfg.DataDir == "" {
		cfg.DataDir = common.DefaultDataDir()
	}

	if len(cfg.KeyStoreDir) == 0 || cfg.KeyStoreDir == "" {
		cfg.KeyStoreDir = cfg.DataDir
	}

	if len(cfg.LogLevel) == 0 {
		cfg.LogLevel = "info"
	}

	if ctx.GlobalBool(utils.MainNetFlag.Name) || cfg.NetID == 1 {
		cfg.NetSelect = "main"
		if cfg.NetID != 1 {
			cfg.NetID = 1
		}
		cfg.DataDir = filepath.Join(cfg.DataDir, "maindata")
		cfg.KeyStoreDir = filepath.Join(cfg.KeyStoreDir, "maindata", "wallet")
		cfg.DataDirPathAbs()
		return
	}

	if ctx.GlobalBool(utils.TestNetFlag.Name) || cfg.NetID == 2 {
		cfg.NetSelect = "test"
		if cfg.NetID != 2 {
			cfg.NetID = 2
		}
		cfg.DataDir = filepath.Join(cfg.DataDir, "testdata")
		cfg.KeyStoreDir = filepath.Join(cfg.KeyStoreDir, "testdata", "wallet")
		cfg.DataDirPathAbs()
		return
	}

	if ctx.GlobalBool(utils.DevNetFlag.Name) || cfg.NetID > 2 || cfg.NetID < 1 {
		cfg.NetSelect = "dev"
		//network override
		if cfg.NetID < 3 {
			cfg.NetID = 3
		}
		//dataDir override
		cfg.DataDir = filepath.Join(cfg.DataDir, "devdata")
		cfg.KeyStoreDir = filepath.Join(cfg.KeyStoreDir, "devdata", "wallet")
		//abs dataDir
		cfg.DataDirPathAbs()
		return
	}
}

func loadNodeConfigFromFile(ctx *cli.Context, cfg *nodeconfig.Config) error {

	configFile := ctx.GlobalString(utils.ConfigFileFlag.Name)
	if configFile == "" {
		configFile = defaultNodeConfigFileName
	}

	err := cfg.ParseFromFile(configFile)
	if err != nil {
		log.Error("load node config fail", "configFile", configFile, "err", err)
		common.Crit(fmt.Sprintf("%v\n", err))
	}
	return nil
}

func IsExist(f string) bool {
	_, err := os.Stat(f)
	return err == nil || os.IsExist(err)
}

func makeRunLogFile(cfg *nodeconfig.Config) {
	defaultHandler := common.LogHandler(cfg.RunLogDir(), "", "vite.log", cfg.LogLevel)
	errorHandler := common.LogHandler(cfg.RunLogDir(), "error", "vite.error.log", log15.LvlError.String())

	log15.Root().SetHandler(log15.MultiHandler(defaultHandler, errorHandler))
}
