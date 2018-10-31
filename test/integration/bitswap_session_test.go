package integrationtest

import (
	"bytes"
	"context"
	"github.com/ipfs/go-ipfs/core/coreapi"
	"github.com/ipfs/go-ipfs/core/coreapi/interface"
	"github.com/ipfs/go-ipfs/core/coreunix"
	"github.com/ipfs/go-ipfs/thirdparty/unit"
	"gx/ipfs/QmNkxFCmPtr2RQxjZNRCNryLud4L9wMEiBJsLgF14MqTHj/go-bitswap"
	"io"
	"math"
	"testing"
	"time"

	"github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/core/mock"
	"gx/ipfs/QmUDTcnDp2WssbmiDLC6aYurUeyt7QeRakHUQMxA2mZ5iB/go-libp2p/p2p/net/mock"
)

func TestBitswapSessions(b *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	const numPeers = 6

	// create network

	mn := mocknet.New(ctx)
	mn.SetLinkDefaults(mocknet.LinkOptions{
		Latency: 250 * time.Microsecond,
		// TODO add to conf. This is tricky because we want 0 values to be functional.
		Bandwidth: math.MaxInt32,
	})

	var nodes []*core.IpfsNode
	for i := 0; i < numPeers; i++ {
		n, err := core.NewNode(ctx, &core.BuildCfg{
			Online:  true,
			Host:    coremock.MockHostOption(mn),
			Routing: core.NilRouterOption, // no routing
		})
		if err != nil {
			b.Fatal(err)
		}
		defer n.Close()
		nodes = append(nodes, n)
	}

	mn.LinkAll()

	// connect them
	for _, n1 := range nodes {
		for _, n2 := range nodes {
			if n1 == n2 {
				continue
			}

			p2 := n2.PeerHost.Peerstore().PeerInfo(n2.PeerHost.ID())
			if err := n1.PeerHost.Connect(ctx, p2); err != nil {
				b.Fatal(err)
			}
		}
	}

	randomBytes := RandomBytes(100*unit.MB)
	added, err := coreunix.Add(nodes[0], bytes.NewReader(randomBytes))
	if err != nil {
		b.Fatal(err)
	}

	ap, err := iface.ParsePath(added)
	if err != nil {
		b.Fatal(err)
	}

	//  get it out.
	for i, n := range nodes {
		// skip first because block not in its exchange. will hang.
		if i == 0 {
			continue
		}

		nApi := coreapi.NewCoreAPI(n)

		got, err := nApi.Unixfs().Get(ctx, ap)
		if err != nil {
			b.Error(err)
		}

		bufout := new(bytes.Buffer)
		io.Copy(bufout, got)
		if 0 != bytes.Compare(bufout.Bytes(), randomBytes) {
			b.Fatal("catted data does not match added data")
		}

		for _, bPeer := range nodes {
			bstat, err := bPeer.Exchange.(*bitswap.Bitswap).Stat()
			if err != nil {
				b.Fatal(err)
			}
			b.Logf("%s blocks sent: %d", bPeer.Identity, bstat.BlocksSent)
		}

		bstat, err := n.Exchange.(*bitswap.Bitswap).Stat()
		if err != nil {
			b.Fatal(err)
		}

		b.Logf("%d %s got data.", i, n.Identity)
		b.Logf("%s duplicate blocks received: %d", n.Identity, bstat.DupBlksReceived)
	}
	cancel()
	return
}
