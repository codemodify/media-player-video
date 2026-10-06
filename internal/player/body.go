package player

import (
	"fmt"
	"time"

	"github.com/codemodify/paintengine2d"
	"github.com/codemodify/uitoolkit/a11y"
	"github.com/codemodify/uitoolkit/layout"
	"github.com/codemodify/uitoolkit/platform"
	"github.com/codemodify/uitoolkit/style"
	"github.com/codemodify/uitoolkit/widget"
	"github.com/codemodify/uitoolkit/widgets"
)

type body struct {
	widget.Base
	p                          *Player
	video                      *videoView
	queue                      *widgets.ListView
	buttons                    map[string]*widgets.Button
	seek, volume               *widgets.Slider
	seekTouched, volumeTouched time.Time
	regions                    map[string]paintengine2d.Rect
	queueLook                  *queueLook
	bindings                   map[string]skinBinding
	panels                     []skinPanel
}

func newBody(p *Player) *body {
	b := &body{p: p, buttons: map[string]*widgets.Button{}}
	b.Init(b)
	b.SetManagesChildren(true)
	b.SetAccessibleName("Video player")
	b.video = newVideoView(p)
	b.Add(b.video)
	for _, entry := range [][2]string{{"file", "File"}, {"playback", "Playback"}, {"video", "Video"}, {"audio", "Audio"}, {"subtitles", "Subtitles"}, {"skins", "Skins"}} {
		key, label := entry[0], entry[1]
		b.button(key, label, func() { p.menu(b.buttons[key], p.menuItems(key)) })
	}
	b.button("prev", "Previous video", p.Previous)
	b.button("play", "Play / pause", p.PlayPause)
	b.button("stop", "Stop", p.Stop)
	b.button("next", "Next video", p.Next)
	b.button("open", "Open video", func() { p.OpenFiles(false, false) })
	b.button("mute", "Mute", p.Mute).Toggle = true
	b.button("playlist", "Show playlist", p.Playlist).Toggle = true
	b.button("full", "Fullscreen", p.Fullscreen).Toggle = true
	b.button("repeat", "Repeat", p.Repeat).Toggle = true
	b.button("shuffle", "Shuffle", p.Shuffle).Toggle = true
	b.button("add", "Add…", func() { p.OpenFiles(true, false) })
	b.button("remove", "Remove", func() { p.Remove(b.queue.Selected) })
	b.button("clear", "Clear", p.ClearQueue)
	for _, id := range []string{"close", "qclose", "minimize", "maximize", "rew", "ffwd", "slower", "faster", "compact", "menu", "pause", "folder", "volup", "voldown", "aspect", "chapterprev", "chapternext", "skipback", "skipforward"} {
		id := id
		b.button(id, id, func() { p.skinAction(id, b.buttons[id], p.Window) })
	}
	b.seek = widgets.NewSlider(0, 1, 0, nil)
	b.seek.Label = "Playback position"
	b.seek.Step = 0.01
	b.seek.OnInput = func(v float32) { b.seekTouched = time.Now(); p.Seek(float64(v) * p.Model.Current().Duration) }
	b.seek.Format = func(v float32) string { return Clock(float64(v) * p.Model.Current().Duration) }
	b.volume = widgets.NewSlider(0, 100, float32(p.Model.Volume), nil)
	b.volume.Label = "Volume"
	b.volume.Step = 5
	b.volume.Vertical = p.Skin.ID == "powerdvd"
	b.volume.Format = func(v float32) string { return fmt.Sprintf("%.0f%%", v) }
	b.volume.OnInput = func(v float32) { b.volumeTouched = time.Now(); p.Volume(float64(v)) }
	b.Add(b.seek)
	b.Add(b.volume)
	b.queue = widgets.NewListView(len(p.Model.Queue), func(i int) string {
		if i < 0 || i >= len(p.Model.Queue) {
			return ""
		}
		return p.Model.Queue[i].Title()
	}, nil)
	b.queue.RowHeight = 32
	b.queue.ItemDetail = func(i int) string {
		if i < 0 || i >= len(p.Model.Queue) || p.Model.Queue[i].Duration <= 0 {
			return ""
		}
		return Clock(p.Model.Queue[i].Duration)
	}
	b.queue.OnSelect = func(i int) { b.buttons["remove"].SetEnabled(i >= 0) }
	b.queue.Selected = p.Model.Index
	b.queue.OnActivate = p.PlayIndex
	b.queue.OnContext = func(i int, at paintengine2d.Point) {
		widgets.ShowContextMenu(b.queue, at, widgets.Item("Play", func() { p.PlayIndex(i) }), widgets.Item("Remove from playlist", func() { p.Remove(i) }), widgets.Item("Add video…", func() { p.OpenFiles(true, false) }))
	}
	b.queue.RowPaint = b.paintRow
	b.Add(b.queue)
	b.syncPainters()
	return b
}
func (b *body) button(id, label string, fn func()) *widgets.Button {
	c := widgets.NewButton(label, fn)
	c.SetAccessibleName(label)
	c.Tip = label
	b.buttons[id] = c
	b.Add(c)
	return c
}
func (b *body) sync() {
	m := b.p.Model
	if b.p.Skin.ID == "default" {
		if b.queueLook != nil {
			b.queue.SetLook(nil)
			b.queueLook = nil
		}
	} else if b.queueLook == nil || b.queueLook.LookAndFeel != b.Look() || b.queueLook.skin != b.p.Skin.ID {
		b.queueLook = &queueLook{LookAndFeel: b.Look(), background: b.p.Skin.Display, skin: b.p.Skin.ID}
		b.queue.SetLook(b.queueLook)
	}
	b.queue.Count = len(m.Queue)
	b.queue.Selected = min(b.queue.Selected, len(m.Queue)-1)
	b.queue.Invalidate()
	if time.Since(b.seekTouched) > 350*time.Millisecond {
		b.seek.SetValue(m.Fraction())
	}
	if time.Since(b.volumeTouched) > 350*time.Millisecond {
		b.volume.SetValue(float32(m.Volume))
	}
	b.seek.SetEnabled(m.Loaded && !m.Stopped && m.Current().Duration > 0)
	b.buttons["mute"].Checked = m.Muted
	b.buttons["playlist"].Checked = m.Playlist
	b.buttons["full"].Checked = b.p.fullscreen
	b.buttons["repeat"].Checked = m.Repeat != 0
	b.buttons["shuffle"].Checked = m.Shuffle
	b.buttons["remove"].SetEnabled(b.queue.Selected >= 0)
	b.buttons["clear"].SetEnabled(len(m.Queue) > 0)
	for _, button := range b.buttons {
		button.Invalidate()
	}
	if b.p.Skin.ID != "default" {
		b.queue.RowHeight = style.Dip(b.Look(), 15)
		b.p.syncSkinWindows()
		b.RequestLayout()
		return
	}
	if b.p.fullscreen {
		for _, id := range []string{"file", "playback", "video", "audio", "subtitles", "skins"} {
			b.buttons[id].SetVisible(false)
		}
	} else {
		for _, id := range []string{"file", "playback", "video", "audio", "subtitles", "skins"} {
			b.buttons[id].SetVisible(true)
		}
	}
	showQueue := m.Playlist && !b.p.fullscreen
	b.queue.SetVisible(showQueue)
	for _, id := range []string{"add", "remove", "clear"} {
		b.buttons[id].SetVisible(showQueue)
	}
}
func (b *body) Measure(c layout.Constraints) paintengine2d.Point {
	return c.Constrain(paintengine2d.Pt(style.Dip(b.Look(), 960), style.Dip(b.Look(), 600)))
}

