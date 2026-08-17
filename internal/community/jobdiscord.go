package community

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/bwmarrin/discordgo"
)

// Discord is the one origin with a durable place to answer in, so it is the
// one transport that gets notified. See docs/sirens-echo-jobs.md.

// discordJobReporter delivers job notices back to the channel a job came from.
type discordJobReporter struct {
	session *discordgo.Session

	mu sync.Mutex
	// progressMessage is the one message per job that updates edit in place, so
	// a long job leaves one line rather than a column of them.
	progressMessage map[string]string
}

func newDiscordJobReporter(session *discordgo.Session) *discordJobReporter {
	return &discordJobReporter{
		session:         session,
		progressMessage: make(map[string]string),
	}
}

// NotifyJob announces a terminal state. The phrase comes from the closed
// notice vocabulary, so no upstream string reaches a member.
func (r *discordJobReporter) NotifyJob(ctx context.Context, job Job) error {
	// The terminal notice replaces the progress line when there is one, so the
	// job ends as a single message that says how it went.
	if id := r.takeProgressMessage(job.ID); id != "" {
		if err := r.edit(job, id, jobTerminalNotice(job)); err == nil {
			return nil
		}
	}
	return r.send(ctx, job, jobTerminalNotice(job))
}

// ReportJobProgress edits the job's progress line, posting one the first time.
func (r *discordJobReporter) ReportJobProgress(ctx context.Context, job Job, notice string) error {
	r.mu.Lock()
	existing := r.progressMessage[job.ID]
	r.mu.Unlock()
	if existing != "" {
		return r.edit(job, existing, notice)
	}
	sent, err := r.post(job, notice)
	if err != nil {
		return err
	}
	r.mu.Lock()
	r.progressMessage[job.ID] = sent
	r.mu.Unlock()
	return nil
}

// takeProgressMessage returns and forgets a job's progress line.
func (r *discordJobReporter) takeProgressMessage(id string) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	message := r.progressMessage[id]
	delete(r.progressMessage, id)
	return message
}

func (r *discordJobReporter) edit(job Job, messageID, notice string) error {
	if r.session == nil {
		return fmt.Errorf("no Discord session")
	}
	content := truncateRunes(notice, 1990)
	_, err := r.session.ChannelMessageEditComplex(&discordgo.MessageEdit{
		Channel: r.channelFor(job),
		ID:      messageID,
		Content: &content,
		AllowedMentions: &discordgo.MessageAllowedMentions{
			Parse: []discordgo.AllowedMentionType{},
		},
	})
	return err
}

func (r *discordJobReporter) post(job Job, notice string) (string, error) {
	if r.session == nil {
		return "", fmt.Errorf("no Discord session")
	}
	sent, err := r.session.ChannelMessageSendComplex(r.channelFor(job), &discordgo.MessageSend{
		Content: truncateRunes(notice, 1990),
		AllowedMentions: &discordgo.MessageAllowedMentions{
			Parse:       []discordgo.AllowedMentionType{},
			RepliedUser: false,
		},
	})
	if err != nil {
		return "", err
	}
	return sent.ID, nil
}

// channelFor prefers a bound thread, so job chatter stays out of the channel.
func (r *discordJobReporter) channelFor(job Job) string {
	if job.Origin.ThreadID != "" {
		return job.Origin.ThreadID
	}
	return job.Origin.ChannelID
}

func (r *discordJobReporter) send(_ context.Context, job Job, notice string) error {
	if r.session == nil {
		return fmt.Errorf("no Discord session")
	}
	// Mention safety: a job update must never notify anyone, which matters more
	// here than on a reply because a job speaks without being asked again.
	_, err := r.post(job, notice)
	return err
}

// jobTerminalNotice renders the outcome in the harness format. The job id is
// included so a member can ask about this run specifically.
func jobTerminalNotice(job Job) string {
	switch job.State {
	case JobSucceeded:
		return harnessNotice(fmt.Sprintf("job %s finished", job.ID))
	case JobCancelled:
		return harnessNotice(fmt.Sprintf("job %s cancelled", job.ID))
	default:
		return harnessNotice(fmt.Sprintf("job %s failed", job.ID))
	}
}

// discordTurnProgress posts and edits one progress line in a turn's channel.
type discordTurnProgress struct {
	session *discordgo.Session
	channel string
	// message is what the line answers. Without it the line is a bare channel
	// post while the reply that replaces it is a reply. See sirens-echo#376.
	message *discordgo.Message
}

