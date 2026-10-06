package player

import (
	"testing"

	"github.com/codemodify/media-player-video/internal/playback"
	"github.com/codemodify/uitoolkit/platform"
)

func testTray(t *testing.T, p *Player) *platform.FakeStatusItem {
	t.Helper()
	t.Setenv("UITK_TRAY", "fake")
	p.openTray()
	tray, ok := p.tray.(*platform.FakeStatusItem)
	if !ok {
		t.Fatal("test tray did not open")
	}
	return tray
}

func TestCloseToTrayKeepsPlaybackAndRestoresWindows(t *testing.T) {
	for _, id := range []string{"default", "fusion", "gtzhd", "brownish", "bsplayer"} {
		t.Run(id, func(t *testing.T) {
			p, engine := newTestPlayer(t)
			if err := p.SetSkin(id); err != nil {
				t.Fatal(err)
			}
			tray := testTray(t, p)
			p.Pose(97)
			p.Playlist()
			p.App.PumpOnce()
			// Native close / Alt+F4 must hide, without destroying the window.
			p.Window.Inject(platform.Event{Kind: platform.EventClose})
			p.App.PumpOnce()
			if !p.hiddenToTray || p.Window.Visible() || p.Window.Closed() || p.App.Quitting() || engine.closed || !p.Model.Playlist {
				t.Fatal("desktop close did not hide the player while preserving its state")
			}
			engine.events <- playback.Event{Snapshot: playback.Snapshot{Generation: p.Model.Generation, Loaded: true, Position: 98, Duration: 264}}
			p.Pump()
			for _, pane := range []*skinPane{p.controller, p.playlist} {
				if pane != nil && pane.w.Visible() {
					t.Fatal("a satellite reappeared while the player was in the tray")
				}
			}
			if p.Model.Position != 98 || len(engine.commands) != 0 {
				t.Fatal("playback stopped or changed when hidden")
			}
			tray.Click()
			p.App.DrainPosted()
			p.App.PumpOnce()
			if p.hiddenToTray || !p.Window.Visible() || p.App.Quitting() || p.Model.Position != 98 {
				t.Fatal("tray activation did not restore the same player")
			}
			for _, pane := range []*skinPane{p.controller, p.playlist} {
				if pane != nil && !pane.w.Visible() {
					t.Fatal("tray activation did not restore the controller/playlist")
				}
			}
			if id != "default" {
				// The original bitmap close buttons use the same close behavior.
				p.body.buttons["close"].OnClick()
				p.App.PumpOnce()
				if !p.hiddenToTray || p.Window.Visible() || p.App.Quitting() {
					t.Fatal("the original close button did not hide to tray")
				}
			}
			tray.ClickMenu(1) // Explicit Quit still exits and releases the decoder.
			p.App.DrainPosted()
			if !p.App.Quitting() {
				t.Fatal("tray Quit did not request exit")
			}
			p.Close()
			if !engine.closed || !tray.Closed() || !p.Window.Closed() {
				t.Fatal("explicit exit did not release windows, decoder and tray")
			}
		})
	}
}

func TestTrayHostLossRestoresHiddenPlayer(t *testing.T) {
	p, _ := newTestPlayer(t)
	tray := testTray(t, p)
	p.App.PumpOnce()
	p.closePlayer()
	tray.SetShown(false)
	p.App.DrainPosted()
	p.App.PumpOnce()
	if p.hiddenToTray || !p.Window.Visible() || p.App.Quitting() {
		t.Fatal("losing the tray host left the hidden player unreachable")
	}
	p.closePlayer()
	if !p.App.Quitting() || p.hiddenToTray {
		t.Fatal("close without a displayed tray did not exit")
	}
}

func TestControllerCloseAndHiddenSkinChangeStayInTray(t *testing.T) {
	p, engine := newTestPlayer(t)
	if err := p.SetSkin("bsplayer"); err != nil {
		t.Fatal(err)
	}
	tray := testTray(t, p)
	p.Pose(97)
	p.App.PumpOnce()
	p.controller.w.Inject(platform.Event{Kind: platform.EventClose})
	p.App.PumpOnce()
	if !p.hiddenToTray || p.Window.Visible() || p.controller.w.Visible() || p.App.Quitting() {
		t.Fatal("closing the controller did not hide the entire player")
	}
	if err := p.SetSkin("fusion"); err != nil {
		t.Fatal(err)
	}
	p.App.PumpOnce()
	if !p.hiddenToTray || p.Window.Visible() || p.playlist.w.Visible() || p.engine != engine || p.Model.Position != 97 {
		t.Fatal("changing a hidden player's skin remapped its windows or changed playback")
	}
	tray.ClickMenu(0)
	p.App.DrainPosted()
	p.App.PumpOnce()
	if p.hiddenToTray || !p.Window.Visible() || p.playlist.w.Visible() {
		t.Fatal("Show player did not preserve the closed-playlist preference")
	}
}
