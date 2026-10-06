# Original player skins

The player embeds original bitmap chassis and control states. Its retro
windows have no desktop caption: their title strips, window buttons,
transport, transparent corners, and playlist frames come from the skin.
The supplied app logo remains the native window/taskbar/tray identity.

| Skin | Source and author | Presentation |
| --- | --- | --- |
| Zoom Player Onyx | [Inmatrix gallery](https://www.inmatrix.com/zplayer/skins/), Inmatrix | 638 × 454 movie window, separate playlist, centered orange transport |
| Zoom Player Silverchrome | [Original archive](https://www.inmatrix.com/skins/gui/Silverchrome.zps), bLight | 412 × 363 movie window and separate playlist |
| Zoom Player Fusion 8.1.5 | [Original archive](https://www.inmatrix.com/skins/gui/Fusion%208.1.5.zps), Inmatrix | 618 × 427 movie window, centered transport, separate playlist |
| Zoom Player GTZ HD | [Original archive](https://www.inmatrix.com/skins/gui/GTZ%20HD.zps), Godwin | 300 × 282 movie window and original mini playlist |
| Zoom Player Brownish | [Original archive](https://www.inmatrix.com/skins/gui/Brownish.zps), bLight | Original 430 × 360 layout, right toolbar and lower transport |
| WMP 9 Series Default / Corona | [Preserved WMZ](https://w2krepo.somnolescent.net/Windows%20Media%20Player/Skins/9SeriesDefault.wmz), Microsoft Corporation | 346 × 344 chassis and sliding left playlist |
| BS.Player Base | [Official Base download](https://bsplayer.com/bsplayer-english/skin.html?cmd=download-skin&id=253), TinaZ | Original 800 × 82 controller and separate movie window |
| PowerDVD 5 Glow | [Preserved 2003 media](https://archive.org/details/powerdvd-5), Alan Yang | Original shaped silver/blue controller and separate movie window |
| VLC 0.8.5 Default Skin | [VLC 0.8.6 release source](https://download.videolan.org/pub/videolan/vlc/0.8.6/vlc-0.8.6.tar.bz2), aLtgLasS | Original Skins2 movie and playlist windows, released in 2006 |
| QuickTime iFix 040611 | [Original archive](https://www.inmatrix.com/skins/gui/iFix.zps), hills; design by Max Rudberg and Rene | 360 × 370 brushed-metal player and separate playlist |

QuickTime's [iFix preview](https://www.inmatrix.com/skins/gui/screenshots/iFix_preview.jpg)
is the exact reference supplied for this project. The original skin comments
date the Zoom Player port to September 5, 2004. The default Onyx download is
Inmatrix's currently maintained revision 19; it retains the design in the
reference with later button revisions. It is not an unchanged 2000s archive.
Silverchrome provides the requested earlier silver appearance.

Fusion, GTZ HD and Brownish use the archives corresponding to the supplied
previews. Inmatrix's gallery swaps the GTZ and GTZ HD preview links: the
requested `gtzhd_preview.png` depicts GTZ HD, confirmed against `GTZ HD.skn`
and its bitmap. GTZ HD opens taller than the archive's inconsistent initial
height so the movie area and original controls fit as in the preview.
Its main layout has no volume slider; keyboard and Audio-menu controls
remain available. Brownish supplies no playlist artwork, so its playlist
uses the application's ordinary list.

## Import and provenance

Unchanged archives and extracted originals are saved in
[`internal/skins/sources`](../internal/skins/sources). The
[manifest](../internal/skins/sources/manifest.json) records download URLs and
SHA-256 checksums. The binary embeds only generated PNG atlases and layout
JSON, and never executes the original JavaScript, SKN commands, or installer.
PowerDVD extraction reads bitmap resources and skin text from `ui_skin.dll`;
neither the DLL nor its installer is included in the repository.

[`tools/import_skins.py`](../tools/import_skins.py), requiring Pillow,
converts the preserved Zoom/QuickTime, BS.Player, PowerDVD, and VLC resources.
Zoom drawing coordinates are evaluated with a restricted arithmetic parser.
Transparency keys become alpha, control states keep their original pixels,
and resize anchors become toolkit slots. WMP's imported atlas was reused from
the neighboring music project and adapted to its original movie chassis and
left playlist. Its preserved WMZ SHA-256 is
`6af05468fcce6af5359d79b5c73ffdb0470e7c54aab077a42b1f2e365fa85be0`.

## Application behavior

The skin supplies artwork and coordinates. Ordinary toolkit buttons,
sliders, and lists provide pointer/keyboard input and accessibility. Right
click opens File, Playback, Video, Audio, Subtitles, and Skins menus wherever
an original interface does not have a corresponding button. All skins use
one libmpv instance and model; switching preserves the video frame, position,
speed, volume, track selection, and queue. A separate playlist's close button
hides it; the main/player controller's close button hides the entire player
to the system tray while playback continues. Tray activation restores the
player and its open satellites. Explicit Quit or Ctrl+Q exits. Without a
displayed tray icon, Close exits instead.

Playlist controls support adding files/folders, removing items, clearing,
shuffle/repeat, sorting, reordering, and local M3U/M3U8 save/load where the
source provides those buttons. DVD-specific controls remain part of the
original controller artwork, but this application plays local video files
and does not implement DVD menus. Original equalizer/audio-mode shortcuts
open the app's audio or interface menus; audio equalizers and historical
audio-only modes are not implemented. Default remains an ordinary toolkit
window. Fullscreen displays the video; keyboard shortcuts and right-click
menus remain available.

Dragging the video area hands the window to the desktop after the normal
drag threshold. Small click movements do not move the window, and a drag
does not count toward double-click fullscreen. Fullscreen suppresses window
dragging. The same gesture works across skins and separate movie windows.

## Original artwork notices

The project's license does not replace rights in imported third-party skin
resources. Microsoft retains its original all-rights-reserved notice in
Corona.wms and the scripts. Inmatrix, bLight, Godwin, hills, TinaZ, and CyberLink/Alan
Yang credits are retained in the source files. Those archives do not supply
a separate redistribution license. iFix's original readme records hills's
permission from Max Rudberg for the historical Zoom Player port.

VLC's original `credits.png` declares copyright © 2006 aLtgLasS and Creative
Commons Attribution-ShareAlike 2.5 UK: Scotland; its font credits Jos Buivenga.
The original credits image and release COPYING are preserved. The converted
VLC skin is an adaptation by this project under that artwork license:
transparency, video/queue binding, window resizing, and local-file controls
were adapted while retaining the original bitmap artwork.
