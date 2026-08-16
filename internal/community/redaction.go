package community

import (
	"regexp"
	"strings"
)

// Removing the block a check refused, rather than the message around it. See
// docs/sirens-echo-reply-redaction.md.

// noticeRedacted marks where a block was removed. It renders in the harness
// notice shape, which model prose cannot forge. See docs/sirens-echo-notices.md.
var noticeRedacted = harnessNotice("content removed by a response check")

// redactableRules are the refusals one block can carry by itself. An allowlist
// rather than a default. See docs/sirens-echo-reply-redaction.md.
var redactableRules = map[string]struct{}{
	replyCheckInventedChannel: {},
	replyCheckClaimedAction:   {},
	replyCheckTrackerAction:   {},
	replyCheckContinuingWork:  {},
	replyCheckToolCallMarkup:  {},
	replyCheckSelfAttributed:  {},
}

// listItem begins a block with no blank line before it, which is the shape the
// reply in sirens-echo#796 had: one bullet per server, twelve of them.
var listItem = regexp.MustCompile(`^\s*(?:[-*+]|\d+[.)])\s+`)

// replyBlock is one independently removable piece of a reply, carrying the
// separator that preceded it so the remainder rejoins in the original shape.
type replyBlock struct {
	gap  string
	text string
}

// splitReplyBlocks cuts a reply at blank lines and at list items, because a
// bulleted answer has no blank lines to cut at.
func splitReplyBlocks(reply string) []replyBlock {
	blocks := make([]replyBlock, 0, 8)
	current := make([]string, 0, 4)
	blanks := 0
	pendingGap := ""
	flush := func() {
		if len(current) == 0 {
			return
		}
		blocks = append(blocks, replyBlock{gap: pendingGap, text: strings.Join(current, "\n")})
		current = current[:0]
	}
	for _, line := range strings.Split(reply, "\n") {
		if strings.TrimSpace(line) == "" {
			flush()
			blanks++
			continue
		}
		if listItem.MatchString(line) {
			flush()
		}
		if len(current) == 0 {
			pendingGap = ""
			if len(blocks) > 0 {
				pendingGap = strings.Repeat("\n", blanks+1)
			}
			blanks = 0
		}
		current = append(current, line)
	}
	flush()
	return blocks
}

// keptText is the model's surviving prose with no notice in it, which is what
// the checks have to pass. Service-authored text is never put to them.
func keptText(blocks []replyBlock, keep []bool) string {
	var out strings.Builder
	first := true
	for index, block := range blocks {
		if !keep[index] {
			continue
		}
		if !first {
			out.WriteString(block.gap)
		}
		out.WriteString(block.text)
		first = false
	}
	return out.String()
}

// renderRedacted puts a notice where each removal was, so the gap is visible
// in place rather than the message silently reading as complete.
func renderRedacted(blocks []replyBlock, keep []bool) string {
	var out strings.Builder
	marked := false
	for index, block := range blocks {
		if keep[index] {
			out.WriteString(block.gap)
			out.WriteString(block.text)
			marked = false
			continue
		}
		// Adjacent removals share one mark. Two identical notices in a row
		// describe the same hole twice.
		if marked {
			continue
		}
		out.WriteString(block.gap)
		out.WriteString(noticeRedacted)
		marked = true
	}
	return out.String()
}

// redactRefusedBlocks removes the blocks carrying the refusal and returns the
// remainder, or reports that the reply has to be refused whole.
func (a *Agent) redactRefusedBlocks(
	reply string,
	rule string,
	prompt TurnPrompt,
	result CompletionResult,
) (string, int, bool) {
	if _, ok := redactableRules[rule]; !ok {
		return "", 0, false
	}
	blocks := splitReplyBlocks(reply)
	// One block is the whole message, so removing it saves nothing.
	if len(blocks) < 2 {
		return "", 0, false
	}
	keep := make([]bool, len(blocks))
	removed := 0
	for index, block := range blocks {
		_, blockRule, err := a.runReplyChecks(block.text, prompt, result)
		// Tied to the rule that refused the message, so a block disagreeing in
		// isolation about some other rule cannot be removed for it.
		if err != nil && blockRule == rule {
			removed++
			continue
		}
		keep[index] = true
	}
	if removed == 0 || removed > maxRedactedBlocks || removed == len(blocks) {
		return "", 0, false
	}
	kept := keptText(blocks, keep)
	if strings.TrimSpace(kept) == "" {
		return "", 0, false
	}
	// The authority. What survives has to pass every check on its own, not
	// merely lack the block that was removed.
	if _, _, err := a.runReplyChecks(kept, prompt, result); err != nil {
		return "", 0, false
	}
	return renderRedacted(blocks, keep), removed, true
}
