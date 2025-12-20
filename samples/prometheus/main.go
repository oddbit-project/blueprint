package main

import (
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/oddbit-project/blueprint/log"
	"github.com/oddbit-project/blueprint/provider/httpserver"
	"github.com/oddbit-project/blueprint/provider/prometheus"
	prom "github.com/prometheus/client_golang/prometheus"
)

// Custom collector that tracks application metrics
type AppMetrics struct {
	requestsTotal   *prom.CounterVec
	requestDuration *prom.HistogramVec
	activeUsers     prom.Gauge
}

func NewAppMetrics() *AppMetrics {
	return &AppMetrics{
		requestsTotal: prom.NewCounterVec(
			prom.CounterOpts{
				Name: "app_requests_total",
				Help: "Total number of requests by endpoint and status",
			},
			[]string{"endpoint", "status"},
		),
		requestDuration: prom.NewHistogramVec(
			prom.HistogramOpts{
				Name:    "app_request_duration_seconds",
				Help:    "Request duration in seconds",
				Buckets: prom.DefBuckets,
			},
			[]string{"endpoint"},
		),
		activeUsers: prom.NewGauge(
			prom.GaugeOpts{
				Name: "app_active_users",
				Help: "Number of currently active users",
			},
		),
	}
}

// Describe implements prometheus.Collector
func (m *AppMetrics) Describe(ch chan<- *prom.Desc) {
	m.requestsTotal.Describe(ch)
	m.requestDuration.Describe(ch)
	m.activeUsers.Describe(ch)
}

// Collect implements prometheus.Collector
func (m *AppMetrics) Collect(ch chan<- prom.Metric) {
	m.requestsTotal.Collect(ch)
	m.requestDuration.Collect(ch)
	m.activeUsers.Collect(ch)
}

// RecordRequest records a request metric
func (m *AppMetrics) RecordRequest(endpoint, status string, duration time.Duration) {
	m.requestsTotal.WithLabelValues(endpoint, status).Inc()
	m.requestDuration.WithLabelValues(endpoint).Observe(duration.Seconds())
}

// SetActiveUsers sets the current active users gauge
func (m *AppMetrics) SetActiveUsers(count float64) {
	m.activeUsers.Set(count)
}

func main() {
	log.Configure(log.NewDefaultConfig())
	logger := log.New("prometheus-sample")

	// create custom metrics collector
	appMetrics := NewAppMetrics()

	// Option 1: Standalone prometheus server
	// Uncomment to use a dedicated metrics server on port 2220
	/*
		promConfig := prometheus.NewConfig()
		promServer, err := prometheus.NewServer(promConfig, logger, appMetrics)
		if err != nil {
			logger.Fatal(err, "could not start prometheus server")
			os.Exit(1)
		}
		go promServer.Start()
		logger.Info("Prometheus metrics available at http://localhost:2220/metrics")
	*/

	// Option 2: Integrate with existing httpserver
	httpConfig := httpserver.NewServerConfig()
	httpConfig.Host = "localhost"
	httpConfig.Port = 8089
	httpConfig.Debug = true

	server, err := httpserver.NewServer(httpConfig, logger)
	if err != nil {
		logger.Fatal(err, "could not start http server")
		os.Exit(1)
	}

	// register prometheus metrics endpoint on existing server
	prometheus.Register(server, "/metrics", appMetrics)
	logger.Info("Prometheus metrics available at http://localhost:8089/metrics")

	// sample API endpoint that records metrics
	server.Route().GET("/hello", func(c *gin.Context) {
		start := time.Now()

		// simulate some work
		time.Sleep(time.Duration(rand.Intn(100)) * time.Millisecond)

		c.JSON(http.StatusOK, gin.H{
			"message": "hello!",
		})

		appMetrics.RecordRequest("/hello", "200", time.Since(start))
	})

	server.Route().GET("/error", func(c *gin.Context) {
		start := time.Now()

		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "something went wrong",
		})

		appMetrics.RecordRequest("/error", "500", time.Since(start))
	})

	// simulate active users changing over time
	go func() {
		for {
			appMetrics.SetActiveUsers(float64(rand.Intn(100)))
			time.Sleep(5 * time.Second)
		}
	}()

	// graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		fmt.Println("\nShutting down...")
		os.Exit(0)
	}()

	// start http server
	logger.Info("Server running on http://localhost:8089")
	logger.Info("Try: curl http://localhost:8089/hello")
	logger.Info("Try: curl http://localhost:8089/metrics")
	server.Start()
}
