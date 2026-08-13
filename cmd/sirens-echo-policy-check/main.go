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
	for _, path := range []string{
		"agent/evaluation.yaml",
		"agent/evaluation-deep.yaml",
	} {
		verifyEvaluationPack(path)
	}
	for _, path := range []string{
		"agent/board-deep.yaml",
	} {
		verifyBoardPack(path)
	}
	for _, path := range []string{
		"agent/rate-deep.yaml",
		"agent/rate-fixture-deep.yaml",
	} {
		verifyRatePack(path)
	}
	for _, path := range []string{
		"agent/tool-fixture-injection.yaml",
	} {
		verifyFixturePack(path)
	}
	verifyContentTaxonomy("agent/content-classes.yaml")
	verifyAccessPolicy("docs/access-policy.reference.yaml")
	// A deployment can point the gate at its own file, so an operator can check
	// a candidate ConfigMap before the rollout that would otherwise fail closed.
	if path := os.Getenv("SIRENS_ECHO_ACCESS_POLICY"); path != "" {
		verifyAccessPolicy(path)
	}
}

func verifyEvaluationPack(path string) {
	pack, err := community.LoadEvaluationPack(path)
	if err != nil {
		log.Fatalf("evaluation pack %s: %v", path, err)
	}
	fmt.Printf("verified evaluation pack %s with %d cases\n", path, len(pack.Cases))
}

// The board never gates a deployment, but a pack that does not load is a pack
// nobody can grade, so the build still refuses to ship a broken one.
func verifyBoardPack(path string) {
	pack, err := community.LoadBoardPack(path)
	if err != nil {
		log.Fatalf("board pack %s: %v", path, err)
	}
	fmt.Printf(
		"verified board pack %s with %d cases across %d pairs\n",
		path,
		len(pack.Cases),
		len(pack.Cases)/2,
	)
}

// The rate pack gates nothing either, but a pack that does not load produces no
// measurement, and a missing measurement reads as a clean one.
func verifyRatePack(path string) {
	pack, err := community.LoadRatePack(path)
	if err != nil {
		log.Fatalf("rate pack %s: %v", path, err)
	}
	runs := 0
	for _, rateCase := range pack.Cases {
		runs += rateCase.Runs
	}
	fmt.Printf(
		"verified rate pack %s with %d cases and %d total runs\n",
		path,
		len(pack.Cases),
		runs,
	)
}

// A taxonomy that is not closed forces a wrong classification rather than
// producing a bad one, which is harder to notice at a member.
func verifyContentTaxonomy(path string) {
	taxonomy, err := community.LoadContentTaxonomy(path)
	if err != nil {
		log.Fatalf("content taxonomy %s: %v", path, err)
	}
	denied := 0
	for _, class := range taxonomy.Classes {
		if class.Deny {
			denied++
		}
	}
	fmt.Printf(
		"verified content taxonomy %s with %d classes, %d denied\n",
		path,
		len(taxonomy.Classes),
		denied,
	)
}

// A fixture pack that does not load leaves its cases reaching no tool, and a
// case that fetched nothing reads as a case that found nothing.
func verifyFixturePack(path string) {
	pack, err := community.LoadFixturePack(path)
	if err != nil {
		log.Fatalf("tool fixture %s: %v", path, err)
	}
	if len(pack.Tools) == 0 {
		log.Fatalf("tool fixture %s declares no tools", path)
	}
	fmt.Printf("verified tool fixture %s with %d tools\n", path, len(pack.Tools))
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
	composed := ""
	if definition.Composed {
		composed = community.PlaceholderComposed
	}
	prompt := community.BuildSystemPrompt(definition, principal, composed, localPolicy)
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
