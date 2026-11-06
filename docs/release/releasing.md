# Cutting a release

Remit ships on one stable channel. The workflow in `.github/workflows/release.yml`
does the work; three things are done once, by a person, before the first tag.

## Once

1. Create the update signing key pair:

   ```sh
   go tool mygo keygen     # writes mygo-update.key and mygo-update.pub
   ```

   Put the private key in the repository's secrets and delete it from the
   checkout:

   ```sh
   gh secret set MYGO_UPDATER_PRIVATE_KEY < mygo-update.key
   rm mygo-update.key
   ```

2. Add the public key and the release repository to `mygo.json`:

   ```json
   "updates": { "publicKey": "<contents of mygo-update.pub>", "github": "transmit-bug/remit" }
   ```

   Without this block `mygo build -upload` has nowhere to publish, and the
   signed update archives cannot be verified by the app.

3. Signing identities, when they exist: `macos.signingIdentity` and
   `macos.notarize` follow [signing](signing.md). Their absence only changes
   what a first launch shows; it never blocks an update, because update
   verification is the Ed25519 key above.

## Every release

1. Bump `version` in `mygo.json` (and `internal/ui.Version`, which a test keeps
   equal), commit.
2. `git tag v<version> && git push origin v<version>`.
3. The workflow creates the draft release, then macOS, Windows and Linux each
   build and upload: disk images, installers, and the update archives with
   their deltas.
4. Read the draft's notes, publish it.

## What must stay true

- Only stable releases. 0.x minors may break; patches must not.
- The update check is user-visible and switchable, and off means no network
  traffic at all.
- Nothing is uploaded by hand; `-upload` is the only publisher, so the
  checksums and signatures always describe the same build.
