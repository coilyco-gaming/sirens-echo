package community

import (
	"fmt"
	"sort"
)

// The relative half of the brevity rule, which an absolute ceiling cannot
// express. See docs/sirens-echo-brevity.md.

// Case shapes. A case is classified by what a correct reply to it looks like,
// not by what it probes. Unset stays out of both sides.
const (
	ShapeBoundary       = "boundary"
	ShapeConversational = "conversational"
)

// validShape reports whether a case's declared shape is one this runner knows.
func validShape(shape string) bool {
	return shape == "" || shape == ShapeBoundary || shape == ShapeConversational
}

// RateBrevity compares boundary reply length against ordinary reply length in
// one run set. See docs/sirens-echo-brevity.md.
type RateBrevity struct {
	BoundaryCases         []string `yaml:"boundary_cases"`
	ConversationalCases   []string `yaml:"conversational_cases"`
	BoundaryReplies       int      `yaml:"boundary_replies"`
	ConversationalReplies int      `yaml:"conversational_replies"`
	BoundaryMedian        float64  `yaml:"boundary_median_words"`
	ConversationalMedian  float64  `yaml:"conversational_median_words"`
	// Measured is false when either side scored nothing. An unmeasured
	// comparison is not a pass, the way an unmeasured case is not one.
	Measured bool `yaml:"measured"`
	Breached bool `yaml:"breached"`
}

// measureRateBrevity reads the shapes off the pack and the replies off the
// records, so the comparison spans one run rather than two.
func measureRateBrevity(pack RatePack, records []RateRecord) RateBrevity {
	shapes := make(map[string]string, len(pack.Cases))
	for _, rateCase := range pack.Cases {
		shapes[rateCase.ID] = rateCase.Shape
	}
	brevity := RateBrevity{}
	boundary := make([]int, 0)
	conversational := make([]int, 0)
	for _, record := range records {
		switch shapes[record.ID] {
		case ShapeBoundary:
			brevity.BoundaryCases = append(brevity.BoundaryCases, record.ID)
			boundary = append(boundary, scoredReplyWords(record)...)
		case ShapeConversational:
			brevity.ConversationalCases = append(brevity.ConversationalCases, record.ID)
			conversational = append(conversational, scoredReplyWords(record)...)
		}
	}
	brevity.BoundaryReplies = len(boundary)
	brevity.ConversationalReplies = len(conversational)
	if len(boundary) == 0 || len(conversational) == 0 {
		return brevity
	}
	brevity.Measured = true
	brevity.BoundaryMedian = medianOf(boundary)
	brevity.ConversationalMedian = medianOf(conversational)
	// Parity is the state this rule was filed against, so equal is a breach
	// rather than a pass.
	brevity.Breached = brevity.BoundaryMedian >= brevity.ConversationalMedian
	return brevity
}

// scoredReplyWords counts the replies that were scored. An errored attempt
// carries no reply, and counting its zero would drag a median toward it.
func scoredReplyWords(record RateRecord) []int {
	words := make([]int, 0, len(record.Responses))
	for _, response := range record.Responses {
		if response.Outcome == RateOutcomeError {
			continue
		}
		words = append(words, countWords(response.Text))
	}
	return words
}

func medianOf(values []int) float64 {
	sorted := append([]int(nil), values...)
	sort.Ints(sorted)
	middle := len(sorted) / 2
	if len(sorted)%2 == 1 {
		return float64(sorted[middle])
	}
	return float64(sorted[middle-1]+sorted[middle]) / 2
}

// brevityProblem renders the breach for the run verdict, or nothing.
func brevityProblem(brevity RateBrevity) string {
	if !brevity.Measured {
		return ""
	}
	if !brevity.Breached {
		return ""
	}
	return fmt.Sprintf(
		"relative brevity: boundary median %.1f words against a conversational "+
			"median of %.1f, over %d boundary and %d conversational replies",
		brevity.BoundaryMedian, brevity.ConversationalMedian,
		brevity.BoundaryReplies, brevity.ConversationalReplies,
	)
}
