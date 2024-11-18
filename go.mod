module github.com/v1v/opentelemetry-github-actions-receiver

go 1.22.0

toolchain go1.23.2

require (
	github.com/google/go-github/v61 v61.0.0
	github.com/stretchr/testify v1.9.0
	go.opentelemetry.io/collector/component v0.102.0
	go.opentelemetry.io/collector/config/confighttp v0.102.0
	go.opentelemetry.io/collector/confmap v0.102.0
	go.opentelemetry.io/collector/consumer v0.102.0
	go.opentelemetry.io/collector/pdata v1.9.0
	go.opentelemetry.io/collector/receiver v0.102.0
	go.opentelemetry.io/otel/trace v1.32.0
	go.uber.org/multierr v1.11.0
	go.uber.org/zap v1.27.0
)

require (
	go.opentelemetry.io/collector/semconv v0.113.0 // indirect
	go.opentelemetry.io/otel v1.32.0 // indirect
)
