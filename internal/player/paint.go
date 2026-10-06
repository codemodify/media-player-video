package player

import (
	"fmt"
	"math"

	"github.com/codemodify/media-player-video/internal/skins"
	"github.com/codemodify/paintengine2d"
	"github.com/codemodify/uitoolkit/style"
)

func Clock(seconds float64) string {
	if !finite(seconds) || seconds < 0 {
		seconds = 0
	}
	n := int(seconds)
	if n >= 3600 {
		return fmt.Sprintf("%d:%02d:%02d", n/3600, n/60%60, n%60)
	}
	return fmt.Sprintf("%02d:%02d", n/60, n%60)
}
func mix(a, b paintengine2d.Color, t float32) paintengine2d.Color {
	return paintengine2d.RGB(a.R+(b.R-a.R)*t, a.G+(b.G-a.G)*t, a.B+(b.B-a.B)*t)
}
func gradient(r paintengine2d.Rect, top, bottom paintengine2d.Color) paintengine2d.Paint {
	return paintengine2d.Linear(paintengine2d.LinearGradient{Start: r.Min, End: paintengine2d.Pt(r.Min.X, r.Max.Y), Stops: []paintengine2d.GradientStop{{Offset: 0, Color: top}, {Offset: 1, Color: bottom}}})
}
func text(ctx *paintengine2d.Context, font *style.Font, r paintengine2d.Rect, value string, color paintengine2d.Color, align style.Align) {
	if r.Empty() {
		return
	}
	value = font.Fit(value, max(0, r.Dx()))
	width := font.Advance(value)
	x := r.Min.X
	if align == style.AlignCenter {
		x += (r.Dx() - width) / 2
	}
	if align == style.AlignEnd {
		x += r.Dx() - width
	}
	font.Draw(ctx, value, paintengine2d.Pt(x, r.Min.Y+(r.Dy()-font.Height())/2), color)
}
func bevel(ctx *paintengine2d.Context, r paintengine2d.Rect, light, shadow paintengine2d.Color, d float32) {
	ctx.DrawLine(r.Min, paintengine2d.Pt(r.Max.X, r.Min.Y), paintengine2d.StrokePaint(light, d))
	ctx.DrawLine(r.Min, paintengine2d.Pt(r.Min.X, r.Max.Y), paintengine2d.StrokePaint(light, d))
	ctx.DrawLine(paintengine2d.Pt(r.Min.X, r.Max.Y), r.Max, paintengine2d.StrokePaint(shadow, d))
	ctx.DrawLine(paintengine2d.Pt(r.Max.X, r.Min.Y), r.Max, paintengine2d.StrokePaint(shadow, d))
}

