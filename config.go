package main

import (
	"fmt"
	"github.com/joho/godotenv"
	"log/slog"
	"os"
	"strconv"
)

type Config struct {
	sequencerUrl          string
	searcherPrivateKey    string
	ethRpcUrl             string
	searcherTasksToSpinUp uint64
	rollupName            string
	latencyMargin         uint64
}

func readConfigFromEnv(fileName string) (Config, error) {
	err := godotenv.Load(fileName)
	if err != nil {
		slog.Error("Error loading .env file")
		return Config{}, nil
	}

	sequencerUrl := os.Getenv("SEQUENCER_URL")
	if sequencerUrl == "" {
		slog.Error("SEQUENCER_URL is not set")
		return Config{}, fmt.Errorf("SEQUENCER_URL is not set")
	}

	searcherPrivateKey := os.Getenv("SEARCHER_PRIVATE_KEY")
	if searcherPrivateKey == "" {
		slog.Error("SEARCHER_PRIVATE_KEY is not set")
		return Config{}, fmt.Errorf("SEARCHER_PRIVATE_KEY is not set")
	}

	ethRpcUrl := os.Getenv("ETH_RPC_URL")
	if ethRpcUrl == "" {
		slog.Error("ETH_RPC_URL is not set")
		return Config{}, fmt.Errorf("ETH_RPC_URL is not set")
	}

	searcherTasksToSpinUpVar := os.Getenv("SEARCHER_TASKS_TO_SPIN_UP")
	if searcherTasksToSpinUpVar == "" {
		slog.Error("SEARCHER_TASKS_TO_SPIN_UP is not set")
		return Config{}, fmt.Errorf("SEARCHER_TASKS_TO_SPIN_UP is not set")
	}
	searcherTasksToSpinUp, err := strconv.ParseUint(searcherTasksToSpinUpVar, 10, 64)
	if err != nil {
		slog.Error("SEARCHER_TASKS_TO_SPIN_UP is not a valid number", "err", err)
		return Config{}, err
	}

	rollupName := os.Getenv("ROLLUP_NAME")
	if rollupName == "" {
		slog.Error("ROLLUP_NAME is not set")
		return Config{}, fmt.Errorf("ROLLUP_NAME is not set")
	}

	latencyMarginVar := os.Getenv("LATENCY_MARGIN")
	if latencyMarginVar == "" {
		slog.Error("LATENCY_MARGIN is not set")
		return Config{}, fmt.Errorf("LATENCY_MARGIN is not set")
	}
	latencyMargin, err := strconv.ParseUint(latencyMarginVar, 10, 64)
	if err != nil {
		slog.Error("LATENCY_MARGIN is not a valid number", "err", err)
		return Config{}, err
	}

	return Config{
		sequencerUrl:          sequencerUrl,
		searcherPrivateKey:    searcherPrivateKey,
		ethRpcUrl:             ethRpcUrl,
		searcherTasksToSpinUp: searcherTasksToSpinUp,
		latencyMargin:         latencyMargin,
	}, nil
}

func (c *Config) PrintConfig() {
	slog.Info("Sequencer url is:", "sequencer_url", c.sequencerUrl)
	slog.Info("Searcher private key is:", "searcher_private_key", c.searcherPrivateKey)
	slog.Info("Eth rpc url is:", "eth_rpc_url", c.ethRpcUrl)
	slog.Info("Number of searcher tasks to spin up is:", "searcher_tasks_to_spin_up", c.searcherTasksToSpinUp)
	slog.Info("Rollup name is:", "rollup_name", c.rollupName)
	slog.Info("Latency margin is:", "latency_margin", c.latencyMargin)
}
