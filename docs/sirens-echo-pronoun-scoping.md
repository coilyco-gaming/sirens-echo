# Pronoun scoping

How `pronoun_policy` decides which sentences are about the named subject. See
[the battery](sirens-echo-battery.md) for where it sits among the checks.

The check walks sentences. It activates at the first sentence naming the
subject, stays active through following sentences so a pronoun in the next
sentence still counts, and deactivates at any sentence naming a `stop_at`
subject. Inside an active sentence, any pronoun in `forbid` is a finding.

`forbid` is explicit rather than inferred from an allow list, because a reply
can correctly use a pronoun for someone else in the same sentence.

Two accepted gaps. The check cannot tell a referential pronoun from a quoted
one, so "I will not guess he or she" fires and a case must not invite the
subject to enumerate pronouns. A pronoun in a sentence that never names the
subject and follows no active one is missed. Tightening either trades a false
negative for false positives, which is the worse failure here. A test pins both
so a future change is deliberate.
