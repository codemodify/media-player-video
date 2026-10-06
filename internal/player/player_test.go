package player

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/codemodify/media-player-video/internal/playback"
	"github.com/codemodify/media-player-video/internal/skins"
	"github.com/codemodify/paintengine2d"
	"github.com/codemodify/uitoolkit/app"
	"github.com/codemodify/uitoolkit/platform"
	"github.com/codemodify/uitoolkit/widget"
)

type fakeEngine struct {
	commands []playback.Command
	events   chan playback.Event
	frames   chan playback.Frame
	closed   bool
}

func newFake() *fakeEngine {
	return &fakeEngine{events: make(chan playback.Event, 32), frames: make(chan playback.Frame, 1)}
}
func (f *fakeEngine) Send(c playback.Command) error { f.commands = append(f.commands, c); return nil }
func (f *fakeEngine) Events() <-chan playback.Event { return f.events }
func (f *fakeEngine) Frames() <-chan playback.Frame { return f.frames }
func (f *fakeEngine) Resize(int, int)               {}
func (f *fakeEngine) Close() error                  { f.closed = true; return nil }

func newTestPlayer(t *testing.T) (*Player, *fakeEngine) {
	t.Helper()
	engine := newFake()
	a := app.New(app.Options{Headless: true, Scale: 1})
	p, err := New(a, Options{Headless: true, Engine: engine})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Close)
	return p, engine
}

func TestSkinChangesPreserveDecoderAndPlayback(t *testing.T) {
	p, engine := newTestPlayer(t)
	p.Pose(97)
	p.Model.Speed = 1.5
	p.Model.Paused = true
	p.Model.Tracks = []playback.Track{{ID: 3, Kind: "sub", Selected: true}}
	for _, skin := range skins.Catalog {
		if err := p.SetSkin(skin.ID); err != nil {
			t.Fatal(err)
		}
		p.App.PumpOnce()
		if p.engine != engine || p.Model.Position != 97 || !p.Model.Paused || p.Model.Speed != 1.5 || len(p.Model.Queue) != 4 || len(engine.commands) != 0 {
			t.Fatalf("skin %s changed playback", skin.ID)
		}
	}
}
func TestSelectionDoesNotPlayAndOldEOFDoesNotAdvance(t *testing.T) {
	p, engine := newTestPlayer(t)
	p.Pose(2)
	p.body.queue.OnSelect(1)
	if len(engine.commands) != 0 || p.Model.Index != 0 {
		t.Fatal("selection played a video")
	}
	p.body.queue.OnActivate(1)
	generation := p.Model.Generation
	engine.events <- playback.Event{Snapshot: playback.Snapshot{Generation: generation - 1}, Ended: true}
	p.Pump()
	if p.Model.Index != 1 {
		t.Fatal("old EOF advanced current queue")
	}
	engine.events <- playback.Event{Snapshot: playback.Snapshot{Generation: generation}, Ended: true}
	p.Pump()
	if p.Model.Index != 2 {
		t.Fatal("current EOF did not advance")
	}
	p.Stop()
	engine.events <- playback.Event{Snapshot: playback.Snapshot{Generation: p.Model.Generation - 1}, Ended: true}
	p.Pump()
	if p.Model.Index != 2 || !p.Model.Stopped {
		t.Fatal("EOF after stop changed queue")
	}
}

func TestFullscreenRestoresPlaylistAndModel(t *testing.T) {
	p, engine := newTestPlayer(t)
	if err := p.SetSkin("default"); err != nil {
		t.Fatal(err)
	}
	p.Model.Playlist = true
	p.Pose(97)
	p.Fullscreen()
	p.App.PumpOnce()
	if !p.fullscreen || p.body.queue.Visible() || p.body.buttons["file"].Visible() {
		t.Fatal("fullscreen did not hide playlist and menu")
	}
	p.Fullscreen()
	p.App.PumpOnce()
	if p.fullscreen || !p.body.queue.Visible() || !p.body.buttons["file"].Visible() || p.Model.Position != 97 || len(engine.commands) != 0 {
		t.Fatal("fullscreen changed playback or did not restore the interface")
	}
}

