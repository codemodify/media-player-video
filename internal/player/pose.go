package player

import (
	"github.com/codemodify/paintengine2d"
)

// Pose uses invented filenames and a code-drawn landscape. No decoder or files
// are touched by screenshots, so all appearances can be reviewed consistently.
func (p *Player) Pose(seconds float64) {
	p.Model.Queue = []Video{{Path: "/demo/The quiet coast.mp4", Duration: 264}, {Path: "/demo/A walk through the city.mkv", Duration: 732}, {Path: "/demo/After the rain.webm", Duration: 386}, {Path: "/demo/Weekend, 2004.avi", Duration: 1255}}
	p.Model.Index = 0
	p.Model.Position = min(264, max(0, seconds))
	p.Model.Stopped = false
	p.Model.Loaded = true
	p.Model.VideoWidth = 1920
	p.Model.VideoHeight = 1080
	p.Model.Generation = 1
	p.body.queue.Selected = 0
	image := paintengine2d.NewImage(960, 540)
	ctx := paintengine2d.NewContext(image)
	r := paintengine2d.XYWH(0, 0, 960, 540)
	ctx.DrawRect(r, gradient(r, paintengine2d.RGB(.24, .41, .52), paintengine2d.RGB(.85, .77, .56)))
	ctx.DrawCircle(paintengine2d.Pt(678, 192), 40, paintengine2d.Fill(paintengine2d.RGB(.96, .88, .62)))
	ctx.DrawRect(paintengine2d.XYWH(0, 275, 960, 265), gradient(paintengine2d.XYWH(0, 275, 960, 265), paintengine2d.RGB(.25, .45, .48), paintengine2d.RGB(.09, .24, .31)))
	for _, band := range []struct {
		points []paintengine2d.Point
		color  paintengine2d.Color
	}{
		{[]paintengine2d.Point{{X: 0, Y: 260}, {X: 84, Y: 213}, {X: 168, Y: 260}, {X: 294, Y: 166}, {X: 399, Y: 248}, {X: 540, Y: 278}, {X: 0, Y: 340}}, paintengine2d.RGB(.22, .36, .38)},
		{[]paintengine2d.Point{{X: 0, Y: 328}, {X: 167, Y: 290}, {X: 302, Y: 327}, {X: 461, Y: 390}, {X: 649, Y: 430}, {X: 770, Y: 540}, {X: 0, Y: 540}}, paintengine2d.RGB(.16, .26, .25)},
		{[]paintengine2d.Point{{X: 0, Y: 437}, {X: 152, Y: 373}, {X: 283, Y: 409}, {X: 393, Y: 501}, {X: 554, Y: 540}, {X: 0, Y: 540}}, paintengine2d.RGB(.09, .16, .16)},
	} {
		path := paintengine2d.NewPath()
		path.MoveTo(band.points[0].X, band.points[0].Y)
		for _, pt := range band.points[1:] {
			path.LineTo(pt.X, pt.Y)
		}
		path.Close()
		ctx.DrawPath(path, paintengine2d.Fill(band.color))
	}
	for y := float32(297); y < 530; y += 15 {
		width := float32(40) + (y-297)*.6
		ctx.DrawLine(paintengine2d.Pt(678-width, y), paintengine2d.Pt(678+width, y), paintengine2d.StrokePaint(paintengine2d.RGBA(.95, .82, .57, .18), 2))
	}
	p.body.video.frame = image
	p.refresh()
}
