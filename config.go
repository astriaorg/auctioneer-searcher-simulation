package main

import (
	"fmt"
	"github.com/joho/godotenv"
	"log/slog"
	"os"
	"strconv"
	"time"
)

type Config struct {
	sequencerUrl               string
	searcherPrivateKey         string
	ethRpcUrl                  string
	searcherTasksToSpinUp      uint64
	rollupName                 string
	latencyMargin              time.Duration
	searchingTimeStart         time.Duration
	searchingTimeIncreaseDelta time.Duration
	searchingTimeEnd           time.Duration
	resultFile                 string
}

func parseAndValidateSearchingTimes() (time.Duration, time.Duration, time.Duration, error) {
	searchingTimeStartVar := os.Getenv("SEARCHING_TIME_START")
	if searchingTimeStartVar == "" {
		slog.Error("SEARCHING_TIME_START is not set")
		return 0, 0, 0, fmt.Errorf("SEARCHING_TIME_START is not set")
	}
	searchingTimeStart, err := time.ParseDuration(searchingTimeStartVar)
	if err != nil {
		slog.Error("SEARCHING_TIME_START is not a valid duration", "err", err)
		return 0, 0, 0, err
	}

	searchingTimeIncreaseDeltaVar := os.Getenv("SEARCHING_TIME_INCREASE_DELTA")
	if searchingTimeIncreaseDeltaVar == "" {
		slog.Error("SEARCHING_TIME_INCREASE_DELTA is not set")
		return 0, 0, 0, fmt.Errorf("SEARCHING_TIME_INCREASE_DELTA is not set")
	}
	searchingTimeIncreaseDelta, err := time.ParseDuration(searchingTimeIncreaseDeltaVar)
	if err != nil {
		slog.Error("SEARCHING_TIME_INCREASE_DELTA is not a valid duration", "err", err)
		return 0, 0, 0, err
	}

	searchingTimeEndVar := os.Getenv("SEARCHING_TIME_END")
	if searchingTimeEndVar == "" {
		slog.Error("SEARCHING_TIME_END is not set")
		return 0, 0, 0, fmt.Errorf("SEARCHING_TIME_END is not set")
	}
	searchingTimeEnd, err := time.ParseDuration(searchingTimeEndVar)
	if err != nil {
		slog.Error("SEARCHING_TIME_END is not a valid duration", "err", err)
		return 0, 0, 0, err
	}

	if searchingTimeEnd < searchingTimeStart {
		slog.Error("SEARCHING_TIME_END is less than SEARCHING_TIME_START")
		return 0, 0, 0, fmt.Errorf("SEARCHING_TIME_END is less than SEARCHING_TIME_START")
	}

	return searchingTimeStart, searchingTimeIncreaseDelta, searchingTimeEnd, nil
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
	latencyMargin, err := time.ParseDuration(latencyMarginVar)
	if err != nil {
		slog.Error("LATENCY_MARGIN is not a valid duration", "err", err)
		return Config{}, err
	}

	searchingTimeStart, searchingTimeIncreaseDelta, searchingTimeEnd, err := parseAndValidateSearchingTimes()
	if err != nil {
		slog.Error("can not parse and validate searching times", "err", err)
		return Config{}, err
	}

	resultFile := os.Getenv("RESULT_FILE")
	if resultFile == "" {
		slog.Error("RESULT_FILE is not set")
		return Config{}, fmt.Errorf("RESULT_FILE is not set")
	}
	if FileExists(resultFile) {
		slog.Error("RESULT_FILE already exists")
		return Config{}, fmt.Errorf("RESULT_FILE already exists")
	}

	return Config{
		sequencerUrl:               sequencerUrl,
		searcherPrivateKey:         searcherPrivateKey,
		ethRpcUrl:                  ethRpcUrl,
		searcherTasksToSpinUp:      searcherTasksToSpinUp,
		latencyMargin:              latencyMargin,
		searchingTimeStart:         searchingTimeStart,
		searchingTimeIncreaseDelta: searchingTimeIncreaseDelta,
		searchingTimeEnd:           searchingTimeEnd,
		resultFile:                 resultFile,
	}, nil
}

func (c *Config) PrintConfig() {
	slog.Info("Sequencer url is:", "sequencer_url", c.sequencerUrl)
	slog.Info("Searcher private key is:", "searcher_private_key", c.searcherPrivateKey)
	slog.Info("Eth rpc url is:", "eth_rpc_url", c.ethRpcUrl)
	slog.Info("Number of searcher tasks to spin up is:", "searcher_tasks_to_spin_up", c.searcherTasksToSpinUp)
	slog.Info("Rollup name is:", "rollup_name", c.rollupName)
	slog.Info("Latency margin is:", "latency_margin", c.latencyMargin)
	slog.Info("Searching time start is:", "searching_time_start", c.searchingTimeStart)
	slog.Info("Searching time increase delta is:", "searching_time_increase_delta", c.searchingTimeIncreaseDelta)
	slog.Info("Searching time end is:", "searching_time_end", c.searchingTimeEnd)
	slog.Info("Result file is:", "result_file", c.resultFile)
}
