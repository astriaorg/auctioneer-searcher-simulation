package main

import (
	"flag"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"log/slog"
	"math/big"
	"os"
)

type Config struct {
	sequencerUrl       string
	searcherPrivateKey string
	ethRpcUrl          string
	rollupName         string
	addressToSend      common.Address
	amountToSend       *big.Int
}

func readConfigFromCmdLine() (Config, error) {
	// TODO - implement flag.Value interface for urls to validate them.
	sequencerUrl := flag.String("sequencer-url", "grpc.sequencer.localdev.me", "The grpc url of the sequencer node")
	ethRpcUrl := flag.String("eth-rpc-url", "http://executor.astria.localdev.me", "The rpc url of the ethereum node")
	searcherPrivateKey := flag.String("searcher-private-key", "", "The searcher eth private key")
	rollupName := flag.String("rollup-name", "", "The name of the rollup")
	addressToSend := flag.String("address-to-send", "0x507056D8174aC4fb40Ec51578d18F853dFe000B2", "The address to send the funds to in hex")
	amountToSend := flag.String("amount-to-send", "100", "The amount to send in gwei")

	flag.Parse()

	provided := make(map[string]bool)
	flag.Visit(func(f *flag.Flag) {
		provided[f.Name] = true
	})

	missing := false
	// Check required flags.
	if !provided["searcher-private-key"] {
		fmt.Fprintln(os.Stderr, "Missing required flag: -searcher-private-key")
		missing = true
	}

	if missing {
		flag.Usage()
		os.Exit(1)
	}

	addressToSendConv := common.HexToAddress(*addressToSend)
	amountToSendConv := new(big.Int)
	amountToSendConv.SetString(*amountToSend, 10)

	return Config{
		sequencerUrl:       *sequencerUrl,
		searcherPrivateKey: *searcherPrivateKey,
		ethRpcUrl:          *ethRpcUrl,
		rollupName:         *rollupName,
		addressToSend:      addressToSendConv,
		amountToSend:       amountToSendConv,
	}, nil
}

func (c *Config) PrintConfig() {
	slog.Info("Sequencer url is:", "sequencer_url", c.sequencerUrl)
	slog.Info("Searcher private key is:", "searcher_private_key", c.searcherPrivateKey)
	slog.Info("Eth rpc url is:", "eth_rpc_url", c.ethRpcUrl)
	slog.Info("Rollup name is:", "rollup_name", c.rollupName)
	slog.Info("Address to send is:", "address_to_send", c.addressToSend)
	slog.Info("Amount to send is:", "amount_to_send", c.amountToSend)
}
