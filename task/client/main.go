package main

import (
	"context"
	"log"
	"randcast/gen/go/task/v1"
	"time"

	"google.golang.org/grpc"
)

// sleep is used to give the server time to unsubscribe the client and reset the stream
func (c *longlivedClient) sleep() {
	time.Sleep(time.Second * 5)
}

func (c *longlivedClient) subscribe() (task.TaskService_SubscribeClient, error) {
	log.Printf("Subscribing Task client ID: %d", c.id)
	return c.client.Subscribe(context.Background(), &task.SubscribeRequest{Id: c.id})
}

// unsubscribe unsubscribes to messages from the gRPC server
func (c *longlivedClient) unsubscribe() error {
	log.Printf("Unsubscribing Task client ID %d", c.id)
	_, err := c.client.Unsubscribe(context.Background(), &task.UnsubscribeRequest{Id: c.id})
	return err
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
		client: task.NewTaskServiceClient(conn),
		conn:   conn,
		id:     id,
	}, nil
}

func (c *longlivedClient) Start() {
	var err error
	// stream is the client side of the RPC stream
	var stream task.TaskService_SubscribeClient
	for {
		if stream == nil {
			if stream, err = c.subscribe(); err != nil {
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

		log.Println("Task Is = ", response)
		// c.unsubscribe()
	}

}

func main() {

	go func() {
		client, _ := MkLonglivedClient(2, ":8081")
		client.Start()
	}()

	client, _ := MkLonglivedClient(1, ":8081")
	client.Start()
}

type longlivedClient struct {
	client task.TaskServiceClient
	conn   *grpc.ClientConn
	id     uint32
}
