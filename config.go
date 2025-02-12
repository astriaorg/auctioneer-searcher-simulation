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
	// Ensure a subcommand is provided.
	if len(os.Args) < 2 {
		fmt.Println("expected smoke-test subcommand")
		os.Exit(1)
	}

	if os.Args[1] != "smoke-test" {
		fmt.Println("expected 'smoke-test' subcommand")
		os.Exit(1)
	}

	// Create a new FlagSet for the 'calc' subcommand.
	smokeTestCmd := flag.NewFlagSet("smoke-test", flag.ExitOnError)
	// Define flags for operation and operands.
	// TODO - implement flag.Value interface for urls
	sequencerUrl := smokeTestCmd.String("sequencer-url", "", "The grpc url of the sequencer node")
	ethRpcUrl := smokeTestCmd.String("eth-rpc-url", "", "The rpc url of the ethereum node")
	searcherPrivateKey := smokeTestCmd.String("searcher-private-key", "", "The searcher eth private key")
	rollupName := smokeTestCmd.String("rollup-name", "", "The name of the rollup")
	addressToSend := smokeTestCmd.String("address-to-send", "", "The address to send the funds to in hex")
	amountToSend := smokeTestCmd.String("amount-to-send", "", "The amount to send in gwei")

	err := smokeTestCmd.Parse(os.Args[2:])
	if err != nil {
		fmt.Println("error parsing flags")
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
