package systemone

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
)

// TypeSafe keys questions and answers by name, and a list is a 422. The Go
// types stay lists; only the wire form is keyed. sirens-echo#8161.

type wireQuestion struct {
	Type         QuestionType `json:"type"`
	Instructions string       `json:"instructions"`
	// A choice's criteria is name -> description; a score's is its ordered
	// level labels.
	Criteria any `json:"criteria,omitempty"`
}

type wireRequest struct {
	Model     string                  `json:"model"`
	State     map[string]any          `json:"state"`
	Questions map[string]wireQuestion `json:"questions"`
}

type wireAnswer struct {
	Type          QuestionType       `json:"type"`
	Noul          *float64           `json:"noul,omitempty"`
	Choice        string             `json:"choice,omitempty"`
	Probabilities map[string]float64 `json:"probabilities,omitempty"`
	Confidence    float64            `json:"confidence,omitempty"`
}

type wireResponse struct {
	Model   string                `json:"model"`
	Answers map[string]wireAnswer `json:"answers"`
	Usage   Usage                 `json:"usage"`
}

// scoreLabels are the level names a score question sends, "1" through levels.
func scoreLabels(levels int) []string {
	labels := make([]string, levels)
	for i := range labels {
		labels[i] = strconv.Itoa(i + 1)
	}
	return labels
}

func (r Request) MarshalJSON() ([]byte, error) {
	wire := wireRequest{Model: r.Model, State: r.State, Questions: make(map[string]wireQuestion, len(r.Questions))}
	for _, q := range r.Questions {
		if _, dup := wire.Questions[q.Key]; dup {
			return nil, fmt.Errorf("duplicate question key %q", q.Key)
		}
		entry := wireQuestion{Type: q.Type, Instructions: q.Prompt}
		switch q.Type {
		case TypeChoice:
			criteria := make(map[string]string, len(q.Criteria))
			for _, c := range q.Criteria {
				criteria[c.Name] = c.Description
			}
			entry.Criteria = criteria
		case TypeScore:
			entry.Criteria = scoreLabels(q.Levels)
		}
		wire.Questions[q.Key] = entry
	}
	return json.Marshal(wire)
}

func (r *Request) UnmarshalJSON(raw []byte) error {
	var wire struct {
		Model     string                     `json:"model"`
		State     map[string]any             `json:"state"`
		Questions map[string]json.RawMessage `json:"questions"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		return err
	}
	r.Model, r.State, r.Questions = wire.Model, wire.State, nil
	for _, key := range sortedKeys(wire.Questions) {
		var entry struct {
			Type         QuestionType    `json:"type"`
			Instructions string          `json:"instructions"`
			Criteria     json.RawMessage `json:"criteria"`
		}
		if err := json.Unmarshal(wire.Questions[key], &entry); err != nil {
			return err
		}
		q := Question{Key: key, Type: entry.Type, Prompt: entry.Instructions}
		switch entry.Type {
		case TypeChoice:
			var criteria map[string]string
			if err := json.Unmarshal(entry.Criteria, &criteria); err != nil {
				return err
			}
			for _, name := range sortedKeys(criteria) {
				q.Criteria = append(q.Criteria, Criterion{Name: name, Description: criteria[name]})
			}
		case TypeScore:
			var labels []string
			if err := json.Unmarshal(entry.Criteria, &labels); err != nil {
				return err
			}
			q.Levels = len(labels)
		}
		r.Questions = append(r.Questions, q)
	}
	return nil
}

func (r Response) MarshalJSON() ([]byte, error) {
	wire := wireResponse{Model: r.Model, Usage: r.Usage, Answers: make(map[string]wireAnswer, len(r.Answers))}
	for _, a := range r.Answers {
		entry := wireAnswer{}
		switch {
		case a.Level > 0:
			entry.Type = TypeScore
			entry.Probabilities = map[string]float64{strconv.Itoa(a.Level - 1): a.Probability}
		case a.Option != "":
			entry.Type = TypeChoice
			entry.Choice = a.Option
			entry.Probabilities = map[string]float64{a.Option: a.Probability}
		default:
			entry.Type = TypeNoul
			p := a.Probability
			entry.Noul = &p
		}
		wire.Answers[a.Key] = entry
	}
	return json.Marshal(wire)
}

func (r *Response) UnmarshalJSON(raw []byte) error {
	var wire wireResponse
	if err := json.Unmarshal(raw, &wire); err != nil {
		return err
	}
	r.Model, r.Usage, r.Answers = wire.Model, wire.Usage, nil
	for _, key := range sortedKeys(wire.Answers) {
		entry := wire.Answers[key]
		answer := Answer{Key: key}
		switch entry.Type {
		case TypeNoul:
			if entry.Noul != nil {
				answer.Probability = *entry.Noul
			}
		case TypeChoice:
			answer.Option = entry.Choice
			answer.Probability = entry.Probabilities[entry.Choice]
		case TypeScore:
			// Probabilities are keyed by level index; the level is the likeliest.
			best, bestP := -1, -1.0
			for index, p := range entry.Probabilities {
				i, err := strconv.Atoi(index)
				if err != nil {
					continue
				}
				if p > bestP || (p == bestP && i < best) {
					best, bestP = i, p
				}
			}
			if best >= 0 {
				answer.Level, answer.Probability = best+1, bestP
			}
		}
		r.Answers = append(r.Answers, answer)
	}
	return nil
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
