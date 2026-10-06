package player

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/codemodify/media-player-video/internal/skins"
	"github.com/codemodify/paintengine2d"
	"github.com/codemodify/uitoolkit/app"
	"github.com/codemodify/uitoolkit/layout"
	"github.com/codemodify/uitoolkit/platform"
	"github.com/codemodify/uitoolkit/style"
	"github.com/codemodify/uitoolkit/widget"
	"github.com/codemodify/uitoolkit/widgets"
)

type skinBinding struct{ role, slot string }
type skinPanel struct {
	role string
	box  paintengine2d.Rect
}

// Panels use each source skin's coordinates, including the WMP drawer and
// Zoom's centered transport. The same rectangles drive painting and input.
func (p *Player) mainPanels(r paintengine2d.Rect, d float32) []skinPanel {
	if p.fullscreen || skins.SeparateController(p.Skin.ID) {
		return nil
	}
	if p.Skin.ID == "series-9" {
		x := float32(0)
		var panels []skinPanel
		if p.Model.Playlist {
			x = 250 * d
			panels = append(panels, skinPanel{"video.playlist", paintengine2d.XYWH(0, 25*d, 263*d, r.Dy()-80*d)})
		} else {
			// Corona slides the drawer behind the main panel when closed.
			// Its exposed edge fills the main artwork's transparent left strip.
			// Keep the original 26px fixed slice so fractional scaling matches
			// the open drawer; the rest is covered by the main panel.
			panels = append(panels, skinPanel{"video.playlist.edge", paintengine2d.XYWH(0, 25*d, 27*d, r.Dy()-80*d)})
		}
		panels = append(panels, skinPanel{"video.main", paintengine2d.XYWH(x, 0, r.Dx()-x, r.Dy()-55*d)}, skinPanel{"video.transport", paintengine2d.XYWH(x+12*d, r.Dy()-60*d, r.Dx()-x-12*d, 60*d)})
		return panels
	}
	panels := []skinPanel{{"video.main", r}}
	if lay, ok := style.SkinLayoutOf(p.Window.Look(), "video.transport"); ok {
		panels = append(panels, skinPanel{"video.transport", paintengine2d.XYWH((r.Dx()-lay.W*d)/2, r.Dy()-lay.H*d, lay.W*d, lay.H*d)})
	}
	return panels
}

func (b *body) arrangeSkin() {
	p := b.p
	r := b.LocalBounds()
	d := style.Dip(b.Look(), 1)
	b.bindings = map[string]skinBinding{}
	b.regions = map[string]paintengine2d.Rect{}
	b.panels = p.mainPanels(r, d)
	for id, c := range b.buttons {
		if c.Parent() == b {
			c.SetVisible(false)
			c.Arrange(paintengine2d.Rect{})
			_ = id
		}
	}
	if b.seek.Parent() == b {
		b.seek.SetVisible(false)
	}
	if b.volume.Parent() == b {
		b.volume.SetVisible(false)
	}
	if b.queue.Parent() == b {
		b.queue.SetVisible(false)
	}
	if p.fullscreen || skins.SeparateController(p.Skin.ID) {
		b.video.Arrange(r)
		return
	}
	for _, panel := range b.panels {
		lay, ok := style.SkinLayoutOf(b.Look(), panel.role)
		if !ok {
			continue
		}
		for _, slot := range lay.SlotNames() {
			box, _ := style.SkinSlotRect(b.Look(), panel.role, slot, panel.box)
			id := slot
			if panel.role == "video.playlist" && slot == "close" {
				id = "qclose"
			}
			b.regions[id] = paintengine2d.XYWH(box.Min.X/d, box.Min.Y/d, box.Dx()/d, box.Dy()/d)
			b.bindings[id] = skinBinding{panel.role, slot}
			switch id {
			case "screen":
				b.video.Arrange(box)
			case "queue", "rows", "list":
				b.queue.SetVisible(true)
				b.queue.Arrange(box)
			case "seek":
				b.seek.SetVisible(true)
				b.seek.Arrange(box)
			case "volume":
				b.volume.SetVisible(true)
				b.volume.Arrange(box)
			default:
				if c := b.buttons[id]; c != nil && c.Parent() == b {
					c.SetVisible(true)
					c.Arrange(box)
				}
			}
		}
	}
}

