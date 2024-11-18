// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package githubactionsreceiver

import (
	"strings"
	"time"

	"github.com/google/go-github/v61/github"
	"go.opentelemetry.io/collector/pdata/pcommon"
	semconv "go.opentelemetry.io/collector/semconv/v1.27.0"
	"go.uber.org/zap"
)

func createResourceAttributes(resource pcommon.Resource, event interface{}, config *Config, logger *zap.Logger) {
	attrs := resource.Attributes()

	switch e := event.(type) {
	case *github.WorkflowJobEvent:
		serviceName := generateServiceName(config, e.GetRepo().GetFullName())
		attrs.PutStr(semconv.AttributeServiceName, serviceName)

		attrs.PutStr(semconv.AttributeCicdPipelineName, e.GetWorkflowJob().GetWorkflowName())
		attrs.PutStr(AttributeCicdSystem, "github")
		attrs.PutStr(semconv.AttributeCicdPipelineTaskType, "job")
		attrs.PutStr("cicd.pipeline.task.created_at", e.GetWorkflowJob().GetCreatedAt().Format(time.RFC3339))
		attrs.PutStr("cicd.pipeline.task.completed_at", e.GetWorkflowJob().GetCompletedAt().Format(time.RFC3339))
		attrs.PutStr("cicd.pipeline.task.conclusion", e.GetWorkflowJob().GetConclusion())
		attrs.PutStr(semconv.AttributeVcsRepositoryRefName, e.GetWorkflowJob().GetHeadBranch())
		attrs.PutStr(semconv.AttributeVcsRepositoryRefRevision, e.GetWorkflowJob().GetHeadSHA())
		attrs.PutStr(semconv.AttributeCicdPipelineTaskRunURLFull, e.GetWorkflowJob().GetHTMLURL())
		attrs.PutInt(semconv.AttributeCicdPipelineTaskRunID, e.GetWorkflowJob().GetID())

		attrs.PutStr(semconv.AttributeCicdPipelineTaskName, e.GetWorkflowJob().GetName())
		attrs.PutInt("cicd.pipeline.task.run.attempt", e.GetWorkflowJob().GetRunAttempt())
		attrs.PutInt(semconv.AttributeCicdPipelineTaskRunID, e.GetWorkflowJob().GetRunID())
		attrs.PutStr("cicd.pipeline.task.runner.group_name", e.GetWorkflowJob().GetRunnerGroupName())
		attrs.PutStr("cicd.pipeline.task.runner.name", e.GetWorkflowJob().GetRunnerName())
		attrs.PutStr("cicd.pipeline.task.started_at", e.GetWorkflowJob().GetStartedAt().Format(time.RFC3339))
		attrs.PutStr("cicd.pipeline.task.status", e.GetWorkflowJob().GetStatus())

		attrs.PutStr(semconv.AttributeVcsRepositoryURLFull, e.GetRepo().GetURL())

	case *github.WorkflowRunEvent:
		serviceName := generateServiceName(config, e.GetRepo().GetFullName())
		attrs.PutStr(semconv.AttributeServiceName, serviceName)
		attrs.PutStr(AttributeCicdSystem, "github")
		attrs.PutStr(semconv.AttributeCicdPipelineTaskType, "run")
		attrs.PutStr("cicd.pipeline.task.conclusion", e.GetWorkflowRun().GetConclusion())
		attrs.PutStr("cicd.pipeline.task.created_at", e.GetWorkflowRun().GetCreatedAt().Format(time.RFC3339))
		attrs.PutStr("cicd.pipeline.task.event", e.GetWorkflowRun().GetEvent())
		attrs.PutStr(semconv.AttributeCicdPipelineTaskRunURLFull, e.GetWorkflowRun().GetHTMLURL())
		attrs.PutInt("cicd.pipeline.run.id", e.GetWorkflowRun().GetID())
		attrs.PutStr("cicd.pipeline.task.name", e.GetWorkflowRun().GetName())
		attrs.PutStr("ci.github.workflow.run.path", e.GetWorkflow().GetPath())

		// NOTE: Maybe some distributed tracing attributes here?
		if len(e.GetWorkflowRun().ReferencedWorkflows) > 0 {
			var referencedWorkflows []string
			for _, workflow := range e.GetWorkflowRun().ReferencedWorkflows {
				referencedWorkflows = append(referencedWorkflows, workflow.GetPath())
			}
			attrs.PutStr("ci.github.workflow.run.referenced_workflows", strings.Join(referencedWorkflows, ";"))
		}

		attrs.PutInt("ci.github.workflow.run.run_attempt", int64(e.GetWorkflowRun().GetRunAttempt()))
		attrs.PutStr("ci.github.workflow.run.run_started_at", e.GetWorkflowRun().RunStartedAt.Format(time.RFC3339))
		attrs.PutStr("ci.github.workflow.run.status", e.GetWorkflowRun().GetStatus())
		attrs.PutStr("ci.github.workflow.run.sender.login", e.GetSender().GetLogin())
		attrs.PutStr("ci.github.workflow.run.triggering_actor.login", e.GetWorkflowRun().GetTriggeringActor().GetLogin())
		attrs.PutStr("ci.github.workflow.run.updated_at", e.GetWorkflowRun().GetUpdatedAt().Format(time.RFC3339))
		duration := e.GetWorkflowRun().GetUpdatedAt().Sub(e.GetWorkflowRun().GetRunStartedAt().Time)
		attrs.PutInt("ci.github.workflow.run.duration_millis", duration.Milliseconds())

		attrs.PutStr("vcs.system", "git")
		attrs.PutStr(semconv.AttributeVcsRepositoryRefName, e.GetWorkflowRun().GetHeadBranch())
		attrs.PutStr(semconv.AttributeVcsRepositoryRefRevision, e.GetWorkflowRun().GetHeadSHA())

		if len(e.GetWorkflowRun().PullRequests) > 0 {
			var prUrls []string
			for _, pr := range e.GetWorkflowRun().PullRequests {
				prUrls = append(prUrls, convertPRURL(pr.GetURL()))
			}
			attrs.PutStr("vcs.repository.change.url.full", strings.Join(prUrls, ";"))
		}

		attrs.PutStr(semconv.AttributeVcsRepositoryURLFull, e.GetRepo().GetURL())

	default:
		logger.Error("unknown event type")
	}
}
