package tel

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	health "github.com/tel-io/tel/v2/monitoring/heallth"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc/credentials"
)

var (
	ErrNoTLS    = errors.New("no tls configuration")
	ErrCaAppend = errors.New("append certs from pem")
)

const (
	// in go.opentelemetry.io/otel/sdk/resource/env declared none-exported svcNameKey
	// with: OTEL_SERVICE_NAME
	envBackPortProject = "PROJECT"
	//
	envServiceName = "OTEL_SERVICE_NAME"

	envNamespace         = "NAMESPACE"
	envDeployEnvironment = "DEPLOY_ENVIRONMENT"
	envVersion           = "VERSION"
	envLogLevel          = "LOG_LEVEL"
	envLogEncode         = "LOG_ENCODE"

	envDebug                = "DEBUG"
	envMonEnable            = "MONITOR_ENABLE"
	envOtelCompression      = "OTEL_ENABLE_COMPRESSION"
	envMon                  = "MONITOR_ADDR"
	envOtelEnable           = "OTEL_ENABLE"
	envLogOtelProcessor     = "LOGGING_OTEL_PROCESSOR"
	envLogOtelClient        = "LOGGING_OTEL_CLIENT"
	envMetricPeriodInterval = "OTEL_METRIC_PERIODIC_INTERVAL_SEC"

	evnOtel = "OTEL_COLLECTOR_GRPC_ADDR"

	envOtelInsec         = "OTEL_EXPORTER_WITH_INSECURE"
	envOtelTlsServerName = "OTEL_COLLECTOR_TLS_SERVER_NAME"
	envOtelTlsCA         = "OTEL_COLLECTOR_TLS_CA_CERT"
	envOtelCert          = "OTEL_COLLECTOR_TLS_CLIENT_CERT"
	envOtelKey           = "OTEL_COLLECTOR_TLS_CLIENT_KEY"
)

const DisableLog = "none"

// Option interface used for setting optional config properties.
type Option interface {
	apply(*Config)
}

type optionFunc func(*Config)

func (o optionFunc) apply(c *Config) {
	o(c)
}

type traceConfiguration struct {
	sampler sdktrace.Sampler
}

type OtelConfig struct {
	Enable bool `env:"OTEL_ENABLE" envDefault:"true"`

	// OtelAddr address where grpc open-telemetry exporter serve
	Addr string `env:"OTEL_COLLECTOR_GRPC_ADDR" envDefault:"0.0.0.0:4317"`
	// WithInsecure controls whether a client verifies the server's
	// certificate chain and host name. If InsecureSkipVerify is true, crypto/tls
	// accepts any certificate presented by the server and any host name in that
	// certificate. In this mode, TLS is susceptible to machine-in-the-middle
	// attacks unless custom verification is used. This should be used only for
	// testing or in combination with VerifyConnection or VerifyPeerCertificate.
	WithInsecure bool `env:"OTEL_EXPORTER_WITH_INSECURE" envDefault:"true"`

	// WithCompression enables gzip compression for all connections: logs, traces, metrics
	WithCompression bool `env:"OTEL_ENABLE_COMPRESSION" envDefault:"true"`

	MetricsPeriodicIntervalSec int `env:"OTEL_METRIC_PERIODIC_INTERVAL_SEC" envDefault:"10"`

	// ServerName is used to verify the hostname on the returned
	// certificates unless InsecureSkipVerify is given. It is also included
	// in the client's handshake to support virtual hosting unless it is
	// an IP address.
	// Disable WithInsecure option if set
	ServerName string `env:"OTEL_COLLECTOR_TLS_SERVER_NAME"`

	Logs struct {
		// OtelClient is logger of otel clients
		OtelClient bool `env:"LOGGING_OTEL_CLIENT"`

		// OtelProcessor is logger of otel processor
		OtelProcessor bool `env:"LOGGING_OTEL_PROCESSOR"`
	}

	Traces traceConfiguration

	// Raw parses a public/private key pair from a pair of
	// PEM encoded data. On successful return, Certificate.Leaf will be nil because
	// the parsed form of the certificate is not retained.
	Raw struct {
		CA   []byte `env:"OTEL_COLLECTOR_TLS_CA_CERT"`
		Cert []byte `env:"OTEL_COLLECTOR_TLS_CLIENT_CERT"`
		Key  []byte `env:"OTEL_COLLECTOR_TLS_CLIENT_KEY"`
	}

	bucketView []HistogramOpt
}

