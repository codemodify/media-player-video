package skins

import (
	"image"
	"image/color"
	_ "image/png"
	"os"
	"testing"

	"github.com/codemodify/paintengine2d"
	"github.com/codemodify/uitoolkit/style"
	_ "golang.org/x/image/bmp"
)

// These comparisons verify the imported controls against the preserved BMPs,
// rather than snapshotting our own renderer as the reference.
func TestOriginalButtonPixelsSurviveImport(t *testing.T) {
	if err := Register(); err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		id, path   string
		x, y, w, h int
		key        color.NRGBA
	}{
		{"zoom-player", "sources/zoom-player/original/Onyx.bmp", 198, 883, 19, 19, color.NRGBA{R: 255, A: 255}},
		{"quicktime", "sources/quicktime/original/iFix 040611.bmp", 56, 160, 16, 16, color.NRGBA{G: 255, A: 255}},
		{"zoom-player-fusion", "sources/zoom-player-fusion/original/Fusion.bmp", 392, 172, 26, 24, color.NRGBA{R: 255, A: 255}},
		{"zoom-player-gtz-hd", "sources/zoom-player-gtz-hd/original/GTZ-HD.bmp", 667, 342, 21, 21, color.NRGBA{R: 255, A: 255}},
		{"zoom-player-brownish", "sources/zoom-player-brownish/original/brownish.bmp", 0, 72, 25, 27, color.NRGBA{R: 255, A: 255}},
	} {
		t.Run(tt.id, func(t *testing.T) {
			f, err := os.Open(tt.path)
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			source, _, err := image.Decode(f)
			if err != nil {
				t.Fatal(err)
			}
			lk := style.Appearance{Name: "video-" + tt.id}.Look()
			out := paintengine2d.NewImage(tt.w, tt.h)
			ctx := paintengine2d.NewContext(out)
			if !style.DrawSkinSlot(lk, ctx, paintengine2d.XYWH(0, 0, float32(tt.w), float32(tt.h)), "video.main", "close", 0) {
				t.Fatal("original button art is missing")
			}
			for y := 0; y < tt.h; y++ {
				for x := 0; x < tt.w; x++ {
					want := color.NRGBAModel.Convert(source.At(tt.x+x, tt.y+y)).(color.NRGBA)
					if want == tt.key {
						want = color.NRGBA{}
					}
					i := (y*tt.w + x) * 4
					got := color.NRGBA{R: out.Pix[i], G: out.Pix[i+1], B: out.Pix[i+2], A: out.Pix[i+3]}
					if got != want {
						t.Fatalf("original pixel (%d,%d) changed: got %v want %v", x, y, got, want)
					}
				}
			}
		})
	}
}
