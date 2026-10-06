package player

import (
	"image"
	"time"

	"github.com/codemodify/paintengine2d"
	"github.com/codemodify/uitoolkit/a11y"
	"github.com/codemodify/uitoolkit/layout"
	"github.com/codemodify/uitoolkit/platform"
	"github.com/codemodify/uitoolkit/style"
	"github.com/codemodify/uitoolkit/widget"
)

type videoView struct {
	widget.Base
	p         *Player
	frame     *paintengine2d.Image
	lastClick time.Time
	clickPos  paintengine2d.Point
	dragging  bool
}

func newVideoView(p *Player) *videoView {
	v := &videoView{p: p}
	v.Init(v)
	v.SetWantsFocus(true)
	v.SetAccessibleName("Video display")
	return v
}
func (v *videoView) SetFrame(frame *image.NRGBA) {
	if v.frame == nil || v.frame.Width != frame.Rect.Dx() || v.frame.Height != frame.Rect.Dy() || v.frame.Stride != frame.Stride {
		v.frame = paintengine2d.WrapImage(frame.Pix, frame.Rect.Dx(), frame.Rect.Dy(), frame.Stride)
	} else {
		copy(v.frame.Pix, frame.Pix)
		v.frame.Touch()
	}
	v.Invalidate()
}
func (v *videoView) Clear() { v.frame = nil; v.Invalidate() }
func (v *videoView) Measure(c layout.Constraints) paintengine2d.Point {
	return c.Constrain(paintengine2d.Pt(480, 270))
}
func (v *videoView) Arrange(r paintengine2d.Rect) { v.SetBounds(r) }
func (v *videoView) Paint(ctx *paintengine2d.Context) {
	r := v.LocalBounds()
	ctx.DrawRect(r, paintengine2d.Fill(paintengine2d.RGB(.015, .02, .035)))
	d := style.Dip(v.Look(), 1)
	if v.frame != nil {
		dst := r
		if v.p.fit != 2 {
			scale := min(r.Dx()/float32(v.frame.Width), r.Dy()/float32(v.frame.Height))
			if v.p.fit == 1 {
				scale = max(r.Dx()/float32(v.frame.Width), r.Dy()/float32(v.frame.Height))
			}
			w, h := float32(v.frame.Width)*scale, float32(v.frame.Height)*scale
			dst = paintengine2d.XYWH((r.Dx()-w)/2, (r.Dy()-h)/2, w, h)
		}
		ctx.Save()
		ctx.ClipRect(r)
		ctx.DrawImageRect(v.frame, paintengine2d.XYWH(0, 0, float32(v.frame.Width), float32(v.frame.Height)), dst)
		ctx.Restore()
	} else {
		center := paintengine2d.XYWH(r.Dx()/2-29*d, r.Dy()/2-68*d, 58*d, 58*d)
		if v.p.logo != nil {
			ctx.DrawImageRect(v.p.logo, paintengine2d.XYWH(0, 0, float32(v.p.logo.Width), float32(v.p.logo.Height)), center)
		} else {
			ctx.DrawRoundRect(center, 9*d, 9*d, gradient(center, paintengine2d.RGB(.15, .22, .34), paintengine2d.RGB(.05, .09, .16)))
			glyph(ctx, center.Inset(17*d), "play", paintengine2d.RGB(.65, .77, .93))
		}
		message := "Drop a video to begin"
		if v.p.Model.Current().Path != "" {
			message = v.p.Model.Current().Title()
		}
		if !v.p.Model.Stopped {
			message = "Opening video…"
		}
		text(ctx, style.BakeFont(18*d, paintengine2d.White), paintengine2d.XYWH(15*d, r.Dy()/2+3*d, r.Dx()-30*d, 28*d), message, paintengine2d.RGB(.8, .86, .94), style.AlignCenter)
		text(ctx, style.BakeFont(11*d, paintengine2d.White), paintengine2d.XYWH(15*d, r.Dy()/2+35*d, r.Dx()-30*d, 20*d), "Ctrl+O open  ·  Space play / pause  ·  F fullscreen", paintengine2d.RGB(.42, .5, .63), style.AlignCenter)
	}
}
func (v *videoView) MousePress(e widget.MouseEvent) bool {
	if e.Button != platform.ButtonLeft {
		v.dragging = false
		v.lastClick = time.Time{}
		return false
	}
	now := time.Now()
	if now.Sub(v.lastClick) < 350*time.Millisecond && v.nearClick(e.Pos) {
		v.dragging = false
		v.p.Fullscreen()
		v.lastClick = time.Time{}
	} else {
		v.lastClick = now
		v.clickPos = e.Pos
		v.dragging = !v.p.fullscreen
	}
	return true
}
func (v *videoView) nearClick(pos paintengine2d.Point) bool {
	delta := pos.Sub(v.clickPos)
	threshold := v.p.App.TitleBarPrefs().DragThreshold
	if threshold <= 0 {
		threshold = 8
	}
	threshold *= max(style.LookScale(v.Look()), 1)
	return delta.X*delta.X+delta.Y*delta.Y <= threshold*threshold
}
func (v *videoView) MouseMove(e widget.MouseEvent) bool {
	if !v.dragging || v.nearClick(e.Pos) {
		return false
	}
	v.dragging = false
	v.lastClick = time.Time{} // A drag must not become the first half of a double click.
	if !v.p.fullscreen {
		v.p.Window.StartMove()
	}
	return true
}
func (v *videoView) MouseRelease(e widget.MouseEvent) bool {
	if e.Button != platform.ButtonLeft {
		return false
	}
	v.dragging = false
	return true
}
func (v *videoView) FocusLost() {
	v.dragging = false
	v.lastClick = time.Time{}
}
func (v *videoView) Describe(n *a11y.Node) {
	n.Role = a11y.RoleImage
	n.Name = "Video display"
	n.Description = v.p.Model.Current().Title() + " · " + v.p.Model.State()
}