type MonitorConfig struct {
	Enable      bool   `env:"MONITOR_ENABLE" envDefault:"true"`
	MonitorAddr string `env:"MONITOR_ADDR" envDefault:"0.0.0.0:8011"`

	healthChecker []health.Checker
}

type Config struct {
	Service     string `env:"OTEL_SERVICE_NAME"`
	Namespace   string `env:"NAMESPACE"`
	Environment string `env:"DEPLOY_ENVIRONMENT"`
	Version     string `env:"VERSION"`
	LogLevel    string `env:"LOG_LEVEL" envDefault:"info"`
	// Valid values are "json", "console" or "none"
	LogEncode string `env:"LOG_ENCODE" envDefault:"json"`
	Debug     bool   `env:"DEBUG" envDefault:"false"`

	MonitorConfig
	OtelConfig
}

// HistogramOpt represent histogram bucket configuration for specific metric
type HistogramOpt struct {
	MetricName string
	Bucket     []float64
}

func DefaultConfig() Config {
	host, _ := os.Hostname()
	host = strings.ToLower(strings.ReplaceAll(host, "-", "_"))

	return Config{
		Service:     host,
		Version:     "dev",
		Namespace:   "default",
		Environment: "dev",
		LogEncode:   "json",
		LogLevel:    "info",
		MonitorConfig: MonitorConfig{
			Enable:      true,
			MonitorAddr: "0.0.0.0:8011",
		},
		OtelConfig: OtelConfig{
			Addr:                       "127.0.0.1:4317",
			WithInsecure:               true,
			Enable:                     true,
			WithCompression:            true,
			MetricsPeriodicIntervalSec: 15,
			Traces: traceConfiguration{
				sampler: sdktrace.AlwaysSample(),
			},
		},
	}
}

func DefaultDebugConfig() Config {
	c := DefaultConfig()
	c.Debug = true
	c.LogLevel = "debug"
	c.LogEncode = "console"
	c.MonitorConfig.Enable = false

	return c
}

// GetConfigFromEnv uses DefaultConfig and overwrite only variables present in env
func GetConfigFromEnv() Config {
	c := DefaultConfig()

	if val, ok := os.LookupEnv(envServiceName); ok {
		c.Service = val
	} else {
		str(envBackPortProject, &c.Service)
	}

	str(envVersion, &c.Version)
	str(envNamespace, &c.Namespace)
	str(envDeployEnvironment, &c.Environment)
	str(envLogLevel, &c.LogLevel)
	str(envLogEncode, &c.LogEncode)

	// if none console opt - use always json by default
	if c.LogEncode != "console" && c.LogEncode != DisableLog {
		c.LogEncode = "json"
	}

	str(envMon, &c.MonitorAddr)
	str(evnOtel, &c.OtelConfig.Addr)

	bl(envOtelInsec, &c.OtelConfig.WithInsecure)
	str(envOtelTlsServerName, &c.OtelConfig.ServerName)

	// ServerName provided assume insecure disabled
	if len(c.OtelConfig.ServerName) > 0 {
		c.OtelConfig.WithInsecure = false
	}

	c.Raw.CA = bt(envOtelTlsCA)
	c.Raw.Cert = bt(envOtelCert)
	c.Raw.Key = bt(envOtelKey)

	bl(envDebug, &c.Debug)
	bl(envOtelEnable, &c.OtelConfig.Enable)
	bl(envOtelCompression, &c.OtelConfig.WithCompression)
	it(envMetricPeriodInterval, &c.OtelConfig.MetricsPeriodicIntervalSec)
	bl(envMonEnable, &c.MonitorConfig.Enable)
	bl(envLogOtelProcessor, &c.Logs.OtelProcessor)
	bl(envLogOtelClient, &c.Logs.OtelClient)

	return c
}

