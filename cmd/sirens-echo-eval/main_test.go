package main

import "testing"

func TestEvaluationOTLPEndpointDefaultsToHostReachableReceiver(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")

	if got := evaluationOTLPEndpoint(); got != defaultEvaluationOTLPEndpoint {
		t.Fatalf(
			"evaluationOTLPEndpoint() = %q, want %q",
			got,
			defaultEvaluationOTLPEndpoint,
		)
	}
}

func TestEvaluationOTLPEndpointPreservesOverride(t *testing.T) {
	const override = "https://collector.example/otlp"
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", override)

	if got := evaluationOTLPEndpoint(); got != override {
		t.Fatalf("evaluationOTLPEndpoint() = %q, want %q", got, override)
	}
}

func TestEvaluationTargetsEchoByDefault(t *testing.T) {
	t.Setenv("SIRENS_ECHO_DEFINITION", "")
	t.Setenv("SIRENS_ECHO_EVALUATION_PACK", "")

	if got := evaluationDefinitionPath(); got != defaultEvaluationDefinition {
		t.Fatalf("evaluationDefinitionPath() = %q, want %q", got, defaultEvaluationDefinition)
	}
	if got := evaluationPackPath(); got != defaultEvaluationPack {
		t.Fatalf("evaluationPackPath() = %q, want %q", got, defaultEvaluationPack)
	}
}

func TestEvaluationTargetsTheSelectedProfile(t *testing.T) {
	t.Setenv("SIRENS_ECHO_DEFINITION", "agent/sirens-deep.yaml")
	t.Setenv("SIRENS_ECHO_EVALUATION_PACK", "agent/evaluation-deep.yaml")

	if got := evaluationDefinitionPath(); got != "agent/sirens-deep.yaml" {
		t.Fatalf("evaluationDefinitionPath() = %q, want the Deep definition", got)
	}
	if got := evaluationPackPath(); got != "agent/evaluation-deep.yaml" {
		t.Fatalf("evaluationPackPath() = %q, want the Deep pack", got)
	}
}
