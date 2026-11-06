# Release: packaging, signing and updates

Remit ships **one stable channel**. 0.x minors may break things; patches are
always safe to take. Distribution is GitHub Releases through MyGo's built-in
packaging, and updates are verified independently of OS code signing.

## What a release contains

`go tool mygo build .` produces, per platform:

- macOS: `remit-<version>-<arch>.dmg` and the `.app` bundle, with the icon from
  `resources/icon.png`, version from `mygo.json`.
- Windows: an installer and an executable.
- Linux: the executable and a Debian package.

`go tool mygo build . -upload` publishes installers and update archives to a
draft GitHub release, which is the only publisher; nothing is uploaded by hand.

## Signing

| Platform | State in v1 | What a user sees without it |
|---|---|---|
| macOS | Notarized with an Apple Developer account ($99/year) when credentials are present. Without them the build is unsigned-but-working. | Gatekeeper's first-launch warning; right-click → Open works. |
| Windows | Unsigned in the first builds. SignPath Foundation (free for open source) is the intended path. | SmartScreen's "unknown publisher" warning on first run. |
| Linux | Unsigned by convention. | Nothing beyond the package manager's normal prompts. |

Rules:

- **Update verification does not depend on OS signing.** MyGo's updater checks
  an Ed25519 signature over the update archive; a missing OS signature blocks
  no update.
- Missing credentials must degrade loudly: the build reports that it is
  unsigned rather than silently shipping something that looks signed.
- The signing identity, notarization profile and update key live in CI secrets,
  never in the repository.

## Update check

- Explicit and switchable. Off means no network traffic at all.
- When on, the app asks the release channel for the latest stable version and
  offers it; it never installs without a user action.
- Delta updates are MyGo's; they are an optimisation, never a correctness
  requirement.

## Versioning

`mygo.json`'s `version` is the released version. `internal/ui.Version` mirrors
it and a test keeps them equal, so the About page cannot drift from the build.
