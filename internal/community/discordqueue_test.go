package community

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
)

func queuedMessage(id, authorID string) *discordgo.MessageCreate {
	return &discordgo.MessageCreate{Message: &discordgo.Message{
		ID: id, ChannelID: "channel-1", GuildID: "guild-1", Content: "hello",
		Author: &discordgo.User{ID: authorID},
	}}
}

// The whole point of the split: every session sees every event, and the key
// makes two intakes and a still-connected worker produce one row.
func TestThreeSessionsOfferingOneMessageProduceOneEvent(t *testing.T) {
	t.Parallel()
	queue := NewMemoryDiscordEventQueue()
	for range 3 {
		offerer := &discordOfferer{queue: queue}
		offerer.onMessage(editSession("bot-1"), queuedMessage("1100", "member-1"))
	}
	events, err := queue.Claim(context.Background(), "worker", 10)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if len(events) != 1 || events[0].Key != "discord:msg:1100" || events[0].Kind != discordEventMessage {
		t.Fatalf("events = %+v, want one message event", events)
	}
}

func TestTheOffererSkipsOwnPostsAndLinkPreviews(t *testing.T) {
	t.Parallel()
	queue := NewMemoryDiscordEventQueue()
	offerer := &discordOfferer{queue: queue}
	session := editSession("bot-1")
	offerer.onMessage(session, queuedMessage("1101", "bot-1"))
	// A link preview is an update with no edited timestamp.
	offerer.onMessageEdit(session, editEvent(&discordgo.Message{
		ID: "1102", ChannelID: "channel-1", GuildID: "guild-1",
		Mentions: []*discordgo.User{{ID: "bot-1"}},
	}))
	events, _ := queue.Claim(context.Background(), "worker", 10)
	if len(events) != 0 {
		t.Fatalf("own posts and previews should not be offered, got %+v", events)
	}
	edited := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	offerer.onMessageEdit(session, editEvent(&discordgo.Message{
		ID: "1102", ChannelID: "channel-1", GuildID: "guild-1", EditedTimestamp: &edited,
		Author: &discordgo.User{ID: "member-1"}, Mentions: []*discordgo.User{{ID: "bot-1"}},
	}))
	events, _ = queue.Claim(context.Background(), "worker", 10)
	if len(events) != 1 || events[0].Key != "discord:edit:1102:2026-09-25T12:00:00Z" {
		t.Fatalf("an edit that newly mentions the service should be offered once, got %+v", events)
	}
}

// An interaction crosses the queue as JSON, and the worker must read back the
// command it names or it cannot answer it.
func TestAnInteractionSurvivesTheQueue(t *testing.T) {
	t.Parallel()
	queue := NewMemoryDiscordEventQueue()
	offerer := &discordOfferer{queue: queue}
	offerer.onInteraction(nil, &discordgo.InteractionCreate{Interaction: &discordgo.Interaction{
		ID: "1200", Type: discordgo.InteractionApplicationCommand, Token: "token",
		GuildID: "guild-1", ChannelID: "channel-1",
		Data: discordgo.ApplicationCommandInteractionData{ID: "cmd-1", Name: "job-status"},
	}})
	var got *discordgo.InteractionCreate
	handlers := discordEventHandlers{interaction: func(event *discordgo.InteractionCreate) { got = event }}
	claimed := drainDiscordEvents(context.Background(), queue, "worker", handlers, telemetryOrNoop(nil), time.Now)
	if claimed != 1 || got == nil {
		t.Fatalf("claimed %d, dispatched %v", claimed, got)
	}
	if got.ID != "1200" || got.Token != "token" || got.ApplicationCommandData().Name != "job-status" {
		t.Fatalf("interaction did not round-trip: %+v", got.Interaction)
	}
}

// Discord refuses an answer after three seconds, and a message from a long
// outage would land far below its conversation, so both are dropped.
func TestStaleEventsAreDroppedRatherThanAnswered(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	message, _ := json.Marshal(discordgo.Message{ID: "1300"})
	interaction, _ := json.Marshal(discordgo.Interaction{ID: "1301", Type: discordgo.InteractionApplicationCommand})
	var dispatched []string
	handlers := discordEventHandlers{
		message:     func(m *discordgo.Message) { dispatched = append(dispatched, m.ID) },
		interaction: func(i *discordgo.InteractionCreate) { dispatched = append(dispatched, i.ID) },
	}
	cases := []DiscordEvent{
		{Key: "a", Kind: discordEventMessage, Payload: message, ReceivedAt: now.Add(-time.Minute)},
		{Key: "b", Kind: discordEventMessage, Payload: message, ReceivedAt: now.Add(-discordEventMaxAge - time.Second)},
		{Key: "c", Kind: discordEventInteraction, Payload: interaction, ReceivedAt: now.Add(-time.Second)},
		{Key: "d", Kind: discordEventInteraction, Payload: interaction, ReceivedAt: now.Add(-4 * time.Second)},
	}
	for _, event := range cases {
		dispatchDiscordEvent(context.Background(), event, handlers, telemetryOrNoop(nil), now)
	}
	if strings.Join(dispatched, ",") != "1300,1301" {
		t.Fatalf("dispatched %v, want only the fresh message and the fresh interaction", dispatched)
	}
}