// Geometry remains in logical pixels; input and painting share these rectangles.
func (b *body) Arrange(r paintengine2d.Rect) {
	b.SetBounds(r)
	if b.p.Skin.ID != "default" {
		b.arrangeSkin()
		return
	}
	s := style.LookScale(b.Look())
	if s <= 0 {
		s = 1
	}
	w, h := b.LocalBounds().Dx()/s, b.LocalBounds().Dy()/s
	b.regions = Geometry(w, h, b.p.Skin, b.p.Model.Playlist, b.p.fullscreen)
	at := func(id string) paintengine2d.Rect {
		r := b.regions[id]
		return paintengine2d.XYWH(r.Min.X*s, r.Min.Y*s, r.Dx()*s, r.Dy()*s)
	}
	b.video.Arrange(at("screen"))
	b.queue.Arrange(at("queue"))
	b.seek.Arrange(at("seek"))
	b.volume.Arrange(at("volume"))
	for id, c := range b.buttons {
		box := at(id)
		c.SetVisible(!box.Empty())
		c.Arrange(box)
	}
}
func (b *body) KeyPress(e widget.KeyEvent) bool { return b.p.Keys(e) }
func (b *body) MousePress(e widget.MouseEvent) bool {
	if e.Button == platform.ButtonRight {
		var items []*widgets.MenuItem
		for _, entry := range [][2]string{{"file", "File"}, {"playback", "Playback"}, {"video", "Video"}, {"audio", "Audio"}, {"subtitles", "Subtitles"}, {"skins", "Skins"}} {
			items = append(items, &widgets.MenuItem{Text: entry[1], Submenu: b.p.menuItems(entry[0])})
		}
		widgets.ShowContextMenu(b, e.Pos.Add(widget.DeviceOrigin(b)), items...)
		return true
	}
	if b.p.Skin.ID != "default" && e.Button == platform.ButtonLeft && !b.p.fullscreen {
		return dragSkin(b.p.Window, e.Pos, b.LocalBounds(), style.Dip(b.Look(), 1))
	}
	return false
}
func (b *body) Tooltip() string     { return b.p.Model.Error }
func (b *body) DropTypes() []string { return []string{"text/uri-list"} }
func (b *body) DropActionFor(offered platform.DragAction) platform.DragAction {
	return offered & platform.DragCopy
}
func (b *body) Drop(e widget.DropEvent) bool {
	if len(e.Paths) == 0 {
		return false
	}
	if len(e.Paths) == 1 && subtitleExtension(e.Paths[0]) {
		if b.p.Model.Stopped {
			b.p.fail(fmt.Errorf("open a video before adding subtitles"))
		} else {
			b.p.send("sub-add", e.Paths[0], "select")
		}
		return true
	}
	b.p.queuePaths(e.Paths, true)
	return true
}
func (b *body) Describe(n *a11y.Node) {
	n.Role = a11y.RoleGroup
	n.Name = "Video Player"
	n.Description = b.p.Model.State()
}
