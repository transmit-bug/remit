---
name: mygo-desktop
description: Desktop services of a MyGo app that reach outside the window — dialogs, notifications, clipboard, shell, displays, theme and accent, power, network status and proxies, secrets, watching files, global shortcuts, menus and tray icons, sound and the media session, released updates, and app lifecycle beyond Run. Use when the app must reach the operating system — ask the user for a file or an answer, notify, store a token, follow dark mode, register a shortcut or menu, put an icon in the tray, or react to a second instance or an opened file. Widgets inside the window are mygo-components.
---

# Desktop services

A MyGo app reaches the operating system through package-level services, each
safe from any goroutine: `mygo.Dialog`, `Notification`, `Clipboard`, `Shell`,
`Screen`, `Theme`, `Power`, `NowPlaying`, `Network`, `Secrets`, `WatchFiles`,
`GlobalShortcut`, and the app's menus, tray icon and lifecycle hooks. This
skill routes a need to its service and carries the rules that bite; the API
surface stays in the module docs, and the version protocol below is what
keeps this file true across MyGo releases.

Scope: services outside the window; widgets inside it are `../mygo-components`.

## Where the rest is

| Material | Reached for |
|---|---|
| `$MYGO/docs/native.md`, where `MYGO=$(go list -m -f '{{.Dir}}' github.com/egoist/mygo)` | every service above: options, returns, per-platform notes |
| `$MYGO/docs/menus.md` | menu templates, roles, accelerators, context menus, tray icons |
| `$MYGO/docs/app.md` | lifecycle hooks, quitting, single instance, deep links, file associations, recent documents, login, directories, Dock and taskbar |
| `$MYGO/docs/data-transfer.md` | clipboard and drag payloads beyond text and images |
| `$MYGO/docs/audio.md` | the `audio` package: sounds, streams, players, codec imports |
| `$MYGO/docs/updates.md`, `$MYGO/docs/github-actions.md` | update signing keys, release sources, publishing from CI |

## Resolve the version, then read the section

Protocol at `../mygo-usage`: read the section of each service you are about
to call in the copy `go list -m -f '{{.Dir}}'` names, prose included — the
per-platform notes state behaviour you would otherwise guess at. Done when
every option you pass exists in that copy.

## Need → service, and the rule that bites