func (b *body) syncPainters() {
	for id, c := range b.buttons {
		id := id
		c := c
		bindButton(b.p, c, func() skinBinding { return b.bindings[id] })
		c.Content = nil
		if b.p.Skin.ID == "default" {
			c.Content = func(ctx *paintengine2d.Context, r paintengine2d.Rect, st style.ControlState) {
				if isGlyph(id) {
					glyph(ctx, r.Inset(style.Dip(c.Look(), 7)), b.glyphFor(id), c.Look().Palette().Text)
				}
			}
		}
		if isGlyph(id) {
			c.Text = ""
		} else {
			switch id {
			case "file":
				c.Text = "File"
			case "playback":
				c.Text = "Playback"
			case "video":
				c.Text = "Video"
			case "audio":
				c.Text = "Audio"
			case "subtitles":
				c.Text = "Subtitles"
			case "skins":
				c.Text = "Skins"
			}
		}
	}
	b.seek.Travel = 5
	b.volume.Travel = 5
	b.seek.Painter = func(ctx *paintengine2d.Context, r paintengine2d.Rect, t float32, st style.ControlState) bool {
		return b.paintSlider(ctx, r, t, st, false)
	}
	b.volume.Painter = func(ctx *paintengine2d.Context, r paintengine2d.Rect, t float32, st style.ControlState) bool {
		return b.paintSlider(ctx, r, t, st, true)
	}
}
func isGlyph(id string) bool {
	switch id {
	case "prev", "play", "stop", "next", "open", "mute", "playlist", "full", "repeat", "shuffle":
		return true
	}
	return false
}
func (b *body) glyphFor(id string) string {
	if id == "play" && !b.p.Model.Stopped && !b.p.Model.Paused {
		return "pause"
	}
	if id == "mute" && b.p.Model.Muted {
		return "muted"
	}
	return id
}
func (b *body) paintSlider(ctx *paintengine2d.Context, r paintengine2d.Rect, t float32, st style.ControlState, volume bool) bool {
	if b.p.Skin.ID == "default" {
		return false
	}
	lk := b.seek.Look()
	name := "seek"
	if volume {
		lk = b.volume.Look()
		name = "volume"
	}
	if volume {
		frames := 0
		for i := 0; i < 64; i++ {
			if _, _, ok := style.SkinSpriteSize(lk, fmt.Sprintf("volume.%d", i)); !ok {
				break
			}
			frames++
		}
		if frames > 0 {
			style.DrawSkinSprite(lk, ctx, r, fmt.Sprintf("volume.%d", int(t*float32(frames-1)+.5)), paintengine2d.Color{})
			return true
		}
	}
	style.DrawSkinSprite(lk, ctx, r, name+".track", paintengine2d.Color{})
	ctx.Save()
	ctx.ClipRect(paintengine2d.XYWH(r.Min.X, r.Min.Y, r.Dx()*t, r.Dy()))
	if !style.DrawSkinSprite(lk, ctx, r, name+".fill", paintengine2d.Color{}) {
		style.DrawSkinSprite(lk, ctx, r, name+".foreground", paintengine2d.Color{})
	}
	ctx.Restore()
	thumb := name + ".thumb"
	tw, th, ok := style.SkinSpriteSize(lk, thumb)
	if !ok {
		thumb += ".normal"
		tw, th, ok = style.SkinSpriteSize(lk, thumb)
	}
	if ok {
		d := style.Dip(lk, 1)
		tw *= d
		th *= d
		style.DrawSkinSprite(lk, ctx, paintengine2d.XYWH(r.Min.X+(r.Dx()-tw)*t, r.Min.Y+(r.Dy()-th)/2, tw, th), thumb, paintengine2d.Color{})
	}
	if !volume && (b.p.Skin.ID == "zoom-player" || b.p.Skin.ID == "zoom-player-fusion" || b.p.Skin.ID == "zoom-player-gtz-hd" || b.p.Skin.ID == "zoom-player-brownish") {
		text(ctx, style.BakeFont(style.Dip(lk, 10), paintengine2d.White), r, Clock(b.p.Model.Position)+" / "+Clock(b.p.Model.Current().Duration), paintengine2d.White, style.AlignCenter)
	}
	return true
}

