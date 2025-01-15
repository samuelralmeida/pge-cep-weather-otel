package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/samuelralmeida/pge-cep-weather-otel/internal/handler"
	"github.com/samuelralmeida/pge-cep-weather-otel/internal/tracing"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.opentelemetry.io/otel"
)

type config struct {
	port              string
	weatherServiceUrl string
	collectorUrl      string
	serviceName       string
	weatherApiKey     string
}

var conf config

func init() {
	port := os.Getenv("SERVICE_PORT")
	weatherServiceUrl := os.Getenv("WEATHER_SERVICE_URL")
	collectorUrl := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	weatherApiKey := os.Getenv("WEATHER_API_KEY")
	serviceName := os.Getenv("SERVICE_NAME")

	conf = config{
		port:              port,
		weatherServiceUrl: weatherServiceUrl,
		collectorUrl:      collectorUrl,
		serviceName:       serviceName,
		weatherApiKey:     weatherApiKey,
	}
}

func main() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	shutdown, err := tracing.InitProvider(conf.serviceName, conf.collectorUrl)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := shutdown(ctx); err != nil {
			log.Fatal("failed to shutdown TracerProvider: %w", err)
		}
	}()

	tracer := otel.Tracer("microservice-tracer")
	h := handler.Handler{Tracer: tracer, WeatherServiceUrl: conf.weatherServiceUrl, WeatherApiKey: conf.weatherApiKey}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Use(middleware.Timeout(60 * time.Second))

	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		w.Write([]byte("route does not exist"))
	})

	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(405)
		w.Write([]byte("method is not valid"))
	})

	r.Post("/", h.InputHandler)
	r.Get("/weather/{cep}", h.WeatherHandler)

	go func() {
		log.Println("listening on port", conf.port)
		if err := http.ListenAndServe(":"+conf.port, r); err != nil {
			log.Fatal(err)
		}
	}()

	select {
	case <-sigCh:
		log.Println("shutting down gracefully, ctrl+c pressed...")
	case <-ctx.Done():
		log.Println("shutting down due to other reason...")
	}

	_, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
}
