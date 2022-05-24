package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"randcast/gen/go/board/v1"
	dkg "randcast/gen/go/dkg/v1"
	"sync"

	client "randcast/client"

	"randcast/serialize"

	"github.com/coinbase/kryptology/pkg/core/curves"
	frost "github.com/coinbase/kryptology/pkg/dkg/frost"
	"github.com/coinbase/kryptology/pkg/sharing"
	"google.golang.org/grpc"
)

var (
	dkg_port *int
)

type server struct {
	shares map[uint32]*sharing.ShamirShare
	bcast  map[uint32]*frost.Round1Bcast
	dp     *frost.DkgParticipant
	dkg.UnimplementedDkgServiceServer
	limit uint32
	round int
	sync.RWMutex
}

type Round1Func func(secret []byte) (*frost.Round1Bcast, frost.Round1P2PSend, error)

func (s *server) StartDkg(ctx context.Context, in *dkg.StartDkgRequest) (*dkg.StartDkgResponse, error) {
	log.Println("start DKG:", in.Id)
	s.limit = uint32(len(in.Others) + 1)
	s.shares = make(map[uint32]*sharing.ShamirShare, len(in.Others))
	s.bcast = make(map[uint32]*frost.Round1Bcast, s.limit)
	s.round = 1

	bcast, p2pSend, dp, err := Round1(in.Id, in.Threshold, in.Msg, curves.BLS12381G1(), in.Others...)

	s.dp = dp

	if err != nil {
		return &dkg.StartDkgResponse{State: dkg.State_FAILED}, err
	}

	for id, share := range p2pSend {
		share_bytes, _ := json.Marshal(share)
		client.SendShares(fmt.Sprintf(":808%d", id), string(share_bytes))
	}

	b, _ := serialize.BcastToBinary(bcast)

	cli, err := client.MkLonglivedClient(in.Id, "0.0.0.0:9090")

	if err != nil {
		log.Fatal(err)
	}

	go cli.Start(string(b), uint32(len(in.Others)+1), func(res *board.SubscribeResponse) {
		// Receive all Round-1 bcast
		s.Lock()
		s.bcast[res.Id] = serialize.BinaryToBcast([]byte(res.Bcast))
		s.Unlock()
		if s.ifCanRound2() {
			s.round2()
		}
	})

	return &dkg.StartDkgResponse{State: dkg.State_OK}, nil
}

// Receive other participants shares
func (s *server) SendShares(ctx context.Context, in *dkg.SendSharesRequest) (*dkg.SendSharesResponse, error) {
	var share *sharing.ShamirShare

	json.Unmarshal([]byte(in.Shares), &share)
	s.shares[share.Id] = share

	if s.ifCanRound2() {
		s.round2()
	}

	return &dkg.SendSharesResponse{State: dkg.State_OK}, nil
}

func (s *server) ifCanRound2() bool {
	ret := len(s.bcast) == int(s.limit) && len(s.shares) == int(s.limit-1) && s.round == 1

	if ret {
		for id := range s.bcast {
			log.Printf("bc from %d", id)
		}

		for id := range s.shares {
			log.Printf("shares from %d", id)
		}
	}

	return ret
}

func (s *server) round2() {
	s.round = 2
	log.Println("Start Round2")
	// rn2Out, err := s.dp.Round2(s.bcast, s.shares)
	// if err != nil {
	// 	log.Fatalf("Round2 failed %v", err)
	// }

	// log.Println(rn2Out)
}

func TryRound1(max uint32, id uint32, threshold uint32, ctx string, curve *curves.Curve, otherParticipants ...uint32) (*frost.Round1Bcast, frost.Round1P2PSend, *frost.DkgParticipant, error) {
	var c uint32 = 0
	var (
		a   *frost.Round1Bcast
		b   frost.Round1P2PSend
		dp  *frost.DkgParticipant
		err error
	)
	var next func()

	next = func() {
		// check times
		if c < max {
			// re-create Participant
			dp, err = frost.NewDkgParticipant(id, threshold, ctx, curve, otherParticipants...)

			// 参数错误，可能是手动构造请求的
			// 恶意攻击
			if err != nil {
				log.Fatal(err)
			}

			a, b, err = dp.Round1(nil)
			c += 1
			// log.Printf("try Round-1 %d times\n", c)
			if err != nil {
				next()
			} else {
				// log.Printf("Round-1 %d times: OK\n", c)
			}
		} else {
			log.Printf("Round-1 %d times: FAILED\n", c)
		}
	}

	next()

	return a, b, dp, err
}

func Round1(id uint32, threshold uint32, ctx string, curve *curves.Curve, otherParticipants ...uint32) (*frost.Round1Bcast, frost.Round1P2PSend, *frost.DkgParticipant, error) {
	bcast, p2pSend, dp, err := TryRound1(2, id, threshold, ctx, curve, otherParticipants...)

	return bcast, p2pSend, dp, err
}

func main() {
	dkg_port = flag.Int("dkg", -1, "dkg port number")
	flag.Parse()

	if *dkg_port == -1 {
		log.Fatalf("Please support DKG port")
	}

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *dkg_port))

	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer()
	ss := &server{}

	dkg.RegisterDkgServiceServer(s, ss)

	log.Printf("server listening at %v", lis.Addr())
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
