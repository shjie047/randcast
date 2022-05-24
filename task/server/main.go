package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	task "randcast/gen/go/task/v1"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Task struct {
	Id        string
	State     int
	Msg       string
	Limit     uint32
	Threshold uint32
	Cur       map[string]bool
}

func newTask() *Task {
	t := &Task{
		Id:        uuid.New().String(),
		Msg:       uuid.New().String(),
		State:     1,
		Limit:     4,
		Threshold: 3,
		Cur:       make(map[string]bool, 4),
	}

	return t
}

func RPC(port uint32) {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatalln("Failed to listen:", err)
	}

	s := grpc.NewServer()
	ss := &server{}
	task.RegisterTaskServiceServer(s, ss)

	go ss.watchSubscribers()

	go func() {
		log.Fatalln(s.Serve(lis))
	}()

	fmt.Println()
	fmt.Printf("  GRPC Serving: %d => %d\n", port, port+10)

	Gateway(fmt.Sprintf("0.0.0.0:%d", port), fmt.Sprintf("%d", port+10))
}

func main() {
	RPC(8081)
}

func Gateway(dialAddr string, port string) error {
	conn, err := grpc.DialContext(
		context.Background(),
		dialAddr,
		grpc.WithBlock(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)

	if err != nil {
		return fmt.Errorf("failed to dial server: %w", err)
	}

	gwmux := runtime.NewServeMux()
	err = task.RegisterTaskServiceHandler(context.Background(), gwmux, conn)

	if err != nil {
		return fmt.Errorf("failed to register gateway: %w", err)
	}

	gatewayAddr := "0.0.0.0:" + port

	gwServer := &http.Server{
		Addr:    gatewayAddr,
		Handler: gwmux,
	}

	return fmt.Errorf("serving Task gRPC-Gateway server: %w", gwServer.ListenAndServe())
}

type server struct {
	tasks       []*Task
	subscribers sync.Map
	// data        map[uint32]string
	task.UnimplementedTaskServiceServer
	// sent map[string]bool
}

type sub struct {
	stream   task.TaskService_SubscribeServer
	finished chan<- bool // finished is used to signal closure of a client subscribing goroutine
}

func (s *server) watchSubscribers() {
	for {
		time.Sleep(time.Second)

		// A list of clients to unsubscribe in case of error
		var unsubscribe []uint32

		s.subscribers.Range(func(k, v interface{}) bool {
			id, ok := k.(uint32)
			// log.Printf("Subscriber is %d", id)

			if !ok {
				log.Printf("Failed to cast subscriber key: %T", k)
				return false
			}

			sub, ok := v.(sub)

			if !ok {
				log.Printf("Failed to cast subscriber value: %T", v)
				return false
			}

			if s.tasks != nil && len(s.tasks) > 0 {
				log.Printf("Exist %d task to send....", len(s.tasks))
				var tasks_new []*Task
				for _, t := range s.tasks {
					if len(t.Cur) == int(t.Limit) {
						continue
					} else {
						tasks_new = append(tasks_new, t)
					}

					if t.Cur[fmt.Sprintf("%d-%s", id, t.Id)] {
						continue
					}

					if err := sub.stream.Send(&task.SubscribeResponse{
						TaskId:    t.Id,
						Msg:       t.Msg,
						Threshold: t.Threshold,
						Limit:     t.Limit,
					}); err != nil {
						log.Printf("Failed to send data to Task client: %v", err)

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

					t.Cur[fmt.Sprintf("%d-%s", id, t.Id)] = true
				}
				s.tasks = tasks_new
			}

			return true
		})

		for _, id := range unsubscribe {
			s.subscribers.Delete(id)
		}
	}
}

func (s *server) NewTask(ctx context.Context, req *task.NewTaskRequest) (*task.NewTaskResponse, error) {
	t := newTask()
	s.tasks = append(s.tasks, t)
	timeoutchan := make(chan bool)
	var signature string
	go func() {
		time.AfterFunc(2*time.Second, func() {
			signature = uuid.New().String()
			timeoutchan <- true
		})
	}()

	select {
	case <-timeoutchan:
		break
	case <-time.After(10 * time.Second):
		return &task.NewTaskResponse{Data: "timeout"}, nil
	}

	return &task.NewTaskResponse{Data: signature}, nil
}

// Subscribe handles a subscribe request from a client
func (s *server) Subscribe(request *task.SubscribeRequest, stream task.TaskService_SubscribeServer) error {
	log.Printf("Received subscribe request from ID: %d", request.Id)
	fin := make(chan bool)
	s.subscribers.Store(request.Id, sub{stream: stream, finished: fin})

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

func (s *server) Unsubscribe(ctx context.Context, request *task.UnsubscribeRequest) (*task.UnsubscribeResponse, error) {
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
	// delete(s.data, request.Id)

	// for k, _ := range s.sent {
	// 	if strings.HasPrefix(k, fmt.Sprintf("%d", request.Id)) {
	// 		delete(s.sent, k)
	// 	}
	// }
	return &task.UnsubscribeResponse{}, nil
}
