// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package githubactionsreceiver

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"

	"github.com/google/go-github/v78/github"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

func TestCreateNewTracesReceiver(t *testing.T) {
	defaultConfig := createDefaultConfig().(*Config)

	tests := []struct {
		desc     string
		config   Config
		consumer consumer.Traces
		err      error
	}{
		{
			desc:     "Default config succeeds",
			config:   *defaultConfig,
			consumer: consumertest.NewNop(),
			err:      nil,
		},
		{
			desc: "User defined config success",
			config: Config{
				ServerConfig: confighttp.ServerConfig{
					Endpoint: "localhost:8080",
				},
				Secret: "mysecret",
			},
			consumer: consumertest.NewNop(),
		},
		{
			desc: "Missing endpoint fails",
			config: Config{
				ServerConfig: confighttp.ServerConfig{},
			},
			consumer: consumertest.NewNop(),
			err:      errMissingEndpoint,
		},
		{
			desc: "TLS config success",
			config: Config{
				ServerConfig: confighttp.ServerConfig{
					Endpoint:   "localhost:8080",
					TLSSetting: &configtls.ServerConfig{},
				},
			},
			consumer: consumertest.NewNop(),
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			rec, err := newTracesReceiver(receivertest.NewNopSettings(), &test.config, test.consumer)
			if test.err == nil {
				require.NotNil(t, rec)
			} else {
				require.ErrorIs(t, err, test.err)
				require.Nil(t, rec)
			}
		})
	}
}

func TestEventToTracesUnknownEvent(t *testing.T) {
	logger := zaptest.NewLogger(t)

	_, err := eventToTraces(struct{ name string }{name: "unsupported"}, &Config{}, logger)
	require.Error(t, err)
	require.Equal(t, "unknown event type", err.Error())
}

func TestGenerateServiceName(t *testing.T) {
	tests := []struct {
		desc     string
		cfg      *Config
		fullName string
		expected string
	}{
		{
			desc: "Custom service name overrides all",
			cfg: &Config{
				CustomServiceName: "custom-service",
				ServiceNamePrefix: "pre-",
				ServiceNameSuffix: "-suf",
			},
			fullName: "Org/Repo_Name",
			expected: "custom-service",
		},
		{
			desc: "Name formatted with prefix and suffix",
			cfg: &Config{
				ServiceNamePrefix: "pre-",
				ServiceNameSuffix: "-suf",
			},
			fullName: "Org/Repo_Name",
			expected: "pre-org-repo-name-suf",
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			require.Equal(t, tc.expected, generateServiceName(tc.cfg, tc.fullName))
		})
	}
}

func TestEventToTracesTraces(t *testing.T) {
	tests := []struct {
		desc            string
		payloadFilePath string
		eventType       string
		expectedError   error
		expectedSpans   int
	}{
		{
			desc:            "WorkflowJobEvent processing",
			payloadFilePath: "./testdata/completed/5_workflow_job_completed.json",
			eventType:       "workflow_job",
			expectedError:   nil,
			expectedSpans:   10, // 10 spans in the payload
		},
		{
			desc:            "WorkflowRunEvent processing",
			payloadFilePath: "./testdata/completed/8_workflow_run_completed.json",
			eventType:       "workflow_run",
			expectedError:   nil,
			expectedSpans:   1, // Root span
		},
	}

	logger := zaptest.NewLogger(t)
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			payload, err := os.ReadFile(test.payloadFilePath)
			require.NoError(t, err)

			event, err := github.ParseWebHook(test.eventType, payload)
			require.NoError(t, err)

			traces, err := eventToTraces(event, &Config{}, logger)

			if test.expectedError != nil {
				require.Error(t, err)
				require.Equal(t, test.expectedError, err)
			} else {
				require.NoError(t, err)
			}

			require.Equal(t, test.expectedSpans, traces.SpanCount(), fmt.Sprintf("%s: unexpected number of spans", test.desc))
		})
	}
}

