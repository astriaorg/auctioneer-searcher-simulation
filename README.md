# Auctioneer Searcher Simulation

This is a simulation of how a searcher can utilize optimistic blocks to time their bundle building processes to submit the bundle to the auctioneer
in time to be included in the next subsequent block.

## How it works

The binary connects to a sequencer optimistic block stream and block commitment stream. Upon receiving an optimistic block, we start the searching process
needed to find the bundle that we want to submit to the auctioneer. We model the searcher process as a simple sleep function that sleeps for a fixed amount of time. 
After the sleep, we submit a bundle to the auctioneer via the auctioneer geth node. Note that, the searcher also listens to a block commitment stream to know when the optimistic block is committed.
Through this, they can understand that the auction is over and that they have to send a bundle quickly in order to be included in the next block.

## Running it

To run it, fill up the values appropriately in the `.env.local` file.

After that, build the binary:

```bash
go build -o auctioneer-searcher-simulation
```

Then run the binary:

```bash
./auctioneer-searcher-simulation
```