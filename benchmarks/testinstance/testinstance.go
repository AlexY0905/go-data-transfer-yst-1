package testinstance

import (
	"context"
	"os"
	"time"

	"github.com/ipfs/go-datastore"
	ds "github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/delayed"
	"github.com/ipfs/go-datastore/namespace"
	ds_sync "github.com/ipfs/go-datastore/sync"
	badgerds "github.com/ipfs/go-ds-badger"
	graphsync "github.com/ipfs/go-graphsync"
	gsimpl "github.com/ipfs/go-graphsync/impl"
	"github.com/ipfs/go-graphsync/storeutil"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	delay "github.com/ipfs/go-ipfs-delay"
	"github.com/ipld/go-ipld-prime"
	peer "github.com/libp2p/go-libp2p-core/peer"

	"github.com/AlexY0905/go-go-storedcounter-yst-1/v2"

	datatransfer "github.com/AlexY0905/go-data-transfer-yst-1/v2"
	tn "github.com/AlexY0905/go-data-transfer-yst-1/v2/benchmarks/testnet"
	dtimpl "github.com/AlexY0905/go-data-transfer-yst-1/v2/impl"
	dtnet "github.com/AlexY0905/go-data-transfer-yst-1/v2/network"
	"github.com/AlexY0905/go-data-transfer-yst-1/v2/testutil"
	gstransport "github.com/AlexY0905/go-data-transfer-yst-1/v2/transport/graphsync"
)

// TempDirGenerator is any interface that can generate temporary directories
type TempDirGenerator interface {
	TempDir() string
}

// NewTestInstanceGenerator generates a new InstanceGenerator for the given
// testnet
func NewTestInstanceGenerator(ctx context.Context, net tn.Network, tempDirGenerator TempDirGenerator, diskBasedDatastore bool) InstanceGenerator {
	ctx, cancel := context.WithCancel(ctx)
	return InstanceGenerator{
		net:                net,
		seq:                0,
		ctx:                ctx, // TODO take ctx as param to Next, Instances
		cancel:             cancel,
		tempDirGenerator:   tempDirGenerator,
		diskBasedDatastore: diskBasedDatastore,
	}
}

// InstanceGenerator generates new test instances of bitswap+dependencies
type InstanceGenerator struct {
	seq                int
	net                tn.Network
	ctx                context.Context
	cancel             context.CancelFunc
	tempDirGenerator   TempDirGenerator
	diskBasedDatastore bool
}

// Close closes the clobal context, shutting down all test instances
func (g *InstanceGenerator) Close() error {
	g.cancel()
	return nil // for Closer interface
}

// Next generates a new instance of graphsync + dependencies
func (g *InstanceGenerator) Next() (Instance, error) {
	g.seq++
	return NewInstance(g.ctx, g.net, g.tempDirGenerator.TempDir(), g.diskBasedDatastore)
}

// Instances creates N test instances of bitswap + dependencies and connects
// them to each other
func (g *InstanceGenerator) Instances(n int) ([]Instance, error) {
	var instances []Instance
	for j := 0; j < n; j++ {
		inst, err := g.Next()
		if err != nil {
			return nil, err
		}
		instances = append(instances, inst)
	}
	ConnectInstances(instances)
	return instances, nil
}

// ConnectInstances connects the given instances to each other
func ConnectInstances(instances []Instance) {
	for i, inst := range instances {
		for j := i + 1; j < len(instances); j++ {
			oinst := instances[j]
			err := inst.Adapter.ConnectTo(context.Background(), oinst.Peer)
			if err != nil {
				panic(err.Error())
			}
		}
	}
}

// Close closes multiple instances at once
func Close(instances []Instance) error {
	for _, i := range instances {
		if err := i.Close(); err != nil {
			return err
		}
	}
	return nil
}

// Instance is a test instance of bitswap + dependencies for integration testing
type Instance struct {
	Peer            peer.ID
	Loader          ipld.Loader
	Storer          ipld.Storer
	BlockStore      blockstore.Blockstore
	Graphsync       graphsync.GraphExchange
	Manager         datatransfer.Manager
	Adapter         dtnet.DataTransferNetwork
	blockstoreDelay delay.D
	ds              datastore.Batching
}

// Close closes the associated datastore
func (i *Instance) Close() error {
	return i.ds.Close()
}

// Blockstore returns the block store for this test instance
func (i *Instance) Blockstore() blockstore.Blockstore {
	return i.BlockStore
}

// SetBlockstoreLatency customizes the artificial delay on receiving blocks
// from a blockstore test instance.
func (i *Instance) SetBlockstoreLatency(t time.Duration) time.Duration {
	return i.blockstoreDelay.Set(t)
}

// NewInstance creates a test bitswap instance.
//
// NB: It's easy make mistakes by providing the same peer ID to two different
// instances. To safeguard, use the InstanceGenerator to generate instances. It's
// just a much better idea.
func NewInstance(ctx context.Context, net tn.Network, tempDir string, diskBasedDatastore bool) (Instance, error) {
	bsdelay := delay.Fixed(0)

	p, gsNet, dtNet := net.Adapter()
	var dstore datastore.Batching
	var err error
	if diskBasedDatastore {
		defopts := badgerds.DefaultOptions
		defopts.SyncWrites = false
		defopts.Truncate = true
		dstore, err = badgerds.NewDatastore(tempDir, &defopts)
		if err != nil {
			return Instance{}, err
		}
	} else {
		dstore = ds_sync.MutexWrap(delayed.New(ds.NewMapDatastore(), bsdelay))
	}
	bstore, err := blockstore.CachedBlockstore(ctx,
		blockstore.NewBlockstore(namespace.Wrap(dstore, datastore.NewKey("blockstore"))),
		blockstore.DefaultCacheOpts())
	if err != nil {
		return Instance{}, err
	}

	loader := storeutil.LoaderForBlockstore(bstore)
	storer := storeutil.StorerForBlockstore(bstore)
	gs := gsimpl.New(ctx, gsNet, loader, storer, gsimpl.RejectAllRequestsByDefault())
	transport := gstransport.NewTransport(p, gs)
	dtCounter := storedcounter.New(dstore, datastore.NewKey("/data-transfers/counter"))
	dt, err := dtimpl.NewDataTransfer(namespace.Wrap(dstore, datastore.NewKey("/data-transfers/transfers")), os.TempDir(), dtNet, transport, dtCounter)
	if err != nil {
		return Instance{}, err
	}
	ready := make(chan error, 1)
	dt.OnReady(func(err error) {
		ready <- err
	})
	err = dt.Start(ctx)
	if err != nil {
		return Instance{}, err
	}
	select {
	case <-ctx.Done():
		return Instance{}, ctx.Err()
	case err := <-ready:
		if err != nil {
			return Instance{}, err
		}
	}
	sv := testutil.NewStubbedValidator()
	sv.StubSuccessPull()
	sv.StubSuccessPush()
	dt.RegisterVoucherType(testutil.NewFakeDTType(), sv)
	dt.RegisterVoucherResultType(testutil.NewFakeDTType())
	return Instance{
		Adapter:         dtNet,
		Peer:            p,
		Graphsync:       gs,
		Manager:         dt,
		Loader:          loader,
		Storer:          storer,
		BlockStore:      bstore,
		blockstoreDelay: bsdelay,
		ds:              dstore,
	}, nil
}