// WithHealthCheckers provide checkers to monitoring system for check health status of service
func WithHealthCheckers(c ...health.Checker) Option {
	return optionFunc(func(config *Config) {
		config.MonitorConfig.healthChecker = append(config.MonitorConfig.healthChecker, c...)
	})
}

// WithServiceName set service name
func WithServiceName(name string) Option {
	return optionFunc(func(config *Config) {
		config.Service = name
	})
}

// WithNamespace set service namespace
func WithNamespace(ns string) Option {
	return optionFunc(func(config *Config) {
		config.Namespace = ns
	})
}

// WithMonitorEnable enable monitoring
func WithMonitorEnable(enable bool) Option {
	return optionFunc(func(config *Config) {
		config.MonitorConfig.Enable = enable
	})
}

// WithMonitoringAddr overwrite monitoring addr
func WithMonitoringAddr(addr string) Option {
	return optionFunc(func(config *Config) {
		config.MonitorConfig.MonitorAddr = addr
	})
}

// WithHistogram register metrics with custom bucket list
func WithHistogram(list ...HistogramOpt) Option {
	return optionFunc(func(config *Config) {
		config.OtelConfig.bucketView = append(config.OtelConfig.bucketView, list...)
	})
}

// WithTraceSampler allow use own sampling strategy for scrapping traces
func WithTraceSampler(sampler sdktrace.Sampler) Option {
	return optionFunc(func(config *Config) {
		config.OtelConfig.Traces.sampler = sampler
	})
}

func (c *Config) Level() zapcore.Level {
	var lvl zapcore.Level
	handleErr(lvl.Set(c.LogLevel), fmt.Sprintf("zap set log lever %q", c.LogLevel))

	return lvl
}

func (c *OtelConfig) IsTLS() bool {
	return (len(c.Raw.Cert) > 0 && len(c.Raw.Key) > 0) || len(c.Raw.CA) > 0
}

// createClientTLSCredentials up to otel-collector
func (c *OtelConfig) createClientTLSCredentials() (credentials.TransportCredentials, error) {
	if !c.IsTLS() {
		return nil, ErrNoTLS
	}

	cfg := &tls.Config{
		ServerName: c.ServerName,
	}

	if len(c.Raw.Cert) > 0 && len(c.Raw.Key) > 0 {
		cert, err := tls.X509KeyPair(c.Raw.Cert, c.Raw.Key)
		if err != nil {
			return nil, errors.WithMessage(err, "load key/pair")
		}

		cfg.Certificates = []tls.Certificate{cert}
	}

	if len(c.Raw.CA) > 0 {
		cfg.RootCAs = x509.NewCertPool()

		if !cfg.RootCAs.AppendCertsFromPEM(c.Raw.CA) {
			return nil, ErrCaAppend
		}
	}

	return credentials.NewTLS(cfg), nil
}

func bt(env string) []byte {
	if val, ok := os.LookupEnv(env); ok {
		return []byte(val)
	}

	return nil
}

func str(env string, v *string) {
	if val, ok := os.LookupEnv(env); ok {
		*v = val
	}
}

func bl(env string, v *bool) {
	if val, err := strconv.ParseBool(os.Getenv(env)); err == nil {
		*v = val
	}
}

func it(env string, v *int) {
	if val, err := strconv.ParseInt(os.Getenv(env), 10, 64); err == nil {
		*v = int(val)
	}
}