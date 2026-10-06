package player

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/codemodify/media-player-video/internal/playback"
)

func TestStaleLoadsAndEOFPolicy(t *testing.T) {
	m := FreshModel()
	m.Queue = []Video{{Path: "/a.mp4"}, {Path: "/b.mkv"}}
	m.Index = 1
	m.Generation = 9
	m.Stopped = false
	if m.Apply(playback.Snapshot{Generation: 8, Path: "/b.mkv", Loaded: true, Position: 90, Error: "stale failure"}) {
		t.Fatal("old load accepted")
	}
	if m.Apply(playback.Snapshot{Generation: 9, Path: "/a.mp4", Loaded: true, Position: 90}) {
		t.Fatal("wrong file accepted")
	}
	if !m.Apply(playback.Snapshot{Generation: 9, Path: "/b.mkv", Loaded: true, Position: 4, Duration: 100, Width: 1920, Height: 1080}) {
		t.Fatal("current load rejected")
	}
	if m.Position != 4 || m.Current().Duration != 100 || !m.Loaded {
		t.Fatalf("snapshot not applied: %+v", m)
	}
	if m.NextIndex(true) != -1 {
		t.Fatal("EOF wrapped with repeat off")
	}
	m.Repeat = 1
	if m.NextIndex(true) != 0 {
		t.Fatal("repeat queue did not wrap")
	}
	m.Repeat = 2
	if m.NextIndex(true) != 1 {
		t.Fatal("repeat video did not retain current video")
	}
	m.Repeat = 0
	m.Shuffle = true
	for i := 0; i < 20; i++ {
		if m.NextIndex(false) != 0 {
			t.Fatal("shuffle repeated current item despite another choice")
		}
	}
}
func touch(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
}
func TestFolderExpansionIsOrderedAndTransactional(t *testing.T) {
	root := t.TempDir()
	touch(t, filepath.Join(root, "z.mp4"))
	touch(t, filepath.Join(root, "a.MKV"))
	touch(t, filepath.Join(root, "nested", "b.webm"))
	touch(t, filepath.Join(root, "cover.jpg"))
	touch(t, filepath.Join(root, "subtitle.srt"))
	if err := os.Symlink(root, filepath.Join(root, "nested", "loop")); err != nil {
		t.Log("symlink unavailable:", err)
	}
	videos, err := Expand(context.Background(), []string{root})
	if err != nil {
		t.Fatal(err)
	}
	if len(videos) != 3 || videos[0].Title() != "a" || videos[1].Title() != "b" || videos[2].Title() != "z" {
		t.Fatalf("folder order: %+v", videos)
	}
	if videos, err := Expand(context.Background(), []string{root, filepath.Join(root, "missing.mp4")}); err == nil || videos != nil {
		t.Fatal("partially successful request committed")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Expand(ctx, []string{root}); err == nil {
		t.Fatal("cancelled scan succeeded")
	}
}
func TestSessionRestoresStoppedAndDropsMissingFiles(t *testing.T) {
	root := t.TempDir()
	first, second := filepath.Join(root, "a.mp4"), filepath.Join(root, "b.mp4")
	touch(t, first)
	touch(t, second)
	m := FreshModel()
	m.Queue = []Video{{Path: first}, {Path: filepath.Join(root, "gone.mp4")}, {Path: second}}
	m.Index = 2
	m.Position = 45
	m.Volume = 35
	m.Speed = 1.5
	m.Skin = "zoom-player-dark"
	m.Width = 1200
	m.Height = 750
	m.Stopped = false
	m.Loaded = true
	path := filepath.Join(root, "config", "session.json")
	if err := SaveSession(path, m); err != nil {
		t.Fatal(err)
	}
	restored, err := ReadSession(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(restored.Queue) != 2 || restored.Index != 1 || restored.Position != 45 || restored.Volume != 35 || restored.Speed != 1.5 || restored.Width != 1200 || restored.Height != 750 || !restored.Stopped || restored.Loaded {
		t.Fatalf("restored session: %+v", restored)
	}
	data, _ := os.ReadFile(path)
	var saved map[string]any
	if err = json.Unmarshal(data, &saved); err != nil {
		t.Fatal(err)
	}
	if _, ok := saved["Generation"]; ok {
		t.Fatal("transient decoder state persisted")
	}
}
