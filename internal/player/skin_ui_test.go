package player

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/codemodify/media-player-video/internal/skins"
	"github.com/codemodify/paintengine2d"
	"github.com/codemodify/uitoolkit/app"
	"github.com/codemodify/uitoolkit/platform"
	"github.com/codemodify/uitoolkit/widget"
)

func clickControl(p *Player, w *app.Window, c widget.Component, fraction float32) {
	r := c.LocalBounds()
	at := widget.DeviceOrigin(c).Add(paintengine2d.Pt(r.Dx()*fraction, r.Dy()/2))
	w.Inject(platform.Event{Kind: platform.EventMouseDown, Pos: at, Button: platform.ButtonLeft})
	w.Inject(platform.Event{Kind: platform.EventMouseUp, Pos: at, Button: platform.ButtonLeft})
	p.App.PumpOnce()
}

func TestOriginalSkinInputAtMultipleScales(t *testing.T) {
	for _, scale := range []float32{1, 1.75} {
		for _, skin := range skins.Catalog {
			if skin.ID == "default" {
				continue
			}
			t.Run(skin.ID+"-"+Clock(float64(scale)*100), func(t *testing.T) {
				a := app.New(app.Options{Headless: true, Scale: scale})
				engine := newFake()
				p, err := New(a, Options{Headless: true, Skin: skin.ID, Engine: engine})
				if err != nil {
					t.Fatal(err)
				}
				defer p.Close()
				p.Pose(97)
				a.PumpOnce()
				w := p.Window
				if p.controller != nil {
					w = p.controller.w
				}
				button := p.body.buttons["play"]
				if button.Bounds().Empty() || !button.Visible() {
					t.Fatal("original play control is absent")
				}
				clickControl(p, w, button, .5)
				if !p.Model.Paused || len(engine.commands) != 1 || engine.commands[0].Args[0] != "set" {
					t.Fatal("original play button did not pause the shared decoder")
				}
				clickControl(p, w, p.body.seek, .72)
				if p.Model.Position < 170 || p.Model.Position > 210 || engine.commands[len(engine.commands)-1].Args[0] != "seek" {
					t.Fatalf("timeline input missed the skin's track: %.2f", p.Model.Position)
				}
				if skin.ID == "zoom-player-gtz-hd" {
					// The original GTZ HD main window omits a volume bar.
					if p.body.volume.Visible() {
						t.Fatal("invented a volume control absent from GTZ HD")
					}
					p.Keys(widget.KeyEvent{Key: platform.KeyUp})
				} else {
					clickControl(p, w, p.body.volume, .3)
				}
				if engine.commands[len(engine.commands)-1].Args[1] != "volume" {
					t.Fatal("original volume control did not send volume")
				}
				if p.body.buttons["file"].Visible() {
					t.Fatal("generic menu strip is present on a bitmap skin")
				}
				p.Window.SetSize(900, 600)
				a.PumpOnce()
				if !p.Window.WindowState().Fullscreen && p.body.video.Bounds().Empty() {
					t.Fatal("video disappeared on resize")
				}
				for id, c := range p.body.buttons {
					if !c.Visible() || c.Parent() != p.body {
						continue
					}
					r := c.Bounds()
					box := p.body.LocalBounds()
					if r.Min.X < 0 || r.Min.Y < 0 || r.Max.X > box.Max.X+.5 || r.Max.Y > box.Max.Y+.5 {
						t.Fatalf("%s lies outside resized skin: %v", id, r)
					}
				}
			})
		}
	}
}

