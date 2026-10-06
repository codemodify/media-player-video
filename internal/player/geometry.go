package player

import (
	"github.com/codemodify/media-player-video/internal/skins"
	"github.com/codemodify/paintengine2d"
)

// Geometry places the ordinary Default interface. Bitmap skins use their
// imported slots in skin_ui.go.
func Geometry(w, h float32, _ skins.Skin, playlist, fullscreen bool) map[string]paintengine2d.Rect {
	r := map[string]paintengine2d.Rect{}
	put := func(id string, x, y, w, h float32) { r[id] = paintengine2d.XYWH(x, y, max(0, w), max(0, h)) }
	toolbar := float32(38)
	if fullscreen {
		toolbar = 0
		playlist = false
	}
	status := float32(22)
	shelf := float32(100)
	if fullscreen {
		shelf = 82
		status = 0
	}
	bottom := h - status - shelf
	put("toolbar", 0, 0, w, toolbar)
	put("shelf", 0, bottom, w, shelf)
	put("status", 0, h-status, w, status)
	x := float32(8)
	for _, m := range []struct {
		id string
		w  float32
	}{{"file", 42}, {"playback", 72}, {"video", 52}, {"audio", 52}, {"subtitles", 72}, {"skins", 52}} {
		if !fullscreen {
			put(m.id, x, 5, m.w, 28)
		}
		x += m.w + 2
	}
	put("appearance", x+14, 0, w-x-24, toolbar)
	queueW := float32(0)
	if playlist {
		queueW = min(260, w*0.3)
	}
	put("screen", 8, toolbar+4, w-16-queueW, max(40, bottom-toolbar-12))
	if playlist {
		qx := w - queueW
		put("queue-heading", qx, toolbar+4, queueW-8, 30)
		put("queue", qx, toolbar+36, queueW-8, bottom-toolbar-82)
		for i, id := range []string{"add", "remove", "clear"} {
			bw := (queueW - 16) / 3
			put(id, qx+float32(i)*(bw+2), bottom-38, bw, 28)
		}
	}
	put("elapsed", 12, bottom+7, 74, 24)
	put("seek", 90, bottom+6, w-180, 26)
	put("duration", w-82, bottom+7, 70, 24)
	cy := bottom + 40
	playSize := float32(36)
	buttonSize := float32(30)
	x = 14
	for _, id := range []string{"prev", "play", "stop", "next", "open"} {
		size := buttonSize
		if id == "play" {
			size = playSize
		}
		put(id, x, cy+(playSize-size)/2, size, size)
		x += size + 6
	}
	if fullscreen {
		put("lcd", x+8, cy, w-x-355, 30)
	} else {
		put("lcd", x+14, cy+2, w-x-365, max(38, playSize-4))
	}
	x = w - 330
	for _, id := range []string{"shuffle", "repeat", "playlist", "full"} {
		put(id, x, cy+(playSize-28)/2, 28, 28)
		x += 32
	}
	put("mute", w-190, cy+(playSize-28)/2, 28, 28)
	put("volume", w-154, cy+(playSize-26)/2, 106, 26)
	put("volume-label", w-44, cy+(playSize-26)/2, 38, 26)
	return r
}
