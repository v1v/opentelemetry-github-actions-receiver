module github.com/v1v/opentelemetry-github-actions-receiver

go 1.24.0

toolchain go1.24.2

require (
	github.com/google/go-github/v78 v78.0.0
	github.com/stretchr/testify v1.9.0
	go.opentelemetry.io/collector/component v0.102.0
	go.opentelemetry.io/collector/config/confighttp v0.102.0
	go.opentelemetry.io/collector/confmap v0.102.0
	go.opentelemetry.io/collector/consumer v0.102.0
	go.opentelemetry.io/collector/pdata v1.9.0
	go.opentelemetry.io/collector/receiver v0.102.0
	go.opentelemetry.io/otel/trace v1.27.0
	go.uber.org/multierr v1.11.0
	go.uber.org/zap v1.27.0
)
