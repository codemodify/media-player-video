// Package skins registers original player artwork and its control layouts.
package skins

import (
	"embed"
	"fmt"
	"github.com/codemodify/paintengine2d"
	"github.com/codemodify/uitoolkit/style"
	"io/fs"
	"strings"
	"sync"
)

type Skin struct {
	ID, Label, Reference, Era                             string
	Face, Light, Shadow, Ink, Accent, Display, DisplayInk paintengine2d.Color
	Shelf                                                 float32
	Round                                                 bool
}

func rgb(v uint32) paintengine2d.Color {
	return paintengine2d.RGB(float32(v>>16&255)/255, float32(v>>8&255)/255, float32(v&255)/255)
}

var Catalog = []Skin{
	{"zoom-player", "Zoom Player · Onyx", "Inmatrix's original Onyx skin", "2000s", rgb(0x292c31), rgb(0x363b41), rgb(0x101113), rgb(0xbfbfbf), rgb(0xffa803), rgb(0x000000), rgb(0x999c9e), 72, false},
	{"zoom-player-silver", "Zoom Player · Silverchrome", "bLight's original Silverchrome skin", "2000s", rgb(0xbec5d0), rgb(0xf6f8fc), rgb(0x5c6a80), rgb(0x263345), rgb(0x315fbc), rgb(0x182b48), rgb(0xbad9ff), 42, false},
	{"zoom-player-fusion", "Zoom Player · Fusion", "Inmatrix's original Fusion 8.1.5 skin", "8.1.5", rgb(0x2c2f2f), rgb(0x737777), rgb(0x101113), rgb(0xe9e9e9), rgb(0x2c9dff), rgb(0x1a1e23), rgb(0x999c9e), 57, true},
	{"zoom-player-gtz-hd", "Zoom Player · GTZ HD", "Godwin's original GTZ HD skin", "2000s", rgb(0x757575), rgb(0xffffff), rgb(0x303030), rgb(0x000000), rgb(0xfe9000), rgb(0x757575), rgb(0x000000), 72, true},
	{"zoom-player-brownish", "Zoom Player · Brownish", "bLight's original Brownish skin", "2000s", rgb(0xb4968b), rgb(0xd3b5aa), rgb(0x9e8075), rgb(0xffffff), rgb(0x74b482), rgb(0x252c27), rgb(0xffffff), 56, false},
	{"bsplayer", "BS.Player · Base", "TinaZ's original Base skin", "2006", rgb(0xa8afb7), rgb(0xe5e9ee), rgb(0x485566), rgb(0x1f3045), rgb(0x427ab0), rgb(0x0c1921), rgb(0xb3df83), 100, false},
	{"powerdvd", "PowerDVD · 5 Glow", "Alan Yang's original PowerDVD 5 Glow skin", "2003", rgb(0xbac6d3), rgb(0xffffff), rgb(0x567798), rgb(0x172d4d), rgb(0x559dde), rgb(0x0d161e), rgb(0x9be6d5), 124, true},
	{"series-9", "Windows Media Player · 9 Series", "Microsoft's blue and silver 9 Series player", "2003", rgb(0xbfc9da), rgb(0xf3f7ff), rgb(0x5a739d), rgb(0x203d70), rgb(0x346cce), rgb(0x1b3971), rgb(0xd2e7ff), 124, true},
	{"vlc", "VLC · Original Default Skin", "aLtgLasS's original Skins2 default from VLC 0.8.6", "2006", rgb(0xd4d0c8), rgb(0xffffff), rgb(0x808080), rgb(0x202020), rgb(0xd87b20), rgb(0x202020), rgb(0xffffff), 96, false},
	{"quicktime", "QuickTime · iFix", "hills's original iFix skin (Max Rudberg and Rene design)", "2004", rgb(0xa9a9a9), rgb(0xffffff), rgb(0x686868), rgb(0x202020), rgb(0x457bd3), rgb(0xffffff), rgb(0x202020), 89, true},
	{"default", "Default", "Ordinary toolkit controls", "", rgb(0xdfe4ea), rgb(0xffffff), rgb(0x99a2b0), rgb(0x263345), rgb(0x346cce), rgb(0xf8faff), rgb(0x263345), 100, false},
}

var registerOnce sync.Once
var registerError error

//go:embed packs/*/skin.json packs/*/art/*.png
var bundled embed.FS

// Register embeds the original artwork through the toolkit's skin-pack API.
func Register() error {
	registerOnce.Do(func() {
		for _, s := range Catalog {
			if s.ID == "default" {
				continue
			}
			pack, err := fs.Sub(bundled, "packs/"+s.ID)
			if err == nil {
				err = style.RegisterSkinFS(pack, "video-"+s.ID)
			}
			if err != nil {
				registerError = fmt.Errorf("register skin %s: %w", s.ID, err)
				return
			}
		}
	})
	return registerError
}

func Theme(id string) style.ThemeOverride {
	if id == "default" {
		return style.ThemeOverride{}
	}
	return style.ThemeOverride{Pack: "video-" + id}
}

func Find(id string) (Skin, bool) {
	id = strings.ToLower(strings.TrimSpace(id))
	if id == "zoomplayer" || id == "zoom" {
		id = "zoom-player"
	}
	if id == "wmp9" {
		id = "series-9"
	}
	if id == "zoom-player-dark" || id == "onyx" {
		id = "zoom-player"
	}
	if id == "silverchrome" {
		id = "zoom-player-silver"
	}
	if id == "fusion" {
		id = "zoom-player-fusion"
	}
	if id == "gtz-hd" || id == "gtzhd" {
		id = "zoom-player-gtz-hd"
	}
	if id == "brownish" {
		id = "zoom-player-brownish"
	}
	if id == "qt" {
		id = "quicktime"
	}
	for _, s := range Catalog {
		if s.ID == id {
			return s, true
		}
	}
	return Skin{}, false
}
func Choices() string {
	var ids []string
	for _, s := range Catalog {
		ids = append(ids, s.ID)
	}
	return strings.Join(ids, ", ")
}