func (b *body) Paint(ctx *paintengine2d.Context) {
	if b.p.Skin.ID != "default" {
		b.paintSkin(ctx)
		return
	}
	p, m, skin := b.p, b.p.Model, b.appearance()
	d := style.Dip(b.Look(), 1)
	r := b.LocalBounds()
	ink := skin.Ink
	ctx.DrawRect(r, paintengine2d.Fill(skin.Face))
	at := func(id string) paintengine2d.Rect {
		x := b.regions[id]
		return paintengine2d.XYWH(x.Min.X*d, x.Min.Y*d, x.Dx()*d, x.Dy()*d)
	}
	toolbar, shelf := at("toolbar"), at("shelf")
	if !p.fullscreen {
		ctx.DrawRect(toolbar, gradient(toolbar, skin.Light, skin.Face))
		bevel(ctx, toolbar, skin.Light, skin.Shadow, d)
		text(ctx, style.BakeFont(11*d, ink), at("appearance"), skin.Label, ink, style.AlignEnd)
	}
	ctx.DrawRect(shelf, gradient(shelf, mix(skin.Light, skin.Face, 0.35), skin.Face))
	bevel(ctx, shelf, skin.Light, skin.Shadow, d)
	clockFont := style.BakeMonoFont(13*d, ink)
	text(ctx, clockFont, at("elapsed"), Clock(m.Position), ink, style.AlignStart)
	text(ctx, clockFont, at("duration"), Clock(m.Current().Duration), ink, style.AlignEnd)
	lcd := at("lcd")
	if lcd.Dx() > 20*d {
		ctx.DrawRoundRect(lcd, 3*d, 3*d, paintengine2d.Fill(skin.Display))
		bevel(ctx, lcd, skin.Shadow, skin.Light, d)
		title := m.Current().Title()
		if m.Current().Path == "" {
			title = "VIDEO PLAYER"
		}
		font := style.BakeFont(11*d, skin.DisplayInk)
		if lcd.Dy() > 33*d {
			text(ctx, font, paintengine2d.XYWH(lcd.Min.X+8*d, lcd.Min.Y+2*d, lcd.Dx()-16*d, 18*d), title, skin.DisplayInk, style.AlignStart)
			state := m.State()
			if m.VideoWidth > 0 {
				state = fmt.Sprintf("%s · %d × %d · %.2g×", state, m.VideoWidth, m.VideoHeight, m.Speed)
			}
			text(ctx, style.BakeMonoFont(10*d, skin.DisplayInk), paintengine2d.XYWH(lcd.Min.X+8*d, lcd.Min.Y+20*d, lcd.Dx()-16*d, 16*d), state, skin.DisplayInk, style.AlignStart)
		} else {
			text(ctx, font, lcd.Inset(6*d), title, skin.DisplayInk, style.AlignStart)
		}
	}
	text(ctx, style.BakeFont(10*d, ink), at("volume-label"), fmt.Sprintf("%.0f%%", m.Volume), ink, style.AlignCenter)
	if m.Playlist && !p.fullscreen {
		heading := at("queue-heading")
		ctx.DrawRect(heading, gradient(heading, skin.Light, skin.Face))
		unit := "videos"
		if len(m.Queue) == 1 {
			unit = "video"
		}
		text(ctx, style.BakeFont(11*d, ink), heading.Inset(8*d), fmt.Sprintf("PLAYLIST   ·   %d %s", len(m.Queue), unit), ink, style.AlignStart)
		if len(m.Queue) == 0 {
			text(ctx, style.BakeFont(12*d, ink), at("queue"), "Drop videos or add a folder", ink, style.AlignCenter)
		}
	}
	if !p.fullscreen {
		status := at("status")
		bevel(ctx, status, skin.Shadow, skin.Light, d)
		value := m.State()
		if m.Error == "" {
			value += "   ·   " + skin.Label + "   ·   F fullscreen   ·   Ctrl+O open"
		}
		text(ctx, style.BakeFont(11*d, ink), status.Inset(5*d), value, ink, style.AlignStart)
	}
}
func (b *body) paintRow(ctx *paintengine2d.Context, r paintengine2d.Rect, i int, st style.ControlState) bool {
	if b.p.Skin.ID == "default" {
		return false
	}
	if i < 0 || i >= len(b.p.Model.Queue) {
		return true
	}
	skin := b.p.Skin
	d := style.Dip(b.Look(), 1)
	bg, ink := skin.Display, skin.DisplayInk
	if skin.ID == "zoom-player" && i%2 != 0 {
		bg = paintengine2d.RGB(20.0/255, 20.0/255, 20.0/255)
	}
	if skin.ID == "quicktime" && i%2 != 0 {
		bg = paintengine2d.RGB(235.0/255, 243.0/255, 1)
	}
	if st&style.StateChecked != 0 {
		bg = mix(skin.Accent, skin.Display, 0.35)
		ink = paintengine2d.White
		if skin.ID == "zoom-player" {
			bg = paintengine2d.RGB(43.0/255, 48.0/255, 55.0/255)
		}
		if skin.ID == "quicktime" {
			bg = paintengine2d.RGB(61.0/255, 128.0/255, 223.0/255)
		}
		if skin.ID == "zoom-player-fusion" {
			bg = paintengine2d.RGB(58.0/255, 62.0/255, 63.0/255)
		}
		if skin.ID == "zoom-player-gtz-hd" {
			bg = paintengine2d.RGB(254.0/255, 119.0/255, 0)
			ink = paintengine2d.Black
		}
	}
	if (skin.ID == "zoom-player" || skin.ID == "zoom-player-fusion") && i == b.p.Model.Index {
		ink = skin.Accent
	}
	ctx.DrawRect(r, paintengine2d.Fill(bg))
	video := b.p.Model.Queue[i]
	label := fmt.Sprintf("%02d  %s", i+1, video.Title())
	text(ctx, style.BakeFont(11*d, ink), paintengine2d.XYWH(r.Min.X+2*d, r.Min.Y, r.Dx()-65*d, r.Dy()), label, ink, style.AlignStart)
	if video.Duration > 0 {
		text(ctx, style.BakeMonoFont(10*d, ink), paintengine2d.XYWH(r.Max.X-57*d, r.Min.Y, 49*d, r.Dy()), Clock(video.Duration), ink, style.AlignEnd)
	}
	return true
}

func (b *body) appearance() skins.Skin {
	s := b.p.Skin
	if s.ID == "default" {
		pal := b.Look().Palette()
		s.Face = pal.Surface
		s.Light = pal.BevelLight
		s.Shadow = pal.Border
		s.Ink = pal.Text
		s.Accent = pal.Accent
		s.Display = pal.Field
		s.DisplayInk = pal.Text
	}
	return s
}

