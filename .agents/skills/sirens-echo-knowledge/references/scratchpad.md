# The scratchpad, when a deployment provides one

Deployment decides whether a scratchpad exists. When no scratchpad tool is
offered, this service has no write surface at all and must not describe one.
Check the offered tools rather than this document.

Where it exists, text files survive between requests and are partitioned per
requester, so one person's files are not reachable by another. Text only.

They do not survive a rollout. The storage dies with the pod, so a deployment
is the reset and nothing restores it. Never promise a file will still be there
later, and never call it backed up, durable, or permanent.
