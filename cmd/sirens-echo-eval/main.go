package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"forgejo.coilysiren.me/coilyco-gaming/sirens-echo/internal/community"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// Neutral fallback only. Native evaluation runs set OTEL_EXPORTER_OTLP_ENDPOINT
// to the receiver they actually target.
const defaultEvaluationOTLPEndpoint = "http://localhost:4318"

const (
	defaultEvaluationDefinition = "agent/sirens-echo.yaml"
	defaultEvaluationPack       = "agent/evaluation.yaml"
)

func main() {
	definition, err := community.LoadDefinition(evaluationDefinitionPath())
	if err != nil {
		log.Fatalf("definition: %v", err)
	}
	localSkillpack, err := community.LoadSkillpack(definition.LocalSkillRoots)
	if err != nil {
		log.Fatalf("skillpack: %v", err)
	}
	packPath := evaluationPackPath()
	packSchema, err := community.PackSchema(packPath)
	if err != nil {
		log.Fatalf("pack schema: %v", err)
	}
	proxyURL := valueOrDefault(os.Getenv("AGENT_PROXY_URL"), community.DefaultAgentProxyURL)
	proxyModel := strings.TrimSpace(os.Getenv("AGENT_PROXY_MODEL"))
	if proxyModel == "" {
		log.Fatal("AGENT_PROXY_MODEL is required from the selected AOSH route")
	}
	timeout := 5 * time.Minute
	telemetry, err := community.NewTelemetry(context.Background(), community.Config{
		Definition:   definition,
		OTLPEndpoint: evaluationOTLPEndpoint(),
	})
	if err != nil {
		log.Fatalf("telemetry: %v", err)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := telemetry.Close(shutdownCtx); err != nil {
			log.Printf("telemetry shutdown: %v", err)
		}
	}()
	httpClient := &http.Client{
		Timeout:   timeout,
		Transport: otelhttp.NewTransport(http.DefaultTransport),
	}
	rosterPath := strings.TrimSpace(os.Getenv("SIRENS_ECHO_MCP_ROSTER"))
	client := community.ProxyClient{
		BaseURL:       proxyURL,
		Model:         proxyModel,
		AuditRole:     definition.AuditRole,
		Attribution:   definition.Identity,
		ResponseStyle: definition.ResponseStyle,
		HTTPClient:    httpClient,
		Tools: &community.MCPProvider{
			Servers:    evaluationMCPServers(rosterPath),
			HTTPClient: httpClient,
		},
		Telemetry: telemetry,
	}
	if packSchema == community.BoardSchema {
		runBoardPack(definition, localSkillpack, packPath, proxyURL, proxyModel, rosterPath, client)
		return
	}
	pack, err := community.LoadEvaluationPack(packPath)
	if err != nil {
		log.Fatalf("evaluation pack: %v", err)
	}
	if err := community.RunEvaluation(
		context.Background(),
		definition,
		community.PlaceholderPrincipal,
		localSkillpack,
		pack,
		client,
		os.Stdout,
	); err != nil {
		log.Fatalf("evaluation: %v", err)
	}
}

// runBoardPack emits the annotation dataset. It reports no verdict, so a
// non-zero exit here means the run did not happen rather than that Deep failed.
func runBoardPack(
	definition community.Definition,
	localSkillpack string,
	packPath string,
	proxyURL string,
	proxyModel string,
	rosterPath string,
	client community.CompletionClient,
) {
	pack, err := community.LoadBoardPack(packPath)
	if err != nil {
		log.Fatalf("board pack: %v", err)
	}
	provenance := community.BoardProvenance{
		Definition:  evaluationDefinitionPath(),
		Pack:        packPath,
		Model:       proxyModel,
		Transport:   proxyURL,
		Roster:      valueOrDefault(rosterPath, "empty"),
		Epochs:      boardEpochs(),
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if err := community.RunBoard(
		context.Background(),
		definition,
		community.PlaceholderPrincipal,
		localSkillpack,
		pack,
		provenance,
		client,
		os.Stdout,
	); err != nil {
		log.Fatalf("board: %v", err)
	}
}

// boardEpochs repeats each case so the grader reads epoch 1 and the rest stay
// in the dataset as a failure-spread estimate.
func boardEpochs() int {
	raw := strings.TrimSpace(os.Getenv("SIRENS_ECHO_BOARD_EPOCHS"))
	if raw == "" {
		return community.DefaultBoardEpochs
	}
	epochs, err := strconv.Atoi(raw)
	if err != nil || epochs < 1 {
		log.Fatalf("SIRENS_ECHO_BOARD_EPOCHS must be a positive integer, got %q", raw)
	}
	return epochs
}

// evaluationMCPServers uses the deployment roster when one is named, and no
// tools otherwise, so an offline run needs no MCP endpoint or secret.
func evaluationMCPServers(path string) []community.MCPServerDefinition {
	if path == "" {
		return nil
	}
	servers, err := community.LoadMCPRoster(path)
	if err != nil {
		log.Fatalf("evaluation roster: %v", err)
	}
	return servers
}

// A case addresses its agent by name and expects that profile's tools, so the
// pack travels with the definition rather than being selected independently.
func evaluationDefinitionPath() string {
	return valueOrDefault(
		os.Getenv("SIRENS_ECHO_DEFINITION"),
		defaultEvaluationDefinition,
	)
}

func evaluationPackPath() string {
	return valueOrDefault(
		os.Getenv("SIRENS_ECHO_EVALUATION_PACK"),
		defaultEvaluationPack,
	)
}

func evaluationOTLPEndpoint() string {
	return valueOrDefault(
		os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"),
		defaultEvaluationOTLPEndpoint,
	)
}

func valueOrDefault(value, fallback string) string {
	if trimmed := strings.TrimSpace(value); trimmed != "" {
		return trimmed
	}
	return fallback
}
