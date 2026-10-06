package player

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"

	"github.com/codemodify/media-player-video/internal/skins"
)

func SessionPath() string {
	root, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(root, "media-player-video", "session.json")
}
func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

func ReadSession(path string) (*Model, error) {
	m := FreshModel()
	if path == "" {
		return m, nil
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return m, nil
	}
	if err != nil {
		return m, err
	}
	if err = json.Unmarshal(data, m); err != nil {
		return FreshModel(), fmt.Errorf("read session: %w", err)
	}
	if _, ok := skins.Find(m.Skin); !ok {
		m.Skin = "zoom-player"
	}
	if !finite(m.Volume) {
		m.Volume = 70
	}
	m.Volume = min(100, max(0, m.Volume))
	if !finite(m.Speed) || m.Speed < 0.25 || m.Speed > 4 {
		m.Speed = 1
	}
	if !finite(m.Position) || m.Position < 0 {
		m.Position = 0
	}
	if m.Repeat < 0 || m.Repeat > 2 {
		m.Repeat = 0
	}
	w, h := skins.Size(m.Skin)
	m.Width, m.Height = min(3840, max(w, m.Width)), min(2160, max(h, m.Height))
	oldIndex := m.Index
	m.Index = -1
	queue := make([]Video, 0, len(m.Queue))
	for i, v := range m.Queue {
		if info, err := os.Stat(v.Path); err == nil && info.Mode().IsRegular() {
			if i == oldIndex {
				m.Index = len(queue)
			}
			if !finite(v.Duration) || v.Duration < 0 {
				v.Duration = 0
			}
			queue = append(queue, v)
		}
	}
	m.Queue = queue
	if m.Index < 0 && len(queue) > 0 {
		m.Index = 0
		m.Position = 0
	}
	if len(queue) == 0 {
		m.Position = 0
	}
	m.Stopped = true
	m.Paused = false
	m.Loaded = false
	return m, nil
}
func SaveSession(path string, m *Model) error {
	if path == "" {
		return nil
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".session-*")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if _, err = f.Write(data); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(name, path)
}
