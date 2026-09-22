package community

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// RoutingSchemaV1 is route.jev's own labeled acceptance set, distinct from
// EvaluationPack, which scores the reply's text rather than the router's picks.
const RoutingSchemaV1 = "sirens-discord-ops.routing.v1"

// RoutingPack is a labeled set of turns and the route.jev decision each
// should produce. The scorer lands with PR 3's eval judge.
type RoutingPack struct {
	Schema string        `json:"schema" yaml:"schema"`
	Cases  []RoutingCase `json:"cases" yaml:"cases"`
}

// RoutingCase names one turn and the route.jev families it exercises. An
// empty Expect* field means the case does not test that family.
type RoutingCase struct {
	ID      string            `json:"id" yaml:"id"`
	History []TranscriptEntry `json:"history" yaml:"history"`
	Current TranscriptEntry   `json:"current" yaml:"current"`

	// ExpectDeniedClasses lists classes the content family must catch. Pair
	// an empty list with ExpectAllowed to assert clean content instead.
	ExpectDeniedClasses []string `json:"expect_denied_classes,omitempty" yaml:"expect_denied_classes,omitempty"`
	// ExpectAllowed asserts the content family denies nothing. Its own field
	// rather than an empty ExpectDeniedClasses, which an unrelated case also leaves unset.
	ExpectAllowed bool `json:"expect_allowed,omitempty" yaml:"expect_allowed,omitempty"`
	// ExpectDrawers lists reference paths the drawer family must ship.
	ExpectDrawers []string `json:"expect_drawers,omitempty" yaml:"expect_drawers,omitempty"`
	// ExpectFocus is the game-focus root the focus family must pick, or "neither".
	ExpectFocus string `json:"expect_focus,omitempty" yaml:"expect_focus,omitempty"`
	// ExpectShape is the option the shape family must pick; "full" is valid.
	ExpectShape string `json:"expect_shape,omitempty" yaml:"expect_shape,omitempty"`
	// ExpectServers lists MCP servers the server family must keep.
	ExpectServers []string `json:"expect_servers,omitempty" yaml:"expect_servers,omitempty"`
}

// checked reports whether a case asserts anything, matching
// EvaluationCase.checked(): an unchecked case reads as coverage it lacks.
func (c RoutingCase) checked() bool {
	return len(c.ExpectDeniedClasses) > 0 ||
		c.ExpectAllowed ||
		len(c.ExpectDrawers) > 0 ||
		c.ExpectFocus != "" ||
		c.ExpectShape != "" ||
		len(c.ExpectServers) > 0
}

// LoadRoutingPack reads and validates a routing set.
func LoadRoutingPack(path string) (RoutingPack, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return RoutingPack{}, fmt.Errorf("read routing pack: %w", err)
	}
	var pack RoutingPack
	if err := yaml.Unmarshal(raw, &pack); err != nil {
		return RoutingPack{}, fmt.Errorf("parse routing pack: %w", err)
	}
	if pack.Schema != RoutingSchemaV1 {
		return RoutingPack{}, fmt.Errorf("unsupported routing schema %q", pack.Schema)
	}
	if len(pack.Cases) == 0 {
		return RoutingPack{}, fmt.Errorf("routing pack contains no cases")
	}
	seen := make(map[string]struct{}, len(pack.Cases))
	for _, routingCase := range pack.Cases {
		if routingCase.ID == "" {
			return RoutingPack{}, fmt.Errorf("routing case with no id")
		}
		if _, duplicate := seen[routingCase.ID]; duplicate {
			return RoutingPack{}, fmt.Errorf("routing case %s declared twice", routingCase.ID)
		}
		seen[routingCase.ID] = struct{}{}
		if routingCase.Current.Content == "" {
			return RoutingPack{}, fmt.Errorf("routing case %s has no current message", routingCase.ID)
		}
		if !routingCase.checked() {
			return RoutingPack{}, fmt.Errorf("routing case %s asserts nothing", routingCase.ID)
		}
	}
	return pack, nil
}