func (p *Player) createPlayerWindow() error {
	w, h := skins.Size(p.Skin.ID)
	minWidth := w
	decor := platform.DecorationsNone
	if p.Skin.ID == "default" {
		decor = platform.DecorationsClient
	}
	if p.Skin.ID == "series-9" && p.Model.Playlist {
		w += 250
		minWidth = w
	}
	win, err := p.App.NewWindow(platform.WindowOptions{Title: "Video Player", Width: w, Height: h, MinWidth: minWidth, MinHeight: h, Headless: p.opts.Headless, Decorations: decor})
	if err != nil {
		return err
	}
	p.Window = win
	p.body = newBody(p)
	win.SetContent(p.body)
	win.SetOnCloseRequest(p.closePlayer)
	if p.Skin.ID != "default" {
		setSkinShape(win, func(r paintengine2d.Rect, d float32, ctx *paintengine2d.Context) {
			if p.fullscreen || skins.SeparateController(p.Skin.ID) {
				ctx.DrawRect(r, paintengine2d.Fill(paintengine2d.White))
				return
			}
			for _, panel := range p.mainPanels(r, d) {
				style.DrawSkinLayout(win.Look(), ctx, panel.box, panel.role)
			}
		})
	}
	win.RequestFocus(p.body.video)
	if p.hiddenToTray {
		win.Hide()
	}
	return p.createSkinWindows()
}

func setSkinShape(w *app.Window, draw func(paintengine2d.Rect, float32, *paintengine2d.Context)) {
	w.SetShapeFunc(func(size paintengine2d.Point, scale float32) *platform.Shape {
		im := paintengine2d.NewImage(int(size.X), int(size.Y))
		ctx := paintengine2d.NewContext(im)
		ctx.Clear(paintengine2d.Transparent)
		draw(paintengine2d.XYWH(0, 0, size.X, size.Y), scale, ctx)
		for i := 3; i < len(im.Pix); i += 4 {
			if im.Pix[i] != 0 {
				im.Pix[i] = 255
			}
		}
		return platform.NewShapeImage(im)
	})
}

// A satellite owns its widgets, while playback and the native tray stay owned
// by Player. Closing a playlist hides it; closing a controller quits the app.
type skinPane struct {
	widget.Base
	p                    *Player
	w                    *app.Window
	role                 string
	buttons              map[string]*widgets.Button
	bindings             map[string]skinBinding
	queue                *widgets.ListView
	controller           bool
	visibilitySet, shown bool
}

func (p *Player) createSkinWindows() error {
	if p.Skin.ID == "default" {
		return nil
	}
	if skins.SeparateController(p.Skin.ID) {
		pane, err := p.newSkinPane("video.controller", true)
		if err != nil {
			return err
		}
		p.controller = pane
		for _, c := range p.body.buttons {
			pane.Add(c)
		}
		pane.buttons = p.body.buttons
		pane.Add(p.body.seek)
		pane.Add(p.body.volume)
	}
	if p.Skin.ID != "series-9" {
		pane, err := p.newSkinPane("video.playlist", false)
		if err != nil {
			return err
		}
		p.playlist = pane
		pane.queue = p.body.queue
		pane.Add(pane.queue)
		for _, id := range []string{"close", "maximize", "minimize", "shuffle", "repeat", "add", "folder", "remove", "clear", "open", "menu", "sort", "save", "load", "up", "down"} {
			id := id
			c := widgets.NewButton(id, nil)
			c.SetAccessibleName(id)
			c.Tip = id
			c.OnClick = func() {
				if id == "close" {
					p.Playlist()
				} else {
					p.skinAction(id, c, pane.w)
				}
			}
			pane.buttons[id] = c
			pane.Add(c)
			bindButton(p, c, func() skinBinding { return pane.bindings[id] })
		}
	}
	p.syncSkinWindows()
	return nil
}

func (p *Player) newSkinPane(role string, controller bool) (*skinPane, error) {
	w, h := 420, 300
	if size, ok := style.SkinLayoutSize(p.Window.Look(), role); ok {
		d := style.Dip(p.Window.Look(), 1)
		w, h = int(size.X/d), int(size.Y/d)
	}
	opts := platform.WindowOptions{Title: "Video Player · Playlist", Width: w, Height: h, MinWidth: w, MinHeight: h, Headless: p.opts.Headless, Decorations: platform.DecorationsNone, Role: platform.RoleUtility, Owner: p.Window.Surface(), SkipTaskbar: true}
	if controller {
		opts.Title = "Video Player · Controls"
		opts.Sizing = platform.SizingFixed
	}
	win, err := p.App.NewWindow(opts)
	if err != nil {
		return nil, err
	}
	pane := &skinPane{p: p, w: win, role: role, controller: controller, buttons: map[string]*widgets.Button{}}
	pane.Init(pane)
	pane.SetManagesChildren(true)
	win.SetContent(pane)
	win.SetOnCloseRequest(func() bool {
		if controller {
			return p.closePlayer()
		}
		if p.Model.Playlist {
			p.Playlist()
		}
		return false
	})
	setSkinShape(win, func(r paintengine2d.Rect, d float32, ctx *paintengine2d.Context) {
		if !style.DrawSkinLayout(win.Look(), ctx, r, role) {
			ctx.DrawRect(r, paintengine2d.Fill(paintengine2d.White))
		}
	})
	return pane, nil
}