func (p discordTurnProgress) Post(_ context.Context, notice string) (string, error) {
	if p.session == nil {
		return "", fmt.Errorf("no Discord session")
	}
	sent, err := p.session.ChannelMessageSendComplex(p.channel, &discordgo.MessageSend{
		Content:   truncateRunes(notice, 1990),
		Reference: p.reference(),
		// RepliedUser false, so answering a member does not also ping them.
		AllowedMentions: &discordgo.MessageAllowedMentions{
			Parse:       []discordgo.AllowedMentionType{},
			RepliedUser: false,
		},
	})
	if err != nil {
		return "", err
	}
	return sent.ID, nil
}

// reference is nil when the sink has no message, which keeps a bare-channel
// caller working rather than failing to post at all.
func (p discordTurnProgress) reference() *discordgo.MessageReference {
	if p.message == nil {
		return nil
	}
	return p.message.SoftReference()
}

func (p discordTurnProgress) Edit(_ context.Context, messageID, notice string) error {
	if p.session == nil {
		return fmt.Errorf("no Discord session")
	}
	content := truncateRunes(notice, 1990)
	_, err := p.session.ChannelMessageEditComplex(&discordgo.MessageEdit{
		Channel:         p.channel,
		ID:              messageID,
		Content:         &content,
		AllowedMentions: &discordgo.MessageAllowedMentions{Parse: []discordgo.AllowedMentionType{}},
	})
	return err
}

func (p discordTurnProgress) Delete(_ context.Context, messageID string) error {
	if p.session == nil {
		return fmt.Errorf("no Discord session")
	}
	return p.session.ChannelMessageDelete(p.channel, messageID)
}

// WorklogAllowed reports the embed permission, which was missing from every
// install link this estate published. See docs/sirens-echo-worklog.md.
func (p discordTurnProgress) WorklogAllowed(context.Context) bool {
	if p.session == nil || p.session.State == nil || p.session.State.User == nil {
		return false
	}
	permissions, err := p.session.State.UserChannelPermissions(
		p.session.State.User.ID, p.channel)
	// Unknown rather than absent, which a direct message always is. A wrong yes
	// is caught by the refusal below; a wrong no degrades the turn for nothing.
	if err != nil {
		return true
	}
	return permissions&discordgo.PermissionEmbedLinks != 0
}

func (p discordTurnProgress) PostWorklog(
	_ context.Context, view progressView,
) (string, error) {
	if p.session == nil {
		return "", fmt.Errorf("no Discord session")
	}
	sent, err := p.session.ChannelMessageSendComplex(p.channel, &discordgo.MessageSend{
		Embeds:    []*discordgo.MessageEmbed{worklogEmbed(view)},
		Reference: p.reference(),
		AllowedMentions: &discordgo.MessageAllowedMentions{
			Parse:       []discordgo.AllowedMentionType{},
			RepliedUser: false,
		},
	})
	if err != nil {
		return "", err
	}
	return sent.ID, nil
}

func (p discordTurnProgress) EditWorklog(
	_ context.Context, messageID string, view progressView,
) error {
	if p.session == nil {
		return fmt.Errorf("no Discord session")
	}
	_, err := p.session.ChannelMessageEditComplex(&discordgo.MessageEdit{
		Channel:         p.channel,
		ID:              messageID,
		Embeds:          &[]*discordgo.MessageEmbed{worklogEmbed(view)},
		AllowedMentions: &discordgo.MessageAllowedMentions{Parse: []discordgo.AllowedMentionType{}},
	})
	return err
}

// WorklogRefused separates a channel that will never take an embed from a call
// that happened to fail, so one timeout does not degrade the whole turn.
func (p discordTurnProgress) WorklogRefused(err error) bool {
	var rest *discordgo.RESTError
	if !errors.As(err, &rest) || rest.Message == nil {
		return false
	}
	return rest.Message.Code == discordgo.ErrCodeMissingPermissions ||
		rest.Message.Code == discordgo.ErrCodeMissingAccess
}

// worklogEmbed renders the view. The rows are already notice-shaped, so the
// embed is a container around the text contract. See sirens-echo#111.
func worklogEmbed(view progressView) *discordgo.MessageEmbed {
	return &discordgo.MessageEmbed{
		Title:       view.Title,
		Description: strings.Join(view.Rows, "\n"),
		Footer:      &discordgo.MessageEmbedFooter{Text: worklogFooter(view)},
	}
}
