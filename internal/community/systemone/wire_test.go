package systemone

import (
	"encoding/json"
	"testing"
)

// The request TypeSafe accepts keys questions by name. A list was a 422 in
// production (sirens-echo#8161), and no fake server caught it.
func TestRequestEncodesQuestionsAsAnObjectKeyedByName(t *testing.T) {
	t.Parallel()
	raw, err := json.Marshal(Request{
		Model: "jev-latest",
		State: map[string]any{"message": "hi"},
		Questions: []Question{
			{Key: "addressed", Type: TypeNoul, Prompt: "Is it a greeting?"},
			{Key: "shape", Type: TypeChoice, Prompt: "What shape?", Criteria: []Criterion{
				{Name: "full", Description: "words"}, {Name: "react:wave", Description: "a wave"},
			}},
			{Key: "depth", Type: TypeScore, Prompt: "How deep?", Levels: 3},
		},
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	want := map[string]any{
		"model": "jev-latest",
		"state": map[string]any{"message": "hi"},
		"questions": map[string]any{
			"addressed": map[string]any{"type": "noul", "instructions": "Is it a greeting?"},
			"shape": map[string]any{"type": "choice", "instructions": "What shape?",
				"criteria": map[string]any{"full": "words", "react:wave": "a wave"}},
			"depth": map[string]any{"type": "score", "instructions": "How deep?",
				"criteria": []any{"1", "2", "3"}},
		},
	}
	gotJSON, _ := json.Marshal(got)
	wantJSON, _ := json.Marshal(want)
	if string(gotJSON) != string(wantJSON) {
		t.Errorf("wire request =\n%s\nwant\n%s", gotJSON, wantJSON)
	}
}

// Verbatim replies from jev-1.13.0 through Agent Proxy, 2026-09-24.
const liveReply = `{"answers":{
	"addressed":{"noul":0.98,"type":"noul"},
	"shape":{"choice":"react:wave","confidence":0.63,"probabilities":{"full":0.19,"react:wave":0.81},"type":"choice"},
	"depth":{"confidence":0.99,"legend":{"0":"1","1":"2","2":"3","3":"4","4":"5"},"probabilities":{"0":0.99,"1":0.01,"2":0,"3":0,"4":0},"score":0.02,"type":"score"}
},"model":"jev-1.13.0","usage":{"input_tokens":337,"output_tokens":17}}`

func TestResponseDecodesTheLiveAnswerShapes(t *testing.T) {
	t.Parallel()
	var got Response
	if err := json.Unmarshal([]byte(liveReply), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	answers := got.AnswerByKey()
	if a := answers["addressed"]; a.Probability != 0.98 {
		t.Errorf("noul = %+v, want probability 0.98", a)
	}
	if a := answers["shape"]; a.Option != "react:wave" || a.Probability != 0.81 {
		t.Errorf("choice = %+v, want react:wave at 0.81", a)
	}
	if a := answers["depth"]; a.Level != 1 || a.Probability != 0.99 {
		t.Errorf("score = %+v, want level 1 at 0.99", a)
	}
	if got.Model != "jev-1.13.0" || got.Usage.InputTokens != 337 {
		t.Errorf("model, usage = %q, %+v", got.Model, got.Usage)
	}
}

// Fake servers in other packages decode a Request and encode a Response, so
// both directions must agree with each other as well as with TypeSafe.
func TestTheWireFormRoundTrips(t *testing.T) {
	t.Parallel()
	req := Request{Model: "m", State: map[string]any{}, Questions: []Question{
		{Key: "a", Type: TypeChoice, Prompt: "p", Criteria: []Criterion{{Name: "x", Description: "d"}}},
		{Key: "b", Type: TypeScore, Prompt: "q", Levels: 5},
	}}
	raw, _ := json.Marshal(req)
	var back Request
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatalf("decode request: %v", err)
	}
	if len(back.Questions) != 2 || back.Questions[1].Levels != 5 || back.Questions[0].Criteria[0].Name != "x" {
		t.Errorf("request round trip = %+v", back)
	}
	resp := Response{Answers: []Answer{{Key: "a", Option: "x", Probability: 0.9}, {Key: "b", Level: 3, Probability: 0.7}, {Key: "c", Probability: 0.2}}}
	raw, _ = json.Marshal(resp)
	var backResp Response
	if err := json.Unmarshal(raw, &backResp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	byKey := backResp.AnswerByKey()
	if byKey["a"].Option != "x" || byKey["b"].Level != 3 || byKey["c"].Probability != 0.2 {
		t.Errorf("response round trip = %+v", backResp)
	}
}
