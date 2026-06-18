// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package githubactionsreceiver // import "github.com/v1v/opentelemetry-github-actions-receiver"

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
)

// This file implements factory for GitHub Actions receiver.

var receiverType = component.MustNewType("githubactions")

const (
	defaultBindEndpoint  = "0.0.0.0:19418"
	defaultPath          = "/ghaevents"
	tracesStability      = component.StabilityLevelAlpha
)

// NewFactory creates a new GitHub Actions receiver factory
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		receiverType,
		createDefaultConfig,
		receiver.WithTraces(createTracesReceiver, tracesStability),
	)
}

// createDefaultConfig creates the default configuration for GitHub Actions receiver.
func createDefaultConfig() component.Config {
	return &Config{
		ServerConfig: confighttp.ServerConfig{
			NetAddr: confignet.AddrConfig{
				Endpoint:  defaultBindEndpoint,
				Transport: confignet.TransportTypeTCP,
			},
		},
		Path:   defaultPath,
		Secret: "",
	}
}

// createTracesReceiver creates a trace receiver based on provided config.
func createTracesReceiver(
	_ context.Context,
	set receiver.Settings,
	cfg component.Config,
	nextConsumer consumer.Traces,
) (receiver.Traces, error) {
	rCfg := cfg.(*Config)
	if nextConsumer == nil {
		return nil, errors.New("no nextConsumer provided")
	}
	return newTracesReceiver(set, rCfg, nextConsumer)
}
