package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"forgejo.coilysiren.me/coilyco-gaming/sirens-echo/internal/community"
)

// Expands the tracked role graph, stages the admitted bodies, and writes the
// declaration agent-compose consumes. See docs/sirens-echo-compose.md.
const sourceID = "aos-public"

func main() {
	catalog := flag.String("catalog", "", "agentic-os checkout supplying the composed catalogue")
	role := flag.String("role", "", "role whose bindings to expand")
	compose := flag.String("compose-dir", "agent/compose", "directory holding roles.kdl")
	flag.Parse()
	if *catalog == "" || *role == "" {
		log.Fatal("--catalog and --role are required")
	}

	raw, err := os.ReadFile(filepath.Join(*compose, "roles.kdl"))
	if err != nil {
		log.Fatalf("read role graph: %v", err)
	}
	graph := community.ParseRoleGraph(string(raw))
	for _, id := range graph.Globals {
		if reason, private := community.PrivateRepositories[graph.Repositories[id]]; private {
			log.Fatalf("global %q resolves to private %s: %s", id, graph.Repositories[id], reason)
		}
	}

	names, err := community.ExpandRole(*catalog, *role, graph)
	if err != nil {
		log.Fatal(err)
	}

	staged := filepath.Join(*compose, "skills")
	if err := os.RemoveAll(staged); err != nil {
		log.Fatalf("clear staged tree: %v", err)
	}
	for _, name := range names {
		if err := stage(filepath.Join(*catalog, ".agents", "composed", name), filepath.Join(staged, name)); err != nil {
			log.Fatalf("stage %s: %v", name, err)
		}
	}
	declaration := filepath.Join(*compose, sourceID+".kdl")
	if err := os.WriteFile(declaration, []byte(community.RenderDeclaration(sourceID, names)), 0o644); err != nil {
		log.Fatalf("write declaration: %v", err)
	}
	fmt.Printf("role %s: %d sources admitted from %s\n", *role, len(names), *catalog)
	for _, name := range names {
		fmt.Printf("  %s\n", name)
	}
}

// stage copies one composed body under its declared path. agent-compose expects
// SKILL.md at a declared skill path, so COMPOSED.md is renamed on the way in.
func stage(source, target string) error {
	if err := os.MkdirAll(target, 0o755); err != nil {
		return err
	}
	body, err := os.ReadFile(filepath.Join(source, "COMPOSED.md"))
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(target, "SKILL.md"), body, 0o644); err != nil {
		return err
	}
	references, err := os.ReadDir(filepath.Join(source, "references"))
	if err != nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Join(target, "references"), 0o755); err != nil {
		return err
	}
	for _, entry := range references {
		if entry.IsDir() {
			continue
		}
		body, err := os.ReadFile(filepath.Join(source, "references", entry.Name()))
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(target, "references", entry.Name()), body, 0o644); err != nil {
			return err
		}
	}
	return nil
}
