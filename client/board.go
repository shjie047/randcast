package client

import (
	"context"
	"log"
	"time"

	board "randcast/gen/go/board/v1"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type longlivedClient struct {
	client board.BoardServiceClient
	conn   *grpc.ClientConn
	id     uint32
}

func mkConnection(addr string) (*grpc.ClientConn, error) {
	return grpc.Dial(addr, []grpc.DialOption{grpc.WithInsecure(), grpc.WithBlock()}...)
}

// mkLonglivedClient creates a new client instance
func MkLonglivedClient(id uint32, addr string) (*longlivedClient, error) {
	conn, err := mkConnection(addr)
	if err != nil {
		return nil, err
	}
	return &longlivedClient{
		client: board.NewBoardServiceClient(conn),
		conn:   conn,
		id:     id,
	}, nil
}

// close is not used but is here as an example of how to close the gRPC client connection
func (c *longlivedClient) close() {
	if err := c.conn.Close(); err != nil {
		log.Fatal(err)
	}
}

func (c *longlivedClient) subscribe(bcast string) (board.BoardService_SubscribeClient, error) {
	log.Printf("Subscribing client ID: %d", c.id)
	return c.client.Subscribe(context.Background(), &board.SubscribeRequest{Id: c.id, Bcast: bcast})
}

// unsubscribe unsubscribes to messages from the gRPC server
func (c *longlivedClient) unsubscribe() error {
	log.Printf("Unsubscribing client ID %d", c.id)
	_, err := c.client.Unsubscribe(context.Background(), &board.UnsubscribeRequest{Id: c.id})
	return err
}

type StartCallback func(*board.SubscribeResponse)

func (c *longlivedClient) Start(bcast string, limit uint32, f StartCallback) {
	var err error
	// stream is the client side of the RPC stream
	var stream board.BoardService_SubscribeClient
	var counter uint32 = 0
	for {
		if stream == nil {
			if stream, err = c.subscribe(bcast); err != nil {
				log.Printf("Failed to subscribe: %v", err)
				c.sleep()
				// Retry on failure
				continue
			}
		}
		response, err := stream.Recv()
		if err != nil {
			log.Printf("Failed to receive message: %v", err)
			// Clearing the stream will force the client to resubscribe on next iteration
			stream = nil
			c.sleep()
			// Retry on failure
			continue
		}

		// log.Printf("Client ID %d got response: %q", c.id, response.Data)

		f(response)

		counter += 1

		if counter == limit {
			c.unsubscribe()
		}
	}
}

// sleep is used to give the server time to unsubscribe the client and reset the stream
func (c *longlivedClient) sleep() {
	time.Sleep(time.Second * 5)
}

type ConnectBoardCallback func(board.BoardServiceClient, context.Context)
type SendVertifiersCallback func()

func connectBoard(addr string, f ConnectBoardCallback) {
	// Set up a connection to the server.
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()
	c := board.NewBoardServiceClient(conn)

	// Contact the server and print out its response.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	f(c, ctx)
}

func SendVertifiers(addr string, vertifiers string, limit uint32) {
	connectBoard(addr, func(c board.BoardServiceClient, ctx context.Context) {

		// stream, err := c.SendVertifiers(ctx)
		// if err != nil {
		// 	log.Fatalf("client.SendVertifiers failed: %v", err)
		// }

		// waitc := make(chan struct{})

		// go func() {
		// 	for {
		// 		in, err := stream.Recv()
		// 		if err == io.EOF {
		// 			log.Println("Client Receive EOF, close stream.")
		// 			close(waitc)
		// 			return
		// 		}

		// 		log.Println("client Receive:", in.Vertifiers)

		// 	}
		// }()

		// stream.Send(&board.SendVertifiersRequest{
		// 	Vertifiers: vertifiers,
		// 	Limit:      limit,
		// })

		// stream.CloseSend()
		// <-waitc
	})
}
