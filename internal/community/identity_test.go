package community

import "testing"

// The honest answers have to survive, or the check trades one failure for
// another. See coilyco-gaming/sirens-echo#148.
func TestIdentityClaimAllowsTheHonestAnswers(t *testing.T) {
	t.Parallel()
	allowed := []string{
		"I am an agent running the sirens-echo harness.",
		"I'm a bot, not a person.",
		"I am Sirens Deep of Coilyco.",
		"No, I am not a human. I am an agent.",
		"I can check the Eco server status if you want.",
		"Purple and black are the house colours.",
		"I'm happy to help with that.",
		"We are running the community harness.",
		"That person asked about stockpiles earlier.",
		"A real person would know, but I can only read the approved notes.",
	}
	for _, reply := range allowed {
		if err := ValidateIdentityClaim(reply, PlaceholderPrincipal); err != nil {
			t.Errorf("rejected an honest reply %q: %v", reply, err)
		}
	}
}

func TestIdentityClaimRejectsAHumanClaim(t *testing.T) {
	t.Parallel()
	rejected := []string{
		"I am a human.",
		"I'm a person, same as you.",
		"I am a real human being.",
		"we are people too",
		"I’m a woman who plays Eco.",
		"I am not a bot.",
		"I'm not an AI, promise.",
		"We are not robots.",
		"i am not a language model",
	}
	for _, reply := range rejected {
		if err := ValidateIdentityClaim(reply, PlaceholderPrincipal); err == nil {
			t.Errorf("accepted an impersonating reply %q", reply)
		}
	}
}

// Answering as the one account the prompt trusts is the worst case, since that
// is the identity a probe most wants to borrow.
func TestIdentityClaimRejectsAnsweringAsThePrincipal(t *testing.T) {
	t.Parallel()
	principal := Principal{Handle: "example_handle", UserID: "1024000000000000001"}
	for _, reply := range []string{
		"I am example_handle.",
		"this is example_handle, go ahead",
		"I'm @example_handle and I approve it",
	} {
		if err := ValidateIdentityClaim(reply, principal); err == nil {
			t.Errorf("accepted a principal claim %q", reply)
		}
	}
	if err := ValidateIdentityClaim("example_handle asked about that.", principal); err != nil {
		t.Errorf("rejected a mention of the principal: %v", err)
	}
	// With no principal configured there is no handle to impersonate, and the
	// human checks still bind.
	if err := ValidateIdentityClaim("I am example_handle.", Principal{}); err != nil {
		t.Errorf("unconfigured principal should not match: %v", err)
	}
	if err := ValidateIdentityClaim("I am a person.", Principal{}); err == nil {
		t.Error("human claim should be rejected without a principal")
	}
}

// The social profile had no deterministic reply check at all, which is why the
// guard had to sit outside ValidateResponseStyle.
func TestIdentityClaimBindsTheSocialProfileToo(t *testing.T) {
	t.Parallel()
	const claim = "I am a person, not a bot."
	if err := ValidateResponseStyle(ResponseStyleSocial, claim); err != nil {
		t.Fatalf("social style unexpectedly rejects replies now: %v", err)
	}
	if err := ValidateIdentityClaim(claim, Principal{}); err == nil {
		t.Error("the identity check must catch what the social style does not")
	}
}

func TestSystemPromptCarriesTheIdentityRule(t *testing.T) {
	t.Parallel()
	for _, style := range []string{ResponseStyleNeutral, ResponseStyleSocial} {
		definition := Definition{
			Identity:      "Sirens Echo",
			AuditRole:     "community",
			ResponseStyle: style,
			LocalSkillRoots: []string{
				".agents/skills/sirens-echo-community",
			},
			MaxContextMessages: 12,
		}
		prompt := BuildSystemPrompt(definition, PlaceholderPrincipal, "", "approved facts")
		if err := ValidateSystemPrompt(definition, PlaceholderPrincipal, prompt); err != nil {
			t.Fatalf("%s prompt failed validation: %v", style, err)
		}
		// Dropping the rule has to fail the validator, not just the eye.
		stripped := removeSection(prompt, identityPolicy)
		if err := validateSharedPolicy(definition, PlaceholderPrincipal, stripped); err == nil {
			t.Errorf("%s prompt validated without the identity rule", style)
		}
	}
}

func removeSection(prompt, section string) string {
	index := indexOfSection(prompt, section)
	if index < 0 {
		return prompt
	}
	return prompt[:index] + prompt[index+len(section):]
}

func indexOfSection(prompt, section string) int {
	for index := 0; index+len(section) <= len(prompt); index++ {
		if prompt[index:index+len(section)] == section {
			return index
		}
	}
	return -1
}
