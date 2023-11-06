package main

import (
	"context"
	"fmt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	pb "k8s-combat/api/google.golang.org/grpc/examples/helloworld/helloworld"
	"log"
	"net"
	"net/http"
	"os"
	"sync"
	"time"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		name, _ := os.Hostname()
		url := os.Getenv("PG_URL")
		pwd := os.Getenv("PG_PWD")
		fmt.Fprint(w, fmt.Sprintf("%s-%s-%s", name, url, pwd))
	})
	http.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		name, _ := os.Hostname()
		log.Printf("%s ping", name)
		fmt.Fprint(w, "pong")
	})
	http.HandleFunc("/service", func(w http.ResponseWriter, r *http.Request) {
		resp, err := http.Get("http://k8s-combat-service:8081/ping")
		if err != nil {
			log.Println(err)
			fmt.Fprint(w, err)
			return
		}
		fmt.Fprint(w, resp.Status)
	})
	var (
		once sync.Once
		c    pb.GreeterClient
	)
	http.HandleFunc("/grpc_client", func(w http.ResponseWriter, r *http.Request) {
		once.Do(func() {
			service := r.URL.Query().Get("name")
			conn, err := grpc.Dial(fmt.Sprintf("%s:50051", service), grpc.WithInsecure(), grpc.WithBlock())
			if err != nil {
				log.Fatalf("did not connect: %v", err)
			}
			c = pb.NewGreeterClient(conn)
		})
		version := r.URL.Query().Get("version")

		// Contact the server and print out its response.
		name := "world"
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		md := metadata.New(map[string]string{
			"version": version,
		})
		ctx = metadata.NewOutgoingContext(ctx, md)
		defer cancel()
		g, err := c.SayHello(ctx, &pb.HelloRequest{Name: name})
		if err != nil {
			log.Fatalf("could not greet: %v", err)
		}
		fmt.Fprint(w, fmt.Sprintf("Greeting: %s", g.GetMessage()))
	})
	go func() {
		var port = ":50051"
		lis, err := net.Listen("tcp", port)
		if err != nil {
			log.Fatalf("failed to listen: %v", err)
		}
		s := grpc.NewServer()
		pb.RegisterGreeterServer(s, &server{})
		if err := s.Serve(lis); err != nil {
			log.Fatalf("failed to serve: %v", err)
		} else {
			log.Printf("served on %s \n", port)
		}
	}()
	http.ListenAndServe(":8081", nil)
}

// server is used to implement helloworld.GreeterServer.
type server struct {
	pb.UnimplementedGreeterServer
}

// SayHello implements helloworld.GreeterServer
func (s *server) SayHello(ctx context.Context, in *pb.HelloRequest) (*pb.HelloReply, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	var version string
	if ok {
		version = md.Get("version")[0]
	}
	log.Printf("Received: %v, version: %s", in.GetName(), version)
	name, _ := os.Hostname()
	return &pb.HelloReply{Message: fmt.Sprintf("hostname:%s, in:%s, version:%s", name, in.Name, version)}, nil
}
