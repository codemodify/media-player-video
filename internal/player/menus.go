package player

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/codemodify/media-player-video/internal/skins"
	"github.com/codemodify/paintengine2d"
	"github.com/codemodify/uitoolkit/platform"
	"github.com/codemodify/uitoolkit/widget"
	"github.com/codemodify/uitoolkit/widgets"
)

func (p *Player) OpenFiles(appendToQueue, folder bool) {
	mode := widgets.FileOpen
	title := "Open video"
	filter := "*.mp4 *.m4v *.mkv *.webm *.avi *.mov *.wmv *.mpg *.mpeg *.m2ts *.mts *.ts *.vob *.ogv *.flv *.3gp"
	if appendToQueue {
		title = "Add video to playlist"
	}
	if folder {
		mode = widgets.FileOpenFolder
		title = "Add video folder"
		filter = ""
	}
	path, _ := os.UserHomeDir()
	if current := p.Model.Current().Path; current != "" {
		path = filepath.Dir(current)
	}
	widgets.ShowFileDialog(p.body, widgets.FileDialogOptions{Title: title, Path: path, Mode: mode, Filter: filter, Native: true, OnPick: func(path string) { p.queuePaths([]string{path}, appendToQueue || folder) }})
}
func (p *Player) OpenSubtitle() {
	if p.Model.Stopped {
		p.fail(fmt.Errorf("open a video before adding subtitles"))
		return
	}
	widgets.ShowFileDialog(p.body, widgets.FileDialogOptions{Title: "Load subtitles", Path: filepath.Dir(p.Model.Current().Path), Mode: widgets.FileOpen, Filter: "*.srt *.ass *.ssa *.vtt *.sub", Native: true, OnPick: func(path string) { p.send("sub-add", path, "select") }})
}
func (p *Player) menu(from widget.Component, items []*widgets.MenuItem) {
	box := from.LocalBounds()
	at := widget.DeviceOrigin(from).Add(paintengine2d.Pt(0, box.Dy()))
	widgets.ShowContextMenu(from, at, items...)
}
func checked(text string, on bool, action func()) *widgets.MenuItem {
	item := widgets.Item(text, action)
	item.Checkable = true
	item.Checked = on
	return item
}
func disabled(text string) *widgets.MenuItem {
	item := widgets.Item(text, nil)
	item.Disabled = true
	return item
}
func (p *Player) menuItems(name string) []*widgets.MenuItem {
	switch name {
	case "file":
		return []*widgets.MenuItem{widgets.ItemAccel("Open video…", "Ctrl+O", func() { p.OpenFiles(false, false) }), widgets.Item("Add video…", func() { p.OpenFiles(true, false) }), widgets.Item("Add folder…", func() { p.OpenFiles(true, true) }), widgets.Sep(), widgets.Item("Clear playlist", p.ClearQueue), widgets.Sep(), widgets.ItemAccel("Quit", "Ctrl+Q", p.App.Quit)}
	case "playback":
		items := []*widgets.MenuItem{widgets.ItemAccel("Play / pause", "Space", p.PlayPause), widgets.ItemAccel("Stop", "S", p.Stop), widgets.ItemAccel("Previous video", "P", p.Previous), widgets.ItemAccel("Next video", "N", p.Next), widgets.Sep(), widgets.ItemAccel("Back 10 seconds", "Left", func() { p.Seek(p.Model.Position - 10) }), widgets.ItemAccel("Forward 10 seconds", "Right", func() { p.Seek(p.Model.Position + 10) }), widgets.Sep(), checked("Shuffle", p.Model.Shuffle, p.Shuffle)}
		for i, label := range []string{"Repeat off", "Repeat playlist", "Repeat video"} {
			i := i
			items = append(items, checked(label, p.Model.Repeat == i, func() { p.Model.Repeat = i; p.refresh() }))
		}
		items = append(items, widgets.Sep())
		for _, speed := range []float64{0.5, 0.75, 1, 1.25, 1.5, 2} {
			speed := speed
			items = append(items, checked(fmt.Sprintf("Speed %.2g×", speed), p.Model.Speed == speed, func() { p.Speed(speed) }))
		}
		return items
	case "video":
		items := []*widgets.MenuItem{checked("Fullscreen", p.fullscreen, p.Fullscreen), checked("Show playlist", p.Model.Playlist, p.Playlist), widgets.Sep()}
		for i, label := range []string{"Original aspect ratio", "Fill window (crop)", "Stretch to window"} {
			i := i
			items = append(items, checked(label, p.fit == i, func() { p.fit = i; p.body.video.Invalidate() }))
		}
		return items
	case "audio":
		return p.trackItems("audio")
	case "subtitles":
		return p.trackItems("sub")
	case "skins":
		var items []*widgets.MenuItem
		for _, skin := range skins.Catalog {
			skin := skin
			items = append(items, checked(skin.Label, p.Skin.ID == skin.ID, func() { p.SetSkin(skin.ID) }))
		}
		return items
	}
	return nil
}
func (p *Player) trackItems(kind string) []*widgets.MenuItem {
	var items []*widgets.MenuItem
	property := "aid"
	if kind == "sub" {
		property = "sid"
		items = append(items, widgets.Item("Load subtitle file…", p.OpenSubtitle), widgets.Item("Hide subtitles", func() { p.send("set", "sid", "no") }), widgets.Sep())
	} else {
		items = append(items, checked("Mute", p.Model.Muted, p.Mute), widgets.Sep())
	}
	count := 0
	for _, t := range p.Model.Tracks {
		if t.Kind != kind {
			continue
		}
		t := t
		label := fmt.Sprintf("Track %d", t.ID)
		if t.Title != "" {
			label += " · " + t.Title
		}
		if t.Language != "" {
			label += " [" + t.Language + "]"
		}
		items = append(items, checked(label, t.Selected, func() { p.send("set", property, fmt.Sprint(t.ID)) }))
		count++
	}
	if count == 0 {
		items = append(items, disabled("No tracks available"))
	}
	return items
}
func (p *Player) Keys(e widget.KeyEvent) bool {
	r := e.Rune
	if r == 0 {
		r = platform.KeyChar(e.Key)
	}
	if e.Mods.Ctrl() {
		switch r {
		case 'o':
			p.OpenFiles(e.Mods.Shift(), false)
			return true
		case 'q':
			p.App.Quit()
			return true
		case 'l':
			p.Playlist()
			return true
		case 'k':
			for i, s := range skins.Catalog {
				if s.ID == p.Skin.ID {
					p.SetSkin(skins.Catalog[(i+1)%len(skins.Catalog)].ID)
					break
				}
			}
			return true
		}
	}
	if e.Mods.Ctrl() || e.Mods.Alt() {
		return false
	}
	switch e.Key {
	case platform.KeySpace:
		p.PlayPause()
		return true
	case platform.KeyLeft:
		p.Seek(p.Model.Position - 10)
		return true
	case platform.KeyRight:
		p.Seek(p.Model.Position + 10)
		return true
	case platform.KeyUp:
		p.Volume(p.Model.Volume + 5)
		return true
	case platform.KeyDown:
		p.Volume(p.Model.Volume - 5)
		return true
	case platform.KeyEscape:
		if p.fullscreen {
			p.Fullscreen()
			return true
		}
		return false
	case platform.KeyF11:
		p.Fullscreen()
		return true
	}
	switch r {
	case 'f':
		p.Fullscreen()
	case 'm':
		p.Mute()
	case 's':
		p.Stop()
	case 'n':
		p.Next()
	case 'p':
		p.Previous()
	case '[':
		p.Speed(p.Model.Speed - 0.25)
	case ']':
		p.Speed(p.Model.Speed + 0.25)
	default:
		return false
	}
	return true
}
