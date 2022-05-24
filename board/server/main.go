package main

import (
	"context"
	"fmt"
	"log"
	"net"
	board "randcast/gen/go/board/v1"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
)

func main() {
	lis, err := net.Listen("tcp", ":9090")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()
	ss := &server{}

	ss.sent = make(map[string]bool)
	ss.data = make(map[uint32]string)
	go ss.watchSubscribers()
	board.RegisterBoardServiceServer(s, ss)

	log.Printf("server listening at %v", lis.Addr())

	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

type server struct {
	subscribers sync.Map
	data        map[uint32]string
	board.UnimplementedBoardServiceServer
	sent map[string]bool
}

type sub struct {
	stream   board.BoardService_SubscribeServer
	finished chan<- bool // finished is used to signal closure of a client subscribing goroutine
}

func (s *server) watchSubscribers() {
	for {
		time.Sleep(time.Second)

		// A list of clients to unsubscribe in case of error
		var unsubscribe []uint32

		s.subscribers.Range(func(k, v interface{}) bool {
			id, ok := k.(uint32)
			if !ok {
				log.Printf("Failed to cast subscriber key: %T", k)
				return false
			}

			sub, ok := v.(sub)

			if !ok {
				log.Printf("Failed to cast subscriber value: %T", v)
				return false
			}

			for i, j := range s.data {
				if s.sent[fmt.Sprintf("%d%d", id, i)] {
					continue
				}

				s.sent[fmt.Sprintf("%d%d", id, i)] = true

				if err := sub.stream.Send(&board.SubscribeResponse{
					Id:    i,
					Bcast: j,
				}); err != nil {
					log.Printf("Failed to send data to client: %v", err)
					select {
					case sub.finished <- true:
						log.Printf("Unsubscribed client: %d", id)
					default:
						// Default case is to avoid blocking in case client has already unsubscribed
					}
					// In case of error the client would re-subscribe so close the subscriber stream
					unsubscribe = append(unsubscribe, id)
					break
				}
			}

			return true
		})

		for _, id := range unsubscribe {
			s.subscribers.Delete(id)
		}
	}
}

// Subscribe handles a subscribe request from a client
func (s *server) Subscribe(request *board.SubscribeRequest, stream board.BoardService_SubscribeServer) error {
	log.Printf("Received subscribe request from ID: %d", request.Id)
	fin := make(chan bool)
	s.subscribers.Store(request.Id, sub{stream: stream, finished: fin})
	s.data[request.Id] = request.Bcast

	ctx := stream.Context()
	for {
		select {
		case <-fin:
			log.Printf("Closing stream for client ID: %d", request.Id)
		case <-ctx.Done():
			log.Printf("Client ID %d has disconnected", request.Id)
			return nil
		}
	}
}

func (s *server) Unsubscribe(ctx context.Context, request *board.UnsubscribeRequest) (*board.UnsubscribeResponse, error) {
	v, ok := s.subscribers.Load(request.Id)
	if !ok {
		return nil, fmt.Errorf("failed to load subscriber key: %d", request.Id)
	}
	sub, ok := v.(sub)
	if !ok {
		return nil, fmt.Errorf("failed to cast subscriber value: %T", v)
	}
	select {
	case sub.finished <- true:
		log.Printf("Unsubscribed client: %d", request.Id)
	default:
		// Default case is to avoid blocking in case client has already unsubscribed
	}
	s.subscribers.Delete(request.Id)
	delete(s.data, request.Id)

	for k, _ := range s.sent {
		if strings.HasPrefix(k, fmt.Sprintf("%d", request.Id)) {
			delete(s.sent, k)
		}
	}
	return &board.UnsubscribeResponse{}, nil
}