func TestPlayAfterFinalEOFRestartsVideo(t *testing.T) {
	p, engine := newTestPlayer(t)
	p.Pose(97)
	p.Model.Index = len(p.Model.Queue) - 1
	engine.events <- playback.Event{Snapshot: playback.Snapshot{Generation: p.Model.Generation}, Ended: true}
	p.Pump()
	if !p.Model.Stopped || p.Model.Position != p.Model.Current().Duration {
		t.Fatal("final EOF did not stop at the end")
	}
	p.PlayPause()
	if p.Model.Stopped || p.Model.Position != 0 || p.seekOnLoad != 0 {
		t.Fatal("Play at EOF tried to resume beyond the final frame")
	}
}
func TestDropAppendsAndControlsUseSharedEngine(t *testing.T) {
	p, engine := newTestPlayer(t)
	p.Pose(20)
	path := filepath.Join(t.TempDir(), "drop.mp4")
	touch(t, path)
	if !p.body.Drop(widget.DropEvent{Paths: []string{path}, Action: platform.DragCopy}) {
		t.Fatal("drop refused")
	}
	deadline := time.Now().Add(time.Second)
	for len(p.Model.Queue) < 5 && time.Now().Before(deadline) {
		p.Pump()
		time.Sleep(time.Millisecond)
	}
	if len(p.Model.Queue) != 5 || p.Model.Index != 0 || p.Model.Position != 20 {
		t.Fatalf("drop changed current video: %+v", p.Model)
	}
	p.Volume(35)
	p.Speed(1.5)
	p.Seek(40)
	p.Mute()
	p.PlayPause()
	if len(engine.commands) != 5 {
		t.Fatalf("controls sent %d commands", len(engine.commands))
	}
	if engine.commands[2].Args[0] != "seek" || engine.commands[2].Args[1] != "40.000" {
		t.Fatalf("seek: %+v", engine.commands[2])
	}
}

func TestEveryAppearanceHasReachableControls(t *testing.T) {
	for _, skin := range []skins.Skin{skins.Catalog[len(skins.Catalog)-1]} {
		for _, size := range [][2]float32{{720, 440}, {960, 600}, {1280, 800}} {
			r := Geometry(size[0], size[1], skin, true, false)
			for _, id := range []string{"screen", "queue", "seek", "volume", "prev", "play", "stop", "next", "open", "shuffle", "repeat", "playlist", "full", "mute", "file", "playback", "video", "audio", "subtitles", "skins", "add", "remove", "clear"} {
				box := r[id]
				if box.Empty() || box.Min.X < 0 || box.Min.Y < 0 || box.Max.X > size[0] || box.Max.Y > size[1] {
					t.Fatalf("%s %v: %s outside client: %v", skin.ID, size, id, box)
				}
			}
			ids := []string{"prev", "play", "stop", "next", "open", "shuffle", "repeat", "playlist", "full", "mute", "volume"}
			for i, a := range ids {
				for _, b := range ids[i+1:] {
					if !r[a].Intersect(r[b]).Empty() {
						t.Fatalf("%s %v: %s overlaps %s", skin.ID, size, a, b)
					}
				}
			}
		}
	}
	for _, scale := range []float32{1, 1.75} {
		for _, skin := range skins.Catalog {
			t.Run(fmt.Sprintf("%s-%.2f", skin.ID, scale), func(t *testing.T) {
				a := app.New(app.Options{Headless: true, Scale: scale})
				p, err := New(a, Options{Headless: true, Skin: skin.ID})
				if err != nil {
					t.Fatal(err)
				}
				defer p.Close()
				p.Pose(97)
				a.PumpOnce()
				for _, id := range []string{"play", "seek", "volume"} {
					if id == "volume" && skin.ID == "zoom-player-gtz-hd" {
						continue // Original GTZ HD uses keyboard/menu volume.
					}
					var box paintengine2d.Rect
					if id == "seek" {
						box = p.body.seek.Bounds()
					} else if id == "volume" {
						box = p.body.volume.Bounds()
					} else {
						box = p.body.buttons[id].Bounds()
					}
					if box.Empty() {
						t.Fatalf("%s not laid out", id)
					}
				}
				if err = p.Window.WritePNG(filepath.Join(t.TempDir(), skin.ID+".png")); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}