func TestVideoAreaDragKeepsClicksAndFullscreen(t *testing.T) {
	for _, scale := range []float32{1, 1.75} {
		for _, skin := range []string{"zoom-player", "default", "bsplayer"} {
			t.Run(skin+"-"+Clock(float64(scale)*100), func(t *testing.T) {
				a := app.New(app.Options{Headless: true, Scale: scale})
				e := newFake()
				p, err := New(a, Options{Headless: true, Skin: skin, Engine: e})
				if err != nil {
					t.Fatal(err)
				}
				defer p.Close()
				p.Pose(97)
				a.PumpOnce()
				w := p.Window
				o := w.Surface().(*platform.Offscreen)
				v := p.body.video
				at := widget.DeviceOrigin(v).Add(paintengine2d.Pt(60*scale, 60*scale))
				send := func(kind platform.EventKind, pos paintengine2d.Point) {
					w.Inject(platform.Event{Kind: kind, Pos: pos, Button: platform.ButtonLeft})
				}
				tap := func(pos paintengine2d.Point) {
					send(platform.EventMouseDown, pos)
					send(platform.EventMouseUp, pos)
					a.PumpOnce()
				}
				send(platform.EventMouseDown, at)
				send(platform.EventMouseMove, at.Add(paintengine2d.Pt(3*scale, 0)))
				send(platform.EventMouseUp, at)
				a.PumpOnce()
				if o.FrameCalls().Moves != 0 || p.fullscreen {
					t.Fatal("a small click movement started a drag or fullscreen")
				}
				v.lastClick = time.Now()
				tap(at)
				if !p.fullscreen || o.FrameCalls().Moves != 0 {
					t.Fatal("double-click did not enter fullscreen without moving")
				}
				a.PumpOnce()
				at = widget.DeviceOrigin(v).Add(paintengine2d.Pt(60*scale, 60*scale))
				send(platform.EventMouseDown, at)
				send(platform.EventMouseMove, at.Add(paintengine2d.Pt(30*scale, 0)))
				send(platform.EventMouseUp, at)
				a.PumpOnce()
				if o.FrameCalls().Moves != 0 || !p.fullscreen {
					t.Fatal("fullscreen allowed a window drag")
				}
				v.lastClick = time.Time{}
				tap(at)
				v.lastClick = time.Now()
				tap(at)
				if p.fullscreen {
					t.Fatal("double-click did not leave fullscreen")
				}
				a.PumpOnce()
				at = widget.DeviceOrigin(v).Add(paintengine2d.Pt(60*scale, 60*scale))
				send(platform.EventMouseDown, at)
				send(platform.EventMouseMove, at.Add(paintengine2d.Pt(30*scale, 0)))
				send(platform.EventMouseUp, at)
				a.PumpOnce()
				if o.FrameCalls().Moves != 1 {
					t.Fatal("dragging the video did not hand the window to the desktop")
				}
				tap(at)
				if p.fullscreen {
					t.Fatal("drag was counted as the first half of a double-click")
				}
				tap(at.Add(paintengine2d.Pt(35*scale, 0)))
				if p.fullscreen || len(e.commands) != 0 || p.Model.Position != 97 {
					t.Fatal("separate clicks or dragging changed fullscreen/playback")
				}
			})
		}
	}
}

func TestWMPDrawerAndSatellitePlaylistKeepPlayback(t *testing.T) {
	p, e := newTestPlayer(t)
	p.Pose(97)
	if err := p.SetSkin("series-9"); err != nil {
		t.Fatal(err)
	}
	p.App.PumpOnce()
	w, h := p.Window.Size()
	p.Playlist()
	p.App.PumpOnce()
	dw, dh := p.Window.Size()
	if dw != w+250 || dh != h || !p.body.queue.Visible() {
		t.Fatal("WMP original left drawer did not open")
	}
	p.body.buttons["qclose"].OnClick()
	p.App.PumpOnce()
	cw, _ := p.Window.Size()
	if cw != w || p.Model.Playlist {
		t.Fatal("WMP drawer did not close")
	}
	if err := p.SetSkin("quicktime"); err != nil {
		t.Fatal(err)
	}
	p.Playlist()
	p.App.PumpOnce()
	if p.playlist == nil || !p.playlist.w.Visible() {
		t.Fatal("QuickTime playlist did not open")
	}
	p.playlist.buttons["close"].OnClick()
	p.App.PumpOnce()
	if p.playlist.w.Visible() || p.Model.Playlist || p.Window.Closed() || e.closed || len(e.commands) != 0 || p.Model.Position != 97 {
		t.Fatal("closing the satellite changed playback or closed the player")
	}
}

func TestWMPBorderSurvivesDrawerToggleAndResize(t *testing.T) {
	for _, scale := range []float32{1, 1.75} {
		t.Run(fmt.Sprint(scale), func(t *testing.T) {
			a := app.New(app.Options{Headless: true, Scale: scale})
			p, err := New(a, Options{Headless: true, Skin: "series-9", Engine: newFake()})
			if err != nil {
				t.Fatal(err)
			}
			defer p.Close()
			for _, size := range [][2]int{{346, 344}, {620, 520}} {
				p.Window.SetSize(size[0], size[1])
				a.PumpOnce()
				for cycle := 0; cycle < 2; cycle++ {
					w, h := p.Window.SurfaceSize()
					shape := p.Window.Shape().Raster(w, h)
					closed := p.Window.CaptureSurface()
					// The original drawer supplies the outer edge in both states.
					// Check the whole vertical gap, including the native silhouette.
					for y := int(math.Ceil(float64(33 * scale))); y < h-int(80*scale); y++ {
						if !shape.Contains(int(6*scale), y) {
							t.Fatalf("closed playlist cuts out the left border at y=%d (size %v, cycle %d)", y, size, cycle)
						}
					}
					p.Playlist()
					a.PumpOnce()
					opened := p.Window.CaptureSurface()
					for y := int(math.Ceil(float64(33 * scale))); y < h-int(80*scale); y++ {
						for x := int(2 * scale); x < int(11*scale); x++ {
							ci, oi := y*closed.RowStride()+x*4, y*opened.RowStride()+x*4
							for c := 0; c < 4; c++ {
								if closed.Pix[ci+c] != opened.Pix[oi+c] {
									t.Fatalf("drawer changes the original border pixel at (%d,%d), size %v", x, y, size)
								}
							}
						}
					}
					p.Playlist()
					a.PumpOnce()
				}
			}
		})
	}
}