func (p *Player) syncSkinWindows() {
	for _, pane := range []*skinPane{p.controller, p.playlist} {
		if pane == nil {
			continue
		}
		visible := !p.hiddenToTray && !p.fullscreen && (pane.controller || p.Model.Playlist)
		// X11 may not have delivered MapNotify yet for a new satellite.
		// State transitions must issue Hide even before Visible reports true.
		if !pane.visibilitySet || visible != pane.shown {
			if visible {
				pane.w.Show()
			} else {
				pane.w.Hide()
			}
			pane.visibilitySet, pane.shown = true, visible
		}
		for id, c := range pane.buttons {
			switch id {
			case "shuffle":
				c.Checked = p.Model.Shuffle
			case "repeat":
				c.Checked = p.Model.Repeat != 0
			}
			c.Invalidate()
		}
		pane.RequestLayout()
		pane.Invalidate()
	}
}
func (p *Player) closeSkinWindows() {
	for _, pane := range []*skinPane{p.controller, p.playlist} {
		if pane != nil {
			pane.w.Close()
		}
	}
	p.controller = nil
	p.playlist = nil
}

func (s *skinPane) Measure(c layout.Constraints) paintengine2d.Point {
	return c.Constrain(paintengine2d.Pt(420, 300))
}
func (s *skinPane) Arrange(r paintengine2d.Rect) {
	s.SetBounds(r)
	s.bindings = map[string]skinBinding{}
	box := s.LocalBounds()
	for _, c := range s.buttons {
		c.SetVisible(false)
		c.Arrange(paintengine2d.Rect{})
	}
	lay, ok := style.SkinLayoutOf(s.Look(), s.role)
	if !ok {
		d := style.Dip(s.Look(), 1)
		if s.queue != nil {
			s.queue.SetVisible(true)
			s.queue.Arrange(paintengine2d.XYWH(8*d, 28*d, box.Dx()-16*d, box.Dy()-68*d))
		}
		for i, id := range []string{"add", "remove", "clear", "close"} {
			c := s.buttons[id]
			c.SetVisible(true)
			c.Arrange(paintengine2d.XYWH(float32(8+i*96)*d, box.Dy()-34*d, 90*d, 26*d))
		}
		return
	}
	if s.controller {
		s.p.body.seek.SetVisible(false)
		s.p.body.volume.SetVisible(false)
	}
	for _, slot := range lay.SlotNames() {
		r, _ := style.SkinSlotRect(s.Look(), s.role, slot, box)
		s.bindings[slot] = skinBinding{s.role, slot}
		if c := s.buttons[slot]; c != nil {
			c.SetVisible(true)
			c.Arrange(r)
		}
		if slot == "queue" && s.queue != nil {
			s.queue.SetVisible(true)
			s.queue.Arrange(r)
		}
		if s.controller {
			s.p.body.bindings[slot] = skinBinding{s.role, slot}
			if slot == "seek" {
				s.p.body.seek.SetVisible(true)
				s.p.body.seek.Arrange(r)
			}
			if slot == "volume" {
				s.p.body.volume.SetVisible(true)
				s.p.body.volume.Arrange(r)
			}
		}
	}
}

