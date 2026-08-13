# Features: cross-repo tooling and release

This repository uses `docs/features-release-tooling.md` as the durable
cross-reference for the catalog entry points in `README.md`, `AGENTS.md`, and
`docs/FEATURES.md`.

The release and validation surfaces stay separate:

- `.pre-commit-config.yaml` pins the shared agentic-os validation suite.
- `.forgejo/workflows/ci.yml` runs the repository build, tests, and validation.
- Main pushes publish the full-source-SHA Sirens Echo image to Forgejo
  OCI only after the test job passes.
- A main push that publishes no image fails the run. The publish job is
  skipped when tests fail, cancelled when a later push supersedes it, and
  failed when the publish itself breaks. All three leave the commit without
  an image, so a terminal job reports the consequence rather than letting a
  stopped pipeline read as a flaky suite.
- An hourly `image-coverage` workflow asks the package registry whether main's
  tip has an image. A cancelled run cancels the in-run check with it, so the
  cancellation case is only visible from outside any push's run.
- `coilyco-bridge/deploy` owns the read-only pull credential, k3s rollout, and
  rollback.

## See also

- [README.md](../README.md) - human-facing introduction.
- [AGENTS.md](../AGENTS.md) - agent-facing rules.
- [FEATURES.md](FEATURES.md) - shipped capability inventory.
- [.ward/ward.yaml](../.ward/ward.yaml) - allowlisted development commands.