func TestNewZoomPlaylistWellsStayInsideWindowShape(t *testing.T) {
	for _, id := range []string{"fusion", "gtzhd"} {
		t.Run(id, func(t *testing.T) {
			p, engine := newTestPlayer(t)
			if err := p.SetSkin(id); err != nil {
				t.Fatal(err)
			}
			p.Pose(97)
			p.Playlist()
			p.App.PumpOnce()
			pane := p.playlist
			w, h := pane.w.SurfaceSize()
			shape := pane.w.Shape().Raster(w, h)
			q := p.body.queue
			r := q.Bounds()
			if !q.Visible() || r.Empty() || !shape.Contains(int(r.Min.X+4), int(r.Min.Y+4)) || !shape.Contains(int((r.Min.X+r.Max.X)/2), int((r.Min.Y+r.Max.Y)/2)) {
				t.Fatal("the original playlist well was cut out of the native window")
			}
			at := widget.DeviceOrigin(q).Add(paintengine2d.Pt(24, q.RowHeightPx()*1.5))
			pane.w.Inject(platform.Event{Kind: platform.EventMouseDown, Pos: at, Button: platform.ButtonLeft})
			pane.w.Inject(platform.Event{Kind: platform.EventMouseUp, Pos: at, Button: platform.ButtonLeft})
			p.App.PumpOnce()
			if q.Selected != 1 || p.Model.Index != 0 || p.Model.Position != 97 || len(engine.commands) != 0 {
				t.Fatal("selecting an original playlist row changed playback or missed the row")
			}
			p.Playlist()
			p.App.PumpOnce()
			if pane.w.Visible() || p.Window.Closed() {
				t.Fatal("closing the original playlist did not leave the player open")
			}
		})
	}
}

func TestSkinWindowCornersFollowOriginalArtwork(t *testing.T) {
	p, _ := newTestPlayer(t)
	p.App.PumpOnce()
	w, h := p.Window.SurfaceSize()
	shape := p.Window.Shape().Raster(w, h)
	if shape.Contains(0, 0) || !shape.Contains(w/2, h/2) {
		t.Fatal("Onyx silhouette does not match the chassis")
	}
	if err := p.SetSkin("quicktime"); err != nil {
		t.Fatal(err)
	}
	p.App.PumpOnce()
	w, h = p.Window.SurfaceSize()
	shape = p.Window.Shape().Raster(w, h)
	if shape.Contains(0, 0) || !shape.Contains(w/2, h/2) {
		t.Fatal("iFix silhouette does not match the chassis")
	}
}

func TestPlaylistReorderKeepsCurrentVideo(t *testing.T) {
	p, e := newTestPlayer(t)
	p.Pose(97)
	p.body.queue.Selected = 1
	current := p.Model.Current().Path
	selected := p.Model.Queue[1].Path
	p.sortQueue()
	if p.Model.Current().Path != current || p.Model.Queue[p.body.queue.Selected].Path != selected {
		t.Fatal("sort changed the playing or selected video")
	}
	p.moveQueue(1)
	if p.Model.Current().Path != current || p.Model.Queue[p.body.queue.Selected].Path != selected || p.Model.Position != 97 || len(e.commands) != 0 {
		t.Fatal("reordering restarted playback")
	}
	path := filepath.Join(t.TempDir(), "playlist.m3u8")
	if err := os.WriteFile(path, []byte("\ufeff#EXTM3U\n#EXTINF:2,First\nfirst.mp4\nsecond.mkv\n"), 0600); err != nil {
		t.Fatal(err)
	}
	paths, err := readPlaylist(path)
	if err != nil || len(paths) != 2 || paths[0] != filepath.Join(filepath.Dir(path), "first.mp4") {
		t.Fatalf("M3U load: %v %v", paths, err)
	}
}
