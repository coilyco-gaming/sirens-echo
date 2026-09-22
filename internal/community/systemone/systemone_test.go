package systemone

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestAskSendsStateAndParsesAnswers(t *testing.T) {
	var gotBody Request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/v1/systemone" {
			t.Errorf("path = %s, want /v1/systemone", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if gotBody.State == nil {
			t.Fatalf("request carried no state, want an object even when empty")
		}
		resp := Response{
			Model: "jev-latest",
			Answers: []Answer{
				{Key: "content:nsfw", Probability: 0.02},
				{Key: "shape", Option: "full", Probability: 0.91},
			},
			Usage: Usage{InputTokens: 4000, OutputTokens: 0},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := Client{BaseURL: server.URL, Model: "jev-latest"}
	resp, err := client.Ask(context.Background(), Request{
		Questions: []Question{
			{Key: "content:nsfw", Type: TypeNoul, Prompt: "is this nsfw?"},
			{
				Key: "shape", Type: TypeChoice, Prompt: "shape?",
				Criteria: []Criterion{{Name: "full", Description: "ordinary answer"}},
			},
		},
	})
	if err != nil {
		t.Fatalf("Ask: %v", err)
	}
	answers := resp.AnswerByKey()
	if got := answers["shape"].Option; got != "full" {
		t.Errorf("shape option = %q, want full", got)
	}
	if got := answers["content:nsfw"].Probability; got != 0.02 {
		t.Errorf("content:nsfw probability = %v, want 0.02", got)
	}
	if gotBody.Model != "jev-latest" {
		t.Errorf("request model = %q, want jev-latest", gotBody.Model)
	}
}

func TestAskDefaultsModelFromClient(t *testing.T) {
	var gotModel string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req Request
		json.NewDecoder(r.Body).Decode(&req)
		gotModel = req.Model
		json.NewEncoder(w).Encode(Response{})
	}))
	defer server.Close()

	client := Client{BaseURL: server.URL, Model: "jev-1.13.0"}
	if _, err := client.Ask(context.Background(), Request{Questions: []Question{{Key: "k", Type: TypeNoul}}}); err != nil {
		t.Fatalf("Ask: %v", err)
	}
	if gotModel != "jev-1.13.0" {
		t.Errorf("model = %q, want jev-1.13.0 (from Client.Model, request left it empty)", gotModel)
	}
}

func TestAskMapsProxyFailureStatuses(t *testing.T) {
	cases := []struct {
		status int
		want   FailureKind
	}{
		{http.StatusBadRequest, FailureBadRequest},
		{http.StatusNotFound, FailureModelNotAllowed},
		{http.StatusServiceUnavailable, FailureNoKeyMounted},
		{http.StatusBadGateway, FailureUpstreamUnreachable},
		{http.StatusGatewayTimeout, FailureTimeout},
		{http.StatusUnprocessableEntity, FailureUpstreamRejected},
		{http.StatusTooManyRequests, FailureUpstreamRejected},
	}
	for _, tc := range cases {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(tc.status)
			w.Write([]byte(`{"error":"synthetic"}`))
		}))
		client := Client{BaseURL: server.URL, Model: "jev-latest"}
		_, err := client.Ask(context.Background(), Request{Questions: []Question{{Key: "k", Type: TypeNoul}}})
		server.Close()
		if err == nil {
			t.Fatalf("status %d: want an error, got nil", tc.status)
		}
		sysErr, ok := err.(*Error)
		if !ok {
			t.Fatalf("status %d: error type = %T, want *Error", tc.status, err)
		}
		if sysErr.Kind != tc.want {
			t.Errorf("status %d: kind = %v, want %v", tc.status, sysErr.Kind, tc.want)
		}
	}
}

func TestAskTimesOut(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		json.NewEncoder(w).Encode(Response{})
	}))
	defer server.Close()

	client := Client{BaseURL: server.URL, Model: "jev-latest", Timeout: 5 * time.Millisecond}
	_, err := client.Ask(context.Background(), Request{Questions: []Question{{Key: "k", Type: TypeNoul}}})
	if err == nil {
		t.Fatal("want a timeout error, got nil")
	}
	sysErr, ok := err.(*Error)
	if !ok || sysErr.Kind != FailureTimeout {
		t.Errorf("err = %v, want FailureTimeout", err)
	}
}

func TestAskRequiresBaseURL(t *testing.T) {
	client := Client{Model: "jev-latest"}
	_, err := client.Ask(context.Background(), Request{})
	if err == nil {
		t.Fatal("want an error with no BaseURL configured")
	}
}