func (b *body) paintSkin(ctx *paintengine2d.Context) {
	r := b.LocalBounds()
	ctx.DrawRect(r, paintengine2d.Paint{Color: paintengine2d.White, Blend: paintengine2d.BlendDestOut})
	for _, panel := range b.panels {
		style.DrawSkinLayout(b.Look(), ctx, panel.box, panel.role)
		b.p.paintSkinText(ctx, b.Look(), panel)
	}
}
func (s *skinPane) Paint(ctx *paintengine2d.Context) {
	r := s.LocalBounds()
	ctx.DrawRect(r, paintengine2d.Paint{Color: paintengine2d.White, Blend: paintengine2d.BlendDestOut})
	if !style.DrawSkinLayout(s.Look(), ctx, r, s.role) {
		ctx.DrawRect(r, paintengine2d.Fill(s.p.Skin.Face))
		text(ctx, style.BakeFont(style.Dip(s.Look(), 12), s.p.Skin.Ink), paintengine2d.XYWH(8, 0, r.Dx()-16, 26), "Playlist", s.p.Skin.Ink, style.AlignStart)
	} else {
		s.p.paintSkinText(ctx, s.Look(), skinPanel{s.role, r})
	}
}
func (p *Player) paintSkinText(ctx *paintengine2d.Context, lk style.LookAndFeel, panel skinPanel) {
	d := style.Dip(lk, 1)
	ink := p.Skin.Ink
	if lay, ok := style.SkinLayoutOf(lk, panel.role); ok {
		for _, slot := range lay.SlotNames() {
			if strings.HasPrefix(slot, "ornament.") {
				r, _ := style.SkinSlotRect(lk, panel.role, slot, panel.box)
				style.DrawSkinSlot(lk, ctx, r, panel.role, slot, 0)
			}
		}
	}
	for _, slot := range []string{"title", "elapsed", "status", "summary"} {
		r, ok := style.SkinSlotRect(lk, panel.role, slot, panel.box)
		if !ok {
			continue
		}
		value := p.Model.State()
		align := style.AlignStart
		size := float32(11)
		if slot == "title" {
			value = p.Model.Current().Title()
			if p.Model.Current().Path == "" {
				value = "Video Player"
			}
			align = style.AlignCenter
		}
		if panel.role == "video.playlist" && slot == "title" {
			value = fmt.Sprintf("Playlist · %d files", len(p.Model.Queue))
			if p.Skin.ID == "zoom-player-fusion" {
				value = fmt.Sprintf("%d Files", len(p.Model.Queue))
			}
		}
		if slot == "elapsed" {
			value = Clock(p.Model.Position)
			if p.Skin.ID != "quicktime" {
				value += " / " + Clock(p.Model.Current().Duration)
			}
			size = 10
			align = style.AlignCenter
		}
		if slot == "summary" {
			value = fmt.Sprintf("%d files", len(p.Model.Queue))
			if p.Skin.ID == "zoom-player-fusion" {
				total := float64(0)
				for _, v := range p.Model.Queue {
					total += v.Duration
				}
				value = Clock(total)
			}
			align = style.AlignCenter
			size = 10
		}
		color := ink
		if p.Skin.ID == "zoom-player-fusion" && slot == "summary" {
			color = p.Skin.Accent
		}
		if p.Skin.ID == "series-9" && slot == "status" {
			color = paintengine2d.RGB(.3, 1, .3)
		}
		if p.Skin.ID == "vlc" {
			color = paintengine2d.White
		}
		text(ctx, style.BakeFont(size*d, color), r, value, color, align)
	}
	if p.logo != nil {
		if r, ok := style.SkinSlotRect(lk, panel.role, "logo", panel.box); ok {
			ctx.DrawImageRect(p.logo, paintengine2d.XYWH(0, 0, float32(p.logo.Width), float32(p.logo.Height)), r)
		}
	}
	if p.Skin.ID == "zoom-player" {
		r, ok := style.SkinSlotRect(lk, panel.role, "seek", panel.box)
		if ok {
			text(ctx, style.BakeFont(10*d, paintengine2d.White), r, Clock(p.Model.Position)+" / "+Clock(p.Model.Current().Duration), paintengine2d.White, style.AlignCenter)
		}
	}
}

func bindButton(p *Player, c *widgets.Button, binding func() skinBinding) {
	c.Painter = func(ctx *paintengine2d.Context, r paintengine2d.Rect, st style.ControlState) bool {
		if p.Skin.ID == "default" {
			return false
		}
		slot := binding()
		if slot.role == "" {
			return false
		}
		if slot.slot == "play" && !p.Model.Stopped && !p.Model.Paused {
			st |= style.StateChecked
		}
		if slot.slot == "play" && p.Skin.ID == "series-9" && !p.Model.Stopped && !p.Model.Paused {
			slot.slot = "pause"
		}
		style.DrawSkinSlot(c.Look(), ctx, r, slot.role, slot.slot, st)
		return true
	}
	c.Shaper = func(lk style.LookAndFeel, size paintengine2d.Point) *style.Silhouette {
		slot := binding()
		return style.SkinSlotShape(lk, size, paintengine2d.XYWH(0, 0, size.X, size.Y), slot.role, slot.slot)
	}
}

