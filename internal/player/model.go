// Package player owns the queue, session, commands and toolkit presentation.
package player

import (
	"math/rand"
	"path/filepath"
	"strings"

	"github.com/codemodify/media-player-video/internal/playback"
)

type Video struct {
	Path     string  `json:"path"`
	Duration float64 `json:"duration,omitempty"`
}

func (v Video) Title() string { return strings.TrimSuffix(filepath.Base(v.Path), filepath.Ext(v.Path)) }

type Model struct {
	Queue                   []Video          `json:"queue"`
	Index                   int              `json:"index"`
	Position                float64          `json:"position"`
	Volume                  float64          `json:"volume"`
	Speed                   float64          `json:"speed"`
	Muted                   bool             `json:"muted"`
	Repeat                  int              `json:"repeat"` // 0 off, 1 entire queue, 2 current video
	Shuffle                 bool             `json:"shuffle"`
	Skin                    string           `json:"skin"`
	Playlist                bool             `json:"playlist"`
	Width                   int              `json:"width,omitempty"`
	Height                  int              `json:"height,omitempty"`
	Paused, Stopped, Loaded bool             `json:"-"`
	Generation              uint64           `json:"-"`
	VideoWidth, VideoHeight int              `json:"-"`
	Tracks                  []playback.Track `json:"-"`
	Error                   string           `json:"-"`
}

func FreshModel() *Model {
	return &Model{Index: -1, Volume: 70, Speed: 1, Skin: "zoom-player", Playlist: false, Width: 638, Height: 454, Stopped: true}
}
func (m *Model) Current() Video {
	if m.Index < 0 || m.Index >= len(m.Queue) {
		return Video{}
	}
	return m.Queue[m.Index]
}
func (m *Model) Fraction() float32 {
	if d := m.Current().Duration; d > 0 {
		return float32(min(1, max(0, m.Position/d)))
	}
	return 0
}
func (m *Model) State() string {
	if m.Error != "" {
		return m.Error
	}
	if m.Stopped {
		return "Stopped"
	}
	if !m.Loaded {
		return "Opening video…"
	}
	if m.Paused {
		return "Paused"
	}
	return "Playing"
}

// Apply rejects old loads, including a previous load of the same pathname.
func (m *Model) Apply(s playback.Snapshot) bool {
	if s.Generation == 0 || s.Generation != m.Generation {
		return false
	}
	if s.Error != "" {
		m.Error = s.Error
		if !m.Loaded || s.Idle {
			m.Stopped = true
			m.Loaded = false
		}
		return true
	}
	if !s.Loaded || m.Stopped {
		return false
	}
	if s.Path != "" && filepath.Clean(s.Path) != filepath.Clean(m.Current().Path) {
		return false
	}
	m.Loaded = true
	m.Paused = s.Paused
	m.Position = max(0, s.Position)
	if s.Duration > 0 && m.Index >= 0 && m.Index < len(m.Queue) {
		m.Queue[m.Index].Duration = s.Duration
	}
	m.VideoWidth, m.VideoHeight = s.Width, s.Height
	m.Tracks = s.Tracks
	return true
}

// NextIndex gives natural EOF and manual navigation different wrap behavior.
func (m *Model) NextIndex(natural bool) int {
	if len(m.Queue) == 0 {
		return -1
	}
	if natural && m.Repeat == 2 {
		return m.Index
	}
	if m.Shuffle && len(m.Queue) > 1 {
		n := rand.Intn(len(m.Queue) - 1)
		if n >= m.Index {
			n++
		}
		return n
	}
	n := m.Index + 1
	if n < len(m.Queue) {
		return n
	}
	if !natural || m.Repeat == 1 {
		return 0
	}
	return -1
}