func TestProcessSteps(t *testing.T) {
	tests := []struct {
		desc             string
		givenSteps       []*github.TaskStep
		expectedSpans    int
		expectedStatuses []ptrace.StatusCode
	}{
		{
			desc: "Multiple steps with mixed status",

			givenSteps: []*github.TaskStep{
				{Name: getPtr("Checkout"), Status: getPtr("completed"), Conclusion: getPtr("success")},
				{Name: getPtr("Build"), Status: getPtr("completed"), Conclusion: getPtr("failure")},
				{Name: getPtr("Test"), Status: getPtr("completed"), Conclusion: getPtr("success")},
			},
			expectedSpans: 4, // Includes parent span
			expectedStatuses: []ptrace.StatusCode{
				ptrace.StatusCodeOk,
				ptrace.StatusCodeError,
				ptrace.StatusCodeOk,
			},
		},
		{
			desc:             "No steps",
			givenSteps:       []*github.TaskStep{},
			expectedSpans:    1, // Only the parent span should be created
			expectedStatuses: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			logger := zap.NewNop()
			traces := ptrace.NewTraces()
			rs := traces.ResourceSpans().AppendEmpty()
			ss := rs.ScopeSpans().AppendEmpty()

			traceID, _ := generateTraceID(123, 1)
			parentSpanID := createParentSpan(ss, tc.givenSteps, &github.WorkflowJob{}, traceID, logger)

			processSteps(ss, tc.givenSteps, &github.WorkflowJob{}, traceID, parentSpanID, logger)

			startIdx := 1 // Skip the parent span if it's the first one
			if len(tc.expectedStatuses) == 0 {
				startIdx = 0 // No steps, only the parent span exists
			}

			require.Equal(t, tc.expectedSpans, ss.Spans().Len(), "Unexpected number of spans")
			for i, expectedStatusCode := range tc.expectedStatuses {
				span := ss.Spans().At(i + startIdx)
				statusCode := span.Status().Code()
				require.Equal(t, expectedStatusCode, statusCode, fmt.Sprintf("Unexpected status code for span #%d", i+startIdx))
			}
		})
	}
}

func TestResourceAndSpanAttributesCreation(t *testing.T) {
	tests := []struct {
		desc            string
		payloadFilePath string
		expectedSteps   []map[string]string
	}{
		{
			desc:            "WorkflowJobEvent Step Attributes",
			payloadFilePath: "./testdata/completed/5_workflow_job_completed.json",
			expectedSteps: []map[string]string{
				{"ci.github.workflow.job.step.name": "Set up job", "ci.github.workflow.job.step.number": "1"},
				{"ci.github.workflow.job.step.name": "Run actions/checkout@v3", "ci.github.workflow.job.step.number": "2"},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			logger := zaptest.NewLogger(t)

			payload, err := os.ReadFile(tc.payloadFilePath)
			require.NoError(t, err)

			event, err := github.ParseWebHook("workflow_job", payload)
			require.NoError(t, err)

			traces, err := eventToTraces(event, &Config{}, logger)
			require.NoError(t, err)

			rs := traces.ResourceSpans().At(0)
			ss := rs.ScopeSpans().At(0)

			for _, expectedStep := range tc.expectedSteps {
				stepFound := false

				for i := 0; i < ss.Spans().Len() && !stepFound; i++ {
					span := ss.Spans().At(i)
					attrs := span.Attributes()

					stepValue, found := attrs.Get("ci.github.workflow.job.step.name")
					stepName := stepValue.Str()

					if !found || stepName == "" { // Skip if the attribute is not found or name is empty
						continue
					}

					expectedStepName := expectedStep["ci.github.workflow.job.step.name"]

					if stepName == expectedStepName {
						stepFound = true
						for attrKey, expectedValue := range expectedStep {
							attrValue, found := attrs.Get(attrKey)
							if !found {
								require.Fail(t, fmt.Sprintf("Attribute '%s' not found in span for step '%s'", attrKey, stepName))
								continue
							}
							actualValue := attributeValueToString(attrValue)
							require.Equal(t, expectedValue, actualValue, "Attribute '%s' does not match expected value for step '%s'", attrKey, stepName)
						}
					}
				}

				require.True(t, stepFound, "Step '%s' not found in any span", expectedStep["ci.github.workflow.job.step.name"])
			}

		})
	}
}

// attributeValueToString converts an attribute value to a string regardless of its actual type
func attributeValueToString(attr pcommon.Value) string {
	switch attr.Type() {
	case pcommon.ValueTypeStr:
		return attr.Str()
	case pcommon.ValueTypeInt:
		return strconv.FormatInt(attr.Int(), 10)
	case pcommon.ValueTypeDouble:
		return strconv.FormatFloat(attr.Double(), 'f', -1, 64)
	case pcommon.ValueTypeBool:
		return strconv.FormatBool(attr.Bool())
	case pcommon.ValueTypeMap:
		return "<Map Value>"
	case pcommon.ValueTypeSlice:
		return "<Slice Value>"
	default:
		return "<Unknown Value Type>"
	}
}

func getPtr(str string) *string {
	return &str
}

type errTracesConsumer struct{}

