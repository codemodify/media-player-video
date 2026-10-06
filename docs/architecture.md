# Application and toolkit boundary

This project follows the music player's separation of presentation, shared
state and decoding. It starts its own implementation around video playback
rather than importing the music player's audio-only engines and equalizers.

| Layer | Responsibility |
| --- | --- |
| `internal/player/Model` | Queue, current item, transport intent, settings, load generation |
| `internal/player/Player` | Shared actions, UI-thread event application, background file scans, session lifecycle |
| `internal/skins` | Original skin archives, PNG atlases, and registered layout packs |
| `internal/appicon` | Embedded logo and scaled native window/taskbar/tray icons |
| `internal/player/body.go`, `paint.go`, `geometry.go` | Original slot layouts, bitmap painters, shared widgets, and shaped windows |
| `internal/playback` | libmpv discovery, client commands, events, tracks, audio and video rendering |
| `uitoolkit` | Native windows, ordinary controls, painting, focus, keyboard/pointer input, drag/drop, dialogs and accessibility |

There are no changes to `uitoolkit` or `media-player-music`. The Go module pins
the same toolkit and paint-engine versions used by the music player.

## Decoder threads and frame ownership

The decoder is opened on a worker when playback is first requested. One
control goroutine owns normal libmpv calls. A separate locked OS thread owns
the software render context and calls only the render API. This honors the
thread boundary described in [libmpv's render API](https://github.com/mpv-player/mpv/blob/master/libmpv/render.h).

Frames travel through a one-frame channel. If the UI falls behind, the latest
decoded picture replaces an older pending one. Critical load/EOF/error events
are delivered in order, while periodic snapshots can be skipped when their
buffer is full. Commands and file requests have bounded queues and report
overflow. The UI polls these channels with a single toolkit timer.

The renderer converts into a C-owned RGB buffer, copies it into an immutable
Go frame and sets opaque alpha. The UI copies accepted frames into its own
reused paint-engine image and invalidates the video widget. The renderer and
UI never write the same image. Display dimensions are fitted to the render
target to preserve aspect ratio before the UI applies contain/crop/stretch.
Audio, timestamps and subtitles all come from the same libmpv core.

## Load identity and queue advancement

Every load has a generation, even when reloading the same filename. A
synchronous `loadfile replace` assigns an mpv playlist entry ID, which is read
before draining its events. Start and end events resolve that native ID to
the application's generation. Rapid replacement can skip starting a file,
so the implementation does not infer identity by counting start events.

The UI rejects snapshots and frames from previous generations. Only natural
EOF advances the queue. Stop increments the generation before sending its
command; replacement, decoder failure and a stale EOF do not advance it.
Repeat off stops after the last video, repeat playlist wraps, and repeat
video reloads the current item. Manual Next wraps the local queue.

## Persistence and file loading

File/folder requests are processed by one cancellable background worker and
committed in arrival order. A request is validated before it changes the
playlist. Nested directory symlinks are skipped. Closing cancels scans,
waits for any decoder startup, stops the control/render workers, frees the
render context and decoder, and unloads the shared library.

Sessions use a temporary file in the same directory, flush and close it, then
rename it over the prior JSON file. Only persistent model fields are written;
decoder handles, generations, frames, runtime status and track catalog are
transient. Restoration validates settings and paths and begins stopped.

## Current scope

The decoder supports local files and their audio/subtitle tracks through the
installed libmpv build. Video has a CPU rendering path with a 1280 × 720
surface cap. The player has no network catalog, DVD navigation, general-purpose runtime skin
archive loader, audio equalizer, or external VLC/MPlayer backend selector.
The player owns one system-tray item using the same logo as its native windows;
skin changes preserve it. Tray activation restores the window, and its menu
offers Show player and Quit. Closing the window exits and releases both the
tray item and decoder.
