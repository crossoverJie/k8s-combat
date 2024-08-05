package main

import (
	"context"
	"fmt"
	"github.com/rs/zerolog/log"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	pb "k8s-combat/api/google.golang.org/grpc/examples/helloworld/helloworld"

	otelhooks "github.com/open-feature/go-sdk-contrib/hooks/open-telemetry/pkg"
	flagd "github.com/open-feature/go-sdk-contrib/providers/flagd/pkg"
	"github.com/open-feature/go-sdk/openfeature"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
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
		log.Info().Msgf("%s ping", name)
		fmt.Sprintf("%s ping====", name)
		fmt.Fprint(w, "pong")
	})
	http.HandleFunc("/service", func(w http.ResponseWriter, r *http.Request) {
		resp, err := http.Get("http://k8s-combat-service:8081/ping")
		if err != nil {
			log.Err(err).Msg("get http://k8s-combat-service:8081/ping error")
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
				log.Fatal().Msgf("did not connect: %v", err)
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
			log.Fatal().Msgf("could not greet: %v", err)
		}
		fmt.Fprint(w, fmt.Sprintf("Greeting: %s", g.GetMessage()))
	})

	// Init OpenTelemetry start
	tp := initTracerProvider()
	defer func() {
		if err := tp.Shutdown(context.Background()); err != nil {
			log.Printf("Error shutting down tracer provider: %v", err)
		}
	}()

	mp := initMeterProvider()
	defer func() {
		if err := mp.Shutdown(context.Background()); err != nil {
			log.Printf("Error shutting down meter provider: %v", err)
		}
	}()

	err := runtime.Start(runtime.WithMinimumReadMemStatsInterval(time.Second))
	if err != nil {
		log.Err(err)
	}

	var meter = otel.Meter("test.io/k8s/combat")
	apiCounter, err = meter.Int64Counter(
		"api.counter",
		metric.WithDescription("Number of API calls."),
		metric.WithUnit("{call}"),
	)
	if err != nil {
		log.Err(err)
	}

	openfeature.SetProvider(flagd.NewProvider())
	openfeature.AddHooks(otelhooks.NewTracesHook())

	tracer = tp.Tracer("k8s-combat")
	// Init OpenTelemetry end

	go func() {
		var port = ":50051"
		lis, err := net.Listen("tcp", port)
		if err != nil {
			log.Fatal().Msgf("failed to listen: %v", err)
		}
		s := grpc.NewServer(
			grpc.StatsHandler(otelgrpc.NewServerHandler()),
		)
		pb.RegisterGreeterServer(s, &server{})
		if err := s.Serve(lis); err != nil {
			log.Fatal().Msgf("failed to serve: %v", err)
		} else {
			log.Printf("served on %s \n", port)
		}
	}()
	quit := make(chan os.Signal)
	signal.Notify(quit, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGPIPE)
	go func() {
		<-quit
		log.Printf("quit signal received, exit \n")
		os.Exit(0)
	}()
	http.ListenAndServe(":8081", nil)
}

// server is used to implement helloworld.GreeterServer.
type server struct {
	pb.UnimplementedGreeterServer
}

// SayHello implements helloworld.GreeterServer
func (s *server) SayHello(ctx context.Context, in *pb.HelloRequest) (*pb.HelloReply, error) {
	defer apiCounter.Add(ctx, 1)
	md, _ := metadata.FromIncomingContext(ctx)
	log.Printf("Received: %v, md: %v", in.GetName(), md)
	name, _ := os.Hostname()
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(attribute.String("request.name", in.Name))
	s.span(ctx)
	return &pb.HelloReply{Message: fmt.Sprintf("hostname:%s, in:%s, md:%v", name, in.Name, md)}, nil
}

func (s *server) span(ctx context.Context) {
	ctx, span := tracer.Start(ctx, "hello-span")
	defer span.End()
	// do some work
	log.Printf("create span")
}

var tracer trace.Tracer
var resource *sdkresource.Resource
var initResourcesOnce sync.Once

var apiCounter metric.Int64Counter

func initResource() *sdkresource.Resource {
	initResourcesOnce.Do(func() {
		extraResources, _ := sdkresource.New(
			context.Background(),
			sdkresource.WithOS(),
			sdkresource.WithProcess(),
			sdkresource.WithContainer(),
			sdkresource.WithHost(),
		)
		resource, _ = sdkresource.Merge(
			sdkresource.Default(),
			extraResources,
		)
	})
	return resource
}

func initTracerProvider() *sdktrace.TracerProvider {
	ctx := context.Background()

	exporter, err := otlptracegrpc.New(ctx)
	if err != nil {
		log.Printf("new otlp trace grpc exporter failed: %v", err)
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(initResource()),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	return tp
}

func initMeterProvider() *sdkmetric.MeterProvider {
	ctx := context.Background()

	exporter, err := otlpmetricgrpc.New(ctx)
	if err != nil {
		log.Printf("new otlp metric grpc exporter failed: %v", err)
	}

	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exporter)),
		sdkmetric.WithResource(initResource()),
	)
	otel.SetMeterProvider(mp)
	return mp
}