func (p *Player) skinAction(id string, anchor widget.Component, w *app.Window) {
	switch id {
	case "close":
		p.closePlayer()
	case "qclose", "playlist":
		p.Playlist()
	case "minimize":
		w.Minimize()
	case "maximize":
		w.ToggleMaximize()
	case "play", "pause":
		p.PlayPause()
	case "stop":
		p.Stop()
	case "prev", "chapterprev":
		p.Previous()
	case "next", "chapternext":
		p.Next()
	case "rew", "skipback":
		p.Seek(p.Model.Position - 10)
	case "ffwd", "skipforward":
		p.Seek(p.Model.Position + 10)
	case "slower":
		p.Speed(p.Model.Speed - .25)
	case "faster":
		p.Speed(p.Model.Speed + .25)
	case "volup":
		p.Volume(p.Model.Volume + 5)
	case "voldown":
		p.Volume(p.Model.Volume - 5)
	case "mute":
		p.Mute()
	case "full":
		p.Fullscreen()
	case "shuffle":
		p.Shuffle()
	case "repeat":
		p.Repeat()
	case "open":
		p.OpenFiles(false, false)
	case "load":
		p.loadPlaylist(anchor)
	case "save":
		p.savePlaylist(anchor)
	case "sort":
		p.sortQueue()
	case "up":
		p.moveQueue(-1)
	case "down":
		p.moveQueue(1)
	case "aspect":
		p.menu(anchor, p.menuItems("video"))
	case "add":
		p.OpenFiles(true, false)
	case "folder":
		p.OpenFiles(true, true)
	case "remove":
		p.Remove(p.body.queue.Selected)
	case "clear":
		p.ClearQueue()
	case "skins":
		p.menu(anchor, p.menuItems("skins"))
	case "audio":
		p.menu(anchor, p.menuItems("audio"))
	case "compact":
		iw, ih := skins.Size(p.Skin.ID)
		if p.Skin.ID == "series-9" && p.Model.Playlist {
			iw += 250
		}
		p.Window.SetSize(iw, ih)
	case "menu":
		var items []*widgets.MenuItem
		for _, entry := range [][2]string{{"file", "File"}, {"playback", "Playback"}, {"video", "Video"}, {"audio", "Audio"}, {"subtitles", "Subtitles"}, {"skins", "Skins"}} {
			items = append(items, &widgets.MenuItem{Text: entry[1], Submenu: p.menuItems(entry[0])})
		}
		p.menu(anchor, items)
	}
}

func dragSkin(w *app.Window, pos paintengine2d.Point, r paintengine2d.Rect, d float32) bool {
	if pos.X >= r.Max.X-18*d && pos.Y >= r.Max.Y-18*d {
		return w.StartResize(platform.EdgeRight | platform.EdgeBottom)
	}
	return w.StartMove()
}
func (s *skinPane) MousePress(e widget.MouseEvent) bool {
	if e.Button == platform.ButtonRight {
		var items []*widgets.MenuItem
		for _, entry := range [][2]string{{"file", "File"}, {"playback", "Playback"}, {"video", "Video"}, {"audio", "Audio"}, {"subtitles", "Subtitles"}, {"skins", "Skins"}} {
			items = append(items, &widgets.MenuItem{Text: entry[1], Submenu: s.p.menuItems(entry[0])})
		}
		widgets.ShowContextMenu(s, e.Pos.Add(widget.DeviceOrigin(s)), items...)
		return true
	}
	if e.Button == platform.ButtonLeft {
		return dragSkin(s.w, e.Pos, s.LocalBounds(), style.Dip(s.Look(), 1))
	}
	return false
}
func (s *skinPane) KeyPress(e widget.KeyEvent) bool { return s.p.Keys(e) }
func (s *skinPane) DropTypes() []string             { return s.p.body.DropTypes() }
func (s *skinPane) DropActionFor(a platform.DragAction) platform.DragAction {
	return s.p.body.DropActionFor(a)
}
func (s *skinPane) Drop(e widget.DropEvent) bool { return s.p.body.Drop(e) }

// WriteScreenshots includes every original panel, even a currently hidden
// playlist, so changes to imported artwork can be reviewed without playback.
func (p *Player) WriteScreenshots(dir string) error {
	if err := p.Window.WritePNG(filepath.Join(dir, p.Skin.ID+"-main.png")); err != nil {
		return err
	}
	for _, pane := range []*skinPane{p.controller, p.playlist} {
		if pane != nil {
			pane.w.Show()
			p.App.PumpOnce()
			suffix := "playlist"
			if pane.controller {
				suffix = "controller"
			}
			if err := pane.w.WritePNG(filepath.Join(dir, p.Skin.ID+"-"+suffix+".png")); err != nil {
				return err
			}
		}
	}
	return nil
}