func (e *errTracesConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (e *errTracesConsumer) ConsumeTraces(context.Context, ptrace.Traces) error {
	return errors.New("consumer failed")
}

func signedWebhookRequest(t *testing.T, path string, payload []byte, eventType, secret string) *http.Request {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(payload))

	mac := hmac.New(sha256.New, []byte(secret))
	_, err := mac.Write(payload)
	require.NoError(t, err)

	req.Header.Set("X-Hub-Signature-256", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	req.Header.Set("X-GitHub-Event", eventType)
	req.Header.Set("Content-Type", "application/json")

	return req
}

func TestConvertPRURL(t *testing.T) {
	apiURL := "https://api.github.com/repos/example/repo/pulls/123"
	require.Equal(t, "https://github.com/example/repo/pull/123", convertPRURL(apiURL))
}

func TestReceiverStartShutdown(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = "127.0.0.1:0"

	rec, err := newTracesReceiver(receivertest.NewNopSettings(), cfg, consumertest.NewNop())
	require.NoError(t, err)

	require.NoError(t, rec.Start(context.Background(), nil))
	require.NoError(t, rec.Shutdown(context.Background()))
}

func TestServeHTTP_PathNotFound(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	rec, err := newTracesReceiver(receivertest.NewNopSettings(), cfg, consumertest.NewNop())
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/wrong-path", bytes.NewReader([]byte("{}")))
	rr := httptest.NewRecorder()

	rec.ServeHTTP(rr, req)
	require.Equal(t, http.StatusNotFound, rr.Code)
}

func TestServeHTTP_InvalidSignature(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Secret = "top-secret"
	rec, err := newTracesReceiver(receivertest.NewNopSettings(), cfg, consumertest.NewNop())
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, cfg.Path, bytes.NewReader([]byte(`{"hook_id":1}`)))
	req.Header.Set("X-GitHub-Event", "ping")
	req.Header.Set("X-Hub-Signature-256", "sha256=invalid")
	rr := httptest.NewRecorder()

	rec.ServeHTTP(rr, req)
	require.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestServeHTTP_UnsupportedEvent(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Secret = "top-secret"
	rec, err := newTracesReceiver(receivertest.NewNopSettings(), cfg, consumertest.NewNop())
	require.NoError(t, err)

	payload := []byte(`{"zen":"keep it logically awesome","hook_id":1}`)
	req := signedWebhookRequest(t, cfg.Path, payload, "ping", cfg.Secret)
	rr := httptest.NewRecorder()

	rec.ServeHTTP(rr, req)
	require.Equal(t, http.StatusNoContent, rr.Code)
}

func TestServeHTTP_WorkflowJobNotCompleted(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Secret = "top-secret"
	rec, err := newTracesReceiver(receivertest.NewNopSettings(), cfg, consumertest.NewNop())
	require.NoError(t, err)

	payload, err := os.ReadFile("./testdata/queued/1_workflow_job_queued.json")
	require.NoError(t, err)

	req := signedWebhookRequest(t, cfg.Path, payload, "workflow_job", cfg.Secret)
	rr := httptest.NewRecorder()

	rec.ServeHTTP(rr, req)
	require.Equal(t, http.StatusNoContent, rr.Code)
}

func TestServeHTTP_WorkflowRunNotCompleted(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Secret = "top-secret"
	rec, err := newTracesReceiver(receivertest.NewNopSettings(), cfg, consumertest.NewNop())
	require.NoError(t, err)

	payload, err := os.ReadFile("./testdata/requested/1_workflow_run_requested.json")
	require.NoError(t, err)

	req := signedWebhookRequest(t, cfg.Path, payload, "workflow_run", cfg.Secret)
	rr := httptest.NewRecorder()

	rec.ServeHTTP(rr, req)
	require.Equal(t, http.StatusNoContent, rr.Code)
}

func TestServeHTTP_WorkflowCompletedAccepted(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Secret = "top-secret"
	sink := &consumertest.TracesSink{}
	rec, err := newTracesReceiver(receivertest.NewNopSettings(), cfg, sink)
	require.NoError(t, err)

	payload, err := os.ReadFile("./testdata/completed/5_workflow_job_completed.json")
	require.NoError(t, err)

	req := signedWebhookRequest(t, cfg.Path, payload, "workflow_job", cfg.Secret)
	rr := httptest.NewRecorder()

	rec.ServeHTTP(rr, req)
	require.Equal(t, http.StatusAccepted, rr.Code)
	require.Greater(t, sink.SpanCount(), 0)
}

func TestServeHTTP_ConsumerError(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Secret = "top-secret"
	rec, err := newTracesReceiver(receivertest.NewNopSettings(), cfg, &errTracesConsumer{})
	require.NoError(t, err)

	payload, err := os.ReadFile("./testdata/completed/5_workflow_job_completed.json")
	require.NoError(t, err)

	req := signedWebhookRequest(t, cfg.Path, payload, "workflow_job", cfg.Secret)
	rr := httptest.NewRecorder()

	rec.ServeHTTP(rr, req)
	require.Equal(t, http.StatusInternalServerError, rr.Code)
}
