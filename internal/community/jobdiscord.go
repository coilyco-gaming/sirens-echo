package community

import (
	"context"
	"fmt"

	"github.com/bwmarrin/discordgo"
)

// Discord is the one origin with a durable place to answer in, so it is the
// one transport that gets notified. See docs/sirens-echo-jobs-lifecycle.md.

// discordJobReporter delivers job notices back to the channel a job came from.
type discordJobReporter struct {
	session *discordgo.Session
}

// NotifyJob announces a terminal state. The phrase comes from the closed
// notice vocabulary, so no upstream string reaches a member.
func (r discordJobReporter) NotifyJob(ctx context.Context, job Job) error {
	return r.send(ctx, job, jobTerminalNotice(job))
}

// ReportJobProgress delivers an intermediate update, already rendered.
func (r discordJobReporter) ReportJobProgress(ctx context.Context, job Job, notice string) error {
	return r.send(ctx, job, notice)
}

func (r discordJobReporter) send(_ context.Context, job Job, notice string) error {
	if r.session == nil {
		return fmt.Errorf("no Discord session")
	}
	channel := job.Origin.ChannelID
	if job.Origin.ThreadID != "" {
		channel = job.Origin.ThreadID
	}
	// Mention safety: a job update must never notify anyone, which matters more
	// here than on a reply because a job speaks without being asked again.
	_, err := r.session.ChannelMessageSendComplex(channel, &discordgo.MessageSend{
		Content: truncateRunes(notice, 1990),
		AllowedMentions: &discordgo.MessageAllowedMentions{
			Parse:       []discordgo.AllowedMentionType{},
			RepliedUser: false,
		},
	})
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
