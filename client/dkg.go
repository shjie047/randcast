package client

import (
	"context"
	"log"
	"time"

	dkg "randcast/gen/go/dkg/v1"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type ConnectDkgCallback func(dkg.DkgServiceClient, context.Context)

func connectDkg(addr string, f ConnectDkgCallback) {
	// Set up a connection to the server.
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()
	c := dkg.NewDkgServiceClient(conn)

	// Contact the server and print out its response.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	f(c, ctx)
}

func SendShares(addr string, shares string) {
	connectDkg(addr, func(c dkg.DkgServiceClient, ctx context.Context) {
		log.Println("send share to:", addr)
		_, err := c.SendShares(ctx, &dkg.SendSharesRequest{
			Shares: shares,
		})

		if err != nil {
			log.Fatalf("could not send share: %v", err)
		}
	})
}