func TestAClaimedEventIsNeverHandedOutAgain(t *testing.T) {
	t.Parallel()
	queue := NewMemoryDiscordEventQueue()
	base := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	for i, key := range []string{"k2", "k1", "k3"} {
		_, _ = queue.Offer(context.Background(), DiscordEvent{
			Key: key, Kind: discordEventMessage, Payload: []byte(`{}`), ReceivedAt: base.Add(time.Duration(i) * time.Second),
		})
	}
	first, _ := queue.Claim(context.Background(), "w", 2)
	second, _ := queue.Claim(context.Background(), "w", 2)
	third, _ := queue.Claim(context.Background(), "w", 2)
	if len(first) != 2 || first[0].Key != "k2" || first[1].Key != "k1" || len(second) != 1 || len(third) != 0 {
		t.Fatalf("claims = %v / %v / %v, want arrival order and each once", first, second, third)
	}
	// A late duplicate from a second intake collides with the claimed row.
	if inserted, _ := queue.Offer(context.Background(), DiscordEvent{
		Key: "k1", Kind: discordEventMessage, Payload: []byte(`{}`), ReceivedAt: base,
	}); inserted {
		t.Fatal("a claimed event was offered again as new")
	}
	if swept, _ := queue.Sweep(context.Background(), base.Add(90*time.Second)); swept != 3 {
		t.Fatalf("swept %d, want 3", swept)
	}
}

// A worker that never opens the gateway has an empty state cache, which
// ownership and mention checks read. One REST read per channel fills it.
func TestAClosedGatewayFillsItsStateOncePerChannel(t *testing.T) {
	var calls atomic.Int32
	session, err := discordgo.New("Bot test")
	if err != nil {
		t.Fatalf("session: %v", err)
	}
	session.Client = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		calls.Add(1)
		body, status := `{"message":"Unknown Channel","code":10003}`, http.StatusNotFound
		switch {
		case strings.HasSuffix(request.URL.Path, "/guilds/guild-1"):
			body, status = `{"id":"guild-1","name":"g"}`, http.StatusOK
		case strings.HasSuffix(request.URL.Path, "/channels/thread-1"):
			body, status = `{"id":"thread-1","guild_id":"guild-1","type":11,"owner_id":"bot-1"}`, http.StatusOK
		}
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)),
			Header: http.Header{"Content-Type": {"application/json"}}, Request: request}, nil
	})}
	session.State.User = &discordgo.User{ID: "bot-1"}
	agent := &Agent{session: session}
	agent.hydrateDiscordState("guild-1", "thread-1")
	agent.hydrateDiscordState("guild-1", "thread-1")
	if got := calls.Load(); got != 2 {
		t.Fatalf("REST calls = %d, want one for the guild and one for the channel", got)
	}
	if !threadOwnedBy(session, "thread-1", "bot-1") {
		t.Fatal("a hydrated thread should report its owner")
	}
	agent.hydrateDiscordState("guild-1", "gone")
	agent.hydrateDiscordState("guild-1", "gone")
	if got := calls.Load(); got != 3 {
		t.Fatalf("REST calls = %d, a failed read should wait before it is retried", got)
	}
	// A connected session's state is the gateway's, and it has no HTTP client
	// here, so a REST read would panic.
	connected := &Agent{session: editSession("bot-1"), cfg: Config{DiscordGateway: true}}
	connected.hydrateDiscordState("guild-1", "thread-1")
}

func TestAClosedGatewayWithNoQueueIsRefused(t *testing.T) {
	path := filepath.Join("..", "..", "agents", "deep", "definition.yaml")
	t.Setenv("SIRENS_ECHO_DEFINITION", path)
	useFixtureBundles(t, "creator")
	t.Setenv("SIRENS_ECHO_STEAM_MCP_URL", "http://sirens-deep-steam-mcp:9112/mcp")
	t.Setenv("SIRENS_ECHO_FORGEJO_MCP_URL", "http://sirens-deep-forgejo-mcp:8080/mcp")
	t.Setenv("SIRENS_ECHO_INSTANCE", "sirens-deep")
	t.Setenv("DISCORD_TOKEN", "discord-token")
	t.Setenv("DISCORD_CHANNEL_ID", "1024000000000000001")
	t.Setenv("AGENT_PROXY_MODEL", "model")
	t.Setenv("SIRENS_ECHO_DISCORD_GATEWAY", "false")
	if _, err := LoadConfig(); err == nil || !strings.Contains(err.Error(), "SIRENS_ECHO_DISCORD_QUEUE") {
		t.Fatalf("a closed gateway with no queue hears nothing, got %v", err)
	}
	t.Setenv("SIRENS_ECHO_DISCORD_QUEUE", "true")
	if _, err := LoadConfig(); err == nil || !strings.Contains(err.Error(), "SIRENS_ECHO_JOB_STORE_DSN") {
		t.Fatalf("the queue lives in the job store database, got %v", err)
	}
}

