# Auctioneer Searcher Simulation

This is a simulation of how a searcher can utilize optimistic blocks to time their bundle building processes to submit the bundle to the auctioneer
in time to be included in the next subsequent block.

## How it works

The binary connects to a sequencer optimistic block stream and block commitment stream. Upon receiving an optimistic block, we start the searching process
needed to find the bundle that we want to submit to the auctioneer. We model the searcher process as a simple sleep function that sleeps for a fixed amount of time. 
After the sleep, we submit a bundle to the auctioneer via the auctioneer geth node. Note that, the searcher also listens to a block commitment stream to know when the optimistic block is committed.
Through this, they can understand that the auction is over and that they have to send a bundle quickly in order to be included in the next block.
Note that, we spin up a searcher task every 4 optimistic blocks. This is because

## Running it

To run it, fill up the values appropriately in the `.env` file. We categorize the `.env` files based on the environment. 
For e.g, for testnet we keep the configurations in the `.env.testnet` file and for mainnet we keep configurations in the `.env.mainnet` file.
A `.env` file looks like the following for testnet:

```bash
SEQUENCER_URL=grpc-auctioneer.sequencer.dawn-1.astria.org
SEARCHER_PRIVATE_KEY="a private key with funds"
ETH_RPC_URL=https://rpc.flame.dawn-1.astria.org
SEARCHER_TASKS_TO_SPIN_UP=1 # the number of times we want to run the searcher
ROLLUP_NAME=astria
# LATENCY_MARGIN is the time a searcher waits after receiving a block commitment. This can be ignored for now as we are not considering block commitment in our tests
LATENCY_MARGIN=100ms 
# the below variables are used to determine the time to spend searching in the searcher tasks
# SEARCHING_TIME_START is the time to spend searching in the first searcher task
SEARCHING_TIME_START=10ms
# SEARCHING_TIME_INCREASE_DELTA is the amount of time to increase the time to spend searching in each subsequent searching task
SEARCHING_TIME_INCREASE_DELTA=0ms
# SEARCHING_TIME_END is the searching time at which the last task will spend searching
SEARCHING_TIME_END=10ms
# RESULT_FILE is the file in which the results of all the searchers will be written to. The results include the time taken to submit the bundle to the auctioneer, the tx hash is the tx has been submitted etc
RESULT_FILE=test_file.csv
```

Below is an example configuration to run 50 searcher tasks which search for 1.2s:
```bash
SEQUENCER_URL=grpc-auctioneer.sequencer.dawn-1.astria.org
SEARCHER_PRIVATE_KEY="a private key with funds"
ETH_RPC_URL=https://rpc.flame.dawn-1.astria.org
# SEARCHER_TASKS_TO_SPIN_UP is the number of times we want to run the searcher
SEARCHER_TASKS_TO_SPIN_UP=50 
ROLLUP_NAME=astria
# LATENCY_MARGIN is the time a searcher waits after receiving a block commitment. This can be ignored for now as we are not considering block commitment in our tests
LATENCY_MARGIN=100ms 
# the below variables are used to determine the time to spend searching in the searcher tasks
# SEARCHING_TIME_START is the time to spend searching in the first searcher task
SEARCHING_TIME_START=1.2s
# SEARCHING_TIME_INCREASE_DELTA is the amount of time to increase the time to spend searching in each subsequent searching task
SEARCHING_TIME_INCREASE_DELTA=0ms
# SEARCHING_TIME_END is the searching time at which the last task will spend searching
SEARCHING_TIME_END=1.2s
# RESULT_FILE is the file in which the results of all the searchers will be written to. The results include the time taken to submit the bundle to the auctioneer, the tx hash is the tx has been submitted etc
RESULT_FILE=testing.csv
```

After that, build the binary:

```bash
go build -o auctioneer-searcher-simulation
```

Then run the binary, specifying the environment as an environment variable `ENV`. For e.g, to run the simulation for testnet, run the following command:
```bash
ENV=testnet ./auctioneer-searcher-simulation
```