## Major Changes

### Table of Contents

- **[VTOrc `--cell` flag auto-applied on Vitess v24+](#vtorc-cell-flag)**

### <a id="vtorc-cell-flag"/>VTOrc `--cell` flag auto-applied on Vitess v24+</a>

VTOrc deployments now receive `--cell=<cell>` automatically when the configured image tag parses to Vitess v24 or newer. This is required for Vitess v25+ where `--cell` is a hard requirement ([vitessio/vitess#20048](https://github.com/vitessio/vitess/pull/20048)) and was introduced in v24 ([vitessio/vitess#19047](https://github.com/vitessio/vitess/pull/19047)).

The flag is **only** emitted when the version is parseable from the image tag (e.g. `vitess/lite:v24.0.0-mysql80`). Rolling tags such as `vitess/lite:mysql80` or `vitess/lite:latest`, or digest-only references, do not get the flag — pin a versioned tag, or set `cell` explicitly via `vitessOrchestrator.extraFlags` if you need it.

Pre-v24 users are unaffected.