| Need | Service | The rule that bites |
|---|---|---|
| Ask for a file, a save path or an answer | `mygo.Dialog.Open/Save/Message` | Blocks until answered: call it from a bound method or a goroutine of the app's own; `Parent` attaches it to a window (a sheet on macOS). Cancel arrives as `nil`, `""`, or the button index; `CancelButton` defaults to the first button labeled Cancel or No — set it by index when Escape must keep the data |
| Tell about a finished job | `mygo.NewNotification` | macOS shows them for packaged apps only (`NotificationsSupported()`); the first `Show` waits for the permission answer (`ErrNotificationsDenied`); a clicked one stays in Notification Center until `Close`, or `ClearNotifications` on `OnDidBecomeActive`; `ID` routes a click that relaunches the app |
| Clipboard, plain | `mygo.Clipboard.WriteText/ReadText/WriteImage` | Files, custom formats and lazy providers go through `transfer.Data` (`data-transfer.md`); a page may also use `navigator.clipboard` |
| Open, reveal, trash | `mygo.Shell.OpenExternal/OpenPath/ShowItemInFolder/TrashItem` | `TrashItem` fails where the trash cannot take the item, and leaves it in place; `FileIcon`'s size is DIPs × the display's scale factor |
| The installed apps, as a launcher shows | `mygo.Shell.InstalledApps/LaunchApp` | Reading them touches the disk, so keep it off the main thread; draw only the icons that show, on the main thread, and keep them |
| Which display; rearrange on change | `mygo.Screen` | `WorkArea` excludes menu bar, Dock and taskbars; `DisplayMatching(rect)` names the display a window is on; `OnDisplaysChanged` re-places windows |
| Follow dark mode, accent, contrast | `mygo.Theme` | `SetSource` before `Run` starts the app in a saved appearance; `Preferences()` is read once and kept until `OnUpdated`; the default `ui` theme already follows accent, contrast and text scale |
| A long task keeps the machine awake | `mygo.Power.KeepAwake(reason, displayOn)` | Returns a release function — defer it; `OnSuspend`/`OnResume` pause and resume the work around it |
| A sound effect or music | the `audio` package (`DecodeSound`, `OpenStream`) | Codecs are imports (`audio/vorbis`, …); an `OpenStream` player closes its file — close it when done; a `Sound`'s players need no closing; mixed in Go, no cgo, browsers included |
| Show what plays; take media keys | `mygo.NowPlaying.Set` + `OnCommand` | While it shows something, media keys drive the app: declare exactly the `Commands` bits you handle, mirror playback back with `SetPlayback`, and `Clear()` when playback stops |
| Online, metered | `mygo.Network` | `Online` is a route out, not a server that answers; the `OnChanged` listener runs on the main thread; push the status to views as a broadcast typed event |
| Go HTTP follows the system proxy | `netproxy.Transport()` | Environment variables win over system settings; system answers are kept 5 s; MyGo's own requests (updater, fetch and websocket plugins) already use it |
| A token or key in the system store | `mygo.Secrets` | At most `MaxSecretSize` (2560 B); `errors.Is(err, errors.ErrUnsupported)` is a machine without a credential store — fall back to asking each run; a `mygo dev` build keeps secrets under `<identifier>.dev`, apart from the installed app's |
| React to file changes | `mygo.WatchFiles` | Own goroutine draining `Next`; each batch is the diff since the last — the hint to re-read, not a changelog; 50 ms debounce, 500 ms `MaxDelay`; a gone directory is a `FileRemove` of `.` then `io.EOF`; `ErrWatchLimit` at `MaxEntries`; `HintsOnly` trades paths for cost — for a reloader only |
| Works while backgrounded | `mygo.GlobalShortcut.Register` | The callback runs on the main thread, briefly; a taken key fails to register; under Wayland the desktop portal provides it — GNOME 48+, KDE 6, an installed app |
| Menus, tray icon | `mygo.NewMenu`, `mygo.NewTray` | Templates of `*mygo.MenuItem`; roles carry standard labels, keys and platform pruning — keep `RoleAppMenu`/`RoleEditMenu` on macOS; change items in use through their `Set*` methods so the native menu follows; `NewTray` errors without the AppIndicator library on Linux |
| One instance at a time | `mygo.App.RequestSingleInstanceLock` | `false` in the second instance — return from `main`; its argv reaches `OnSecondInstance`; on Windows and Linux this is also the road later launches' deep links and files take |
| Opened by URL or by file type | `App.OnOpenURL` / `App.OnOpenFile` | Register before `Run` to catch the launch arguments too; schemes and file types are declared in the mygo config; `RegisterURLScheme` covers unpackaged apps |
| Data and logs in the right place | `mygo.App.Path(PathUserData/PathCache/PathLogs/…)` | Directories derive from the app name and are created on first use; `SetPath` isolates test data |
| Start with the system | `App.SetOpenAtLogin` | Needs the packaged app on macOS 13+; `WasOpenedAtLogin` starts that session hidden |
| Install a released update | `mygo.Updater` | Only updates signed with the `mygo keygen` pair are accepted, and losing the secret key strands installed apps; the public key and the release source — a GitHub repo (give it `tagPrefix` when it shares releases) or an HTTPS `url` — live in the mygo config |

## Lifecycle beyond `Run`

- `Run` belongs to the main goroutine; windows are created in `WhenReady`.
  Before it, `main` sets the app up: bind, listeners, `SetName`, `SetMenu`,
  `Theme.SetSource`, `SetOpenAtLogin`, `RegisterURLScheme`, and reads of
  `App.Path`, `App.Locale`, `Power.IsOnBattery`. Everything else panics with
  a message naming the call: call it from `WhenReady` on.
- Closing the last window quits, unless `OnWindowAllClosed` has a listener —
  the macOS pattern keeps the app running and reopens on `OnActivate`.
  `Quit` walks `OnClose`, `OnBeforeQuit`, `OnWillQuit`; each may cancel.
  `Exit(code)` skips the events; `Relaunch()` restarts with the same
  arguments.

## Verify

Build and test as `../mygo-usage` says. The tester covers this skill's
surface where it reaches the window: `tt.Clipboard`, `tt.Announcements`,
`tt.OpenedURLs`, `tt.Menu`/`tt.ChooseMenuItem`, `tt.SetDark`,
`tt.SetPreferences`, `tt.SetLocale`, `tt.SetDirection`. It cannot answer a
native dialog, deliver a real notification, touch the system keyring or the
desktop's proxy settings: drive those by hand on the real platform.

Done when: every service option you passed exists in the copy go.mod
resolves to, and each of those tester-blind paths is named in the handoff
as unverified, left for a human to drive on the real platform.
