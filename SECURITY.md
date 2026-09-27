# Security Policy

## Supported versions

Blueprint is pre-1.0 and versions each module independently (the core module and every
`provider/*` module). Security fixes are released for the **latest minor version of each module**
only; please upgrade to the newest core and provider releases before reporting.

| Module | Supported |
|--------|-----------|
| `github.com/oddbit-project/blueprint` — latest `v0.x` release | yes |
| `github.com/oddbit-project/blueprint/provider/*` — latest release of each provider | yes |
| older releases | no |

## Reporting a vulnerability

**Do not open a public issue, discussion or pull request for a security problem.**

Report it privately through GitHub:
[Report a vulnerability](https://github.com/oddbit-project/blueprint/security/advisories/new)
(the **Security** tab → **Report a vulnerability**).

Please include:

- the affected module(s) and version(s) or commit;
- the database, broker or service involved and its driver/client version, if relevant;
- a minimal reproduction (code, configuration, request) and what goes wrong;
- any known workaround.

We will acknowledge the report, work on a fix in a private advisory, and publish the advisory
together with the patched release(s). Reporters are credited in the advisory unless they ask not
to be.

## Notes on scope

- Credentials that appear in test files and testcontainer setups are throwaway test values for
  local containers, not secrets.
- The SQL query builder used by `dbx` is being moved to its own module,
  [`github.com/oddbit-project/gohan`](https://github.com/oddbit-project/gohan); report builder
  issues there once it is released.
- Issues in third-party dependencies should be reported upstream; a heads-up here is welcome if
  Blueprint needs to react (pin, patch or work around).
