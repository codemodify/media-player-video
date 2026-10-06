package player

import (
	"log"

	"github.com/codemodify/uitoolkit/platform"
)

// The tray belongs to the player, so changing skins keeps the same icon and
// callbacks. Its artwork is the same embedded logo used by native windows.
func (p *Player) openTray() {
	if p.closed || p.tray != nil {
		return
	}
	item, err := p.App.NewStatusItem(platform.StatusItemOptions{
		ID: "media-player-video", Title: "Video Player", Tooltip: "Video Player",
		// Plasma resolves theme names before pixmaps. An app-specific name
		// lets it use our artwork instead of a generic application icon.
		Icon:    platform.StatusIcon{Name: "media-player-video", Image: p.logo},
		OnClick: p.showPlayer,
		Menu: []platform.StatusMenuItem{
			{Text: "Show player", OnClick: p.showPlayer},
			{Text: "Quit", OnClick: p.App.Quit},
		},
	})
	if err != nil {
		if item != nil {
			_ = item.Close()
		}
		log.Printf("system tray: %v", err)
		return
	}
	p.tray = item
	item.SetOnShownChange(func(shown bool) {
		if !shown && p.hiddenToTray {
			p.showPlayer()
		}
	})
}

// closePlayer handles both the desktop close request and bitmap close buttons.
// Without a displayed tray icon, exit so the player cannot become unreachable.
func (p *Player) closePlayer() bool {
	if p.closed || p.App.Quitting() {
		return true
	}
	if p.tray == nil || !p.tray.Shown() {
		p.App.Quit()
		return true
	}
	p.hiddenToTray = true
	p.Window.DismissPopup()
	p.Window.HideTooltip()
	p.Window.Hide()
	for _, pane := range []*skinPane{p.controller, p.playlist} {
		if pane != nil {
			pane.w.DismissPopup()
			pane.w.HideTooltip()
		}
	}
	p.syncSkinWindows()
	return false
}

func (p *Player) showPlayer() {
	if p.closed || p.App.Quitting() || p.Window.Closed() {
		return
	}
	p.hiddenToTray = false
	p.Window.Show()
	p.syncSkinWindows()
	for _, pane := range []*skinPane{p.controller, p.playlist} {
		if pane != nil && pane.shown {
			pane.w.Show()
		}
	}
	p.Window.Activate()
}

func (p *Player) closeTray() {
	if p.tray == nil {
		return
	}
	p.tray.SetOnShownChange(nil)
	if err := p.tray.Close(); err != nil {
		log.Printf("close system tray: %v", err)
	}
	p.tray = nil
}