// Transport marks are vector drawings, not font symbols, so every skin and DPI works.
func glyph(ctx *paintengine2d.Context, r paintengine2d.Rect, id string, color paintengine2d.Color) {
	cx, cy := (r.Min.X+r.Max.X)/2, (r.Min.Y+r.Max.Y)/2
	u := min(r.Dx(), r.Dy()) / 2
	fill := paintengine2d.Fill(color)
	line := paintengine2d.StrokePaint(color, max(1, u/5))
	tri := func(x, y, size float32, left bool) {
		path := paintengine2d.NewPath()
		direction := float32(1)
		if left {
			direction = -1
		}
		path.MoveTo(x-direction*size/2, y-size/2)
		path.LineTo(x+direction*size/2, y)
		path.LineTo(x-direction*size/2, y+size/2)
		path.Close()
		ctx.DrawPath(path, fill)
	}
	bar := func(x, y, w, h float32) { ctx.DrawRect(paintengine2d.XYWH(x-w/2, y-h/2, w, h), fill) }
	switch id {
	case "play":
		tri(cx+u*0.1, cy, u*1.5, false)
	case "pause":
		bar(cx-u*0.45, cy, u*0.42, u*1.6)
		bar(cx+u*0.45, cy, u*0.42, u*1.6)
	case "stop":
		bar(cx, cy, u*1.45, u*1.45)
	case "prev", "next":
		left := id == "prev"
		x := cx - u*0.7
		if !left {
			x = cx + u*0.7
		}
		bar(x, cy, u*0.3, u*1.55)
		tri(cx, cy, u*1.45, left)
	case "open":
		path := paintengine2d.NewPath()
		path.MoveTo(cx-u, cy+u*0.6)
		path.LineTo(cx, cy-u*0.8)
		path.LineTo(cx+u, cy+u*0.6)
		path.Close()
		ctx.DrawPath(path, fill)
		bar(cx, cy+u, u*2, u*0.3)
	case "playlist":
		for n := float32(-1); n <= 1; n++ {
			bar(cx-u*0.85, cy+n*u*0.75, u*0.3, u*0.24)
			bar(cx+u*0.2, cy+n*u*0.75, u*1.3, u*0.24)
		}
	case "full":
		for _, x := range []float32{-1, 1} {
			for _, y := range []float32{-1, 1} {
				corner := paintengine2d.Pt(cx+x*u, cy+y*u)
				ctx.DrawLine(corner, paintengine2d.Pt(cx+x*u*0.35, cy+y*u), line)
				ctx.DrawLine(corner, paintengine2d.Pt(cx+x*u, cy+y*u*0.35), line)
			}
		}
	case "mute", "muted":
		bar(cx-u*0.65, cy, u*0.55, u*0.8)
		path := paintengine2d.NewPath()
		path.MoveTo(cx-u*0.5, cy-u*0.4)
		path.LineTo(cx+u*0.25, cy-u)
		path.LineTo(cx+u*0.25, cy+u)
		path.LineTo(cx-u*0.5, cy+u*0.4)
		path.Close()
		ctx.DrawPath(path, fill)
		if id == "muted" {
			ctx.DrawLine(paintengine2d.Pt(cx+u*0.5, cy-u*0.5), paintengine2d.Pt(cx+u, cy+u*0.5), line)
			ctx.DrawLine(paintengine2d.Pt(cx+u, cy-u*0.5), paintengine2d.Pt(cx+u*0.5, cy+u*0.5), line)
		} else {
			path := paintengine2d.NewPath()
			path.AddArc(paintengine2d.Pt(cx+u*0.1, cy), u, u, -0.8, 1.6)
			ctx.DrawPath(path, line)
		}
	case "repeat":
		path := paintengine2d.NewPath()
		path.AddArc(paintengine2d.Pt(cx, cy), u*0.8, u*0.8, -0.9, math.Pi*1.75)
		ctx.DrawPath(path, line)
		tri(cx+u*0.6, cy-u*0.6, u*0.65, false)
	case "shuffle":
		ctx.DrawLine(paintengine2d.Pt(cx-u, cy-u*0.7), paintengine2d.Pt(cx+u*0.5, cy+u*0.7), line)
		ctx.DrawLine(paintengine2d.Pt(cx-u, cy+u*0.7), paintengine2d.Pt(cx+u*0.5, cy-u*0.7), line)
		tri(cx+u*0.65, cy+u*0.7, u*0.65, false)
		tri(cx+u*0.65, cy-u*0.7, u*0.65, false)
	}
}