func TestIntakeIsNotReadyUntilTheGatewayIdentifies(t *testing.T) {
	t.Parallel()
	intake := &Intake{telemetry: telemetryOrNoop(nil)}
	recorder := httptest.NewRecorder()
	intake.HTTPHandler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, readyzPath, nil))
	if recorder.Code != http.StatusServiceUnavailable || !strings.Contains(recorder.Body.String(), "gateway_not_ready") {
		t.Fatalf("readyz = %d %s, want 503 until READY", recorder.Code, recorder.Body.String())
	}
	recorder = httptest.NewRecorder()
	intake.HTTPHandler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, healthzPath, nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("healthz = %d, want 200 regardless of the gateway", recorder.Code)
	}
}

func TestLoadIntakeConfigNeedsTheTokenAndTheQueue(t *testing.T) {
	t.Setenv("DISCORD_TOKEN", "")
	t.Setenv("SIRENS_ECHO_JOB_STORE_DSN", "")
	if _, err := LoadIntakeConfig(); err == nil ||
		!strings.Contains(err.Error(), "DISCORD_TOKEN") || !strings.Contains(err.Error(), "SIRENS_ECHO_JOB_STORE_DSN") {
		t.Fatalf("the intake needs both, got %v", err)
	}
	t.Setenv("DISCORD_TOKEN", "discord-token")
	t.Setenv("SIRENS_ECHO_JOB_STORE_DSN", "postgres://sirens@db/sirens_jobs")
	t.Setenv("SIRENS_ECHO_DISCORD_DM_ENABLED", "true")
	cfg, err := LoadIntakeConfig()
	if err != nil {
		t.Fatalf("LoadIntakeConfig: %v", err)
	}
	if discordIntents(cfg)&discordgo.IntentsDirectMessages == 0 || cfg.InstanceName != "sirens-echo-intake" {
		t.Fatalf("cfg = %+v, want direct messages and the intake's own service name", cfg)
	}
}

// Against a real database: two intakes, one row, and two workers claiming at
// once never share an event. Skipped without SIRENS_ECHO_TEST_JOB_STORE_DSN.
func TestThePostgresDiscordQueueClaimsEachEventOnce(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("SIRENS_ECHO_TEST_JOB_STORE_DSN"))
	if dsn == "" {
		t.Skip("SIRENS_ECHO_TEST_JOB_STORE_DSN is unset")
	}
	ctx := context.Background()
	queue, err := OpenPostgresDiscordEventQueue(ctx, dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer queue.Close()
	prefix := "test:" + strconv.FormatInt(time.Now().UnixNano(), 36) + ":"
	now := time.Now().UTC()
	t.Cleanup(func() { _, _ = queue.pool.Exec(ctx, `DELETE FROM discord_events WHERE key LIKE $1`, prefix+"%") })
	for i := range 40 {
		key := prefix + string(rune('a'+i%26)) + string(rune('0'+i/26))
		for range 2 {
			_, _ = queue.Offer(ctx, DiscordEvent{Key: key, Kind: discordEventMessage, Payload: []byte(`{}`), ReceivedAt: now})
		}
	}
	seen := map[string]int{}
	var mu sync.Mutex
	var wait sync.WaitGroup
	for worker := range 2 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			for {
				events, err := queue.Claim(ctx, "worker-"+string(rune('a'+worker)), 3)
				if err != nil || len(events) == 0 {
					return
				}
				mu.Lock()
				for _, event := range events {
					if strings.HasPrefix(event.Key, prefix) {
						seen[event.Key]++
					}
				}
				mu.Unlock()
			}
		}()
	}
	wait.Wait()
	if len(seen) != 40 {
		t.Fatalf("claimed %d distinct events, want 40", len(seen))
	}
	for key, count := range seen {
		if count != 1 {
			t.Fatalf("%s was claimed %d times", key, count)
		}
	}
}

// Readiness follows the gateway both ways, so a rolling update never counts a
// reconnecting intake as serving, and a resume is visible in the log.
func TestIntakeReadinessFollowsDisconnectAndResume(t *testing.T) {
	t.Parallel()
	intake := &Intake{telemetry: telemetryOrNoop(nil)}
	status := func() int {
		recorder := httptest.NewRecorder()
		intake.HTTPHandler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, readyzPath, nil))
		return recorder.Code
	}
	intake.onReady(nil, &discordgo.Ready{SessionID: "s"})
	if status() != http.StatusServiceUnavailable {
		t.Fatal("with no queue open the intake is not ready, whatever the gateway says")
	}
	intake.onDisconnect(nil, &discordgo.Disconnect{})
	if intake.ready.Load() || intake.disconnectedAt.Load() == 0 {
		t.Fatal("a disconnect clears readiness and records when")
	}
	intake.onResumed(nil, &discordgo.Resumed{})
	if !intake.ready.Load() {
		t.Fatal("a resume restores readiness")
	}
}
