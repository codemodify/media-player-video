// Package appicon prepares the player logo for native windows and the tray.
package appicon

import (
	"bytes"
	_ "embed"
	"fmt"
	"image"
	"image/png"

	"github.com/codemodify/paintengine2d"
	"golang.org/x/image/draw"
)

// logoPNG is an exact copy of logo.png at the project root. Embedding it keeps
// the application independent of Downloads and its working directory.
//
//go:embed logo.png
var logoPNG []byte

// Images preserves transparency while resizing the logo for title bars,
// taskbars, application switchers and the system tray.
func Images() ([]*paintengine2d.Image, error) {
	source, err := png.Decode(bytes.NewReader(logoPNG))
	if err != nil {
		return nil, fmt.Errorf("decode player logo: %w", err)
	}
	sizes := []int{16, 22, 24, 32, 48, 64, 96, 128, 256}
	images := make([]*paintengine2d.Image, 0, len(sizes))
	for _, size := range sizes {
		resized := image.NewRGBA(image.Rect(0, 0, size, size))
		draw.CatmullRom.Scale(resized, resized.Bounds(), source, source.Bounds(), draw.Src, nil)
		images = append(images, paintengine2d.NewImageFromNRGBA(resized))
	}
	return images, nil
}
