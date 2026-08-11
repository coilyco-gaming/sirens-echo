package main

import (
	"fmt"
	"log"
	"os"

	"forgejo.coilysiren.me/coilyco-gaming/sirens-echo/internal/community"
)

// This binary runs during the image build. It may only read paths the
// Dockerfile copies into the build stage.
func main() {
	for _, path := range []string{
		"agent/sirens-echo.yaml",
		"agent/sirens-deep.yaml",
	} {
		verify(path)
	}
	verifyAccessPolicy("docs/access-policy.reference.yaml")
	// A deployment can point the gate at its own file, so an operator can check
	// a candidate ConfigMap before the rollout that would otherwise fail closed.
	if path := os.Getenv("SIRENS_ECHO_ACCESS_POLICY"); path != "" {
		verifyAccessPolicy(path)
	}
}

func verifyAccessPolicy(path string) {
	policy, err := community.LoadAccessPolicy(path)
	if err != nil {
		log.Fatalf("access policy %s: %v", path, err)
	}
	fmt.Printf(
		"verified access policy %s with %d guilds and %d direct-message accounts\n",
		path,
		len(policy.Guilds),
		len(policy.DirectMessages.Allow),
	)
}

func verify(path string) {
	definition, err := community.LoadDefinition(path)
	if err != nil {
		log.Fatalf("definition %s: %v", path, err)
	}
	localPolicy, err := community.LoadSkillpack(definition.LocalSkillRoots)
	if err != nil {
		log.Fatalf("local policy %s: %v", path, err)
	}
	principal := community.PlaceholderPrincipal
	prompt := community.BuildSystemPrompt(definition, principal, localPolicy)
	if err := community.ValidateSystemPrompt(definition, principal, prompt); err != nil {
		log.Fatalf("response policy %s: %v", path, err)
	}
	fmt.Printf(
		"verified %s response policy with %d bytes and %d local roots\n",
		definition.ResponseStyle,
		len(prompt),
		len(definition.LocalSkillRoots),
	)
}
