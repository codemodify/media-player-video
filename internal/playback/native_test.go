//go:build cgo

package playback

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func videoFixture(t *testing.T, seconds string) string {
	t.Helper()
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("integration test requires ffmpeg")
	}
	path := filepath.Join(t.TempDir(), "a video with spaces.mp4")
	out, err := exec.Command(ffmpeg, "-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i", "testsrc2=size=160x90:rate=24", "-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000", "-t", seconds, "-c:v", "mpeg4", "-c:a", "aac", "-y", path).CombinedOutput()
	if err != nil {
		t.Fatalf("fixture: %v: %s", err, out)
	}
	return path
}
func waitEvent(t *testing.T, e Engine, predicate func(Event) bool) Event {
	t.Helper()
	timer := time.NewTimer(8 * time.Second)
	defer timer.Stop()
	for {
		select {
		case event, ok := <-e.Events():
			if !ok {
				t.Fatal("decoder closed")
			}
			if event.Snapshot.Error != "" {
				t.Fatalf("decoder: %s", event.Snapshot.Error)
			}
			if predicate(event) {
				return event
			}
		case <-timer.C:
			t.Fatal("timed out waiting for decoder event")
		}
	}
}

func TestEmbeddedPlayback(t *testing.T) {
	path := videoFixture(t, "6")
	e, err := New(Options{AudioOutput: "null"})
	if err != nil {
		t.Skip(err)
	}
	defer e.Close()
	// A square viewport must still produce a 16:9 frame for contain/crop/stretch.
	e.Resize(320, 320)
	send := func(generation uint64, args ...string) {
		t.Helper()
		if err := e.Send(Command{Args: args, Generation: generation}); err != nil {
			t.Fatal(err)
		}
	}
	send(1, "loadfile", path, "replace")
	loaded := waitEvent(t, e, func(v Event) bool { return v.Snapshot.Generation == 1 && v.Snapshot.Loaded }).Snapshot
	if loaded.Duration < 5.5 || loaded.Width != 160 || loaded.Height != 90 {
		t.Fatalf("metadata: %+v", loaded)
	}
	// Opening a render context can produce an initial black redraw before the
	// first decoded picture. Wait for an actual frame rather than that redraw.
	frameDeadline := time.NewTimer(5 * time.Second)
	defer frameDeadline.Stop()
	decoded := false
	for !decoded {
		select {
		case frame, ok := <-e.Frames():
			if !ok {
				t.Fatal("frame stream closed")
			}
			if frame.Generation != 1 {
				continue
			}
			if frame.Image.Rect.Dx() != 320 || frame.Image.Rect.Dy() != 180 {
				t.Fatalf("unexpected frame size: %v", frame.Image.Rect)
			}
			for i := 0; i < len(frame.Image.Pix); i += 4 {
				if frame.Image.Pix[i] != frame.Image.Pix[i+1] || frame.Image.Pix[i+1] != frame.Image.Pix[i+2] {
					decoded = true
					break
				}
			}
		case <-frameDeadline.C:
			t.Fatal("no decoded embedded frame")
		}
	}
	send(1, "set", "pause", "yes")
	paused := waitEvent(t, e, func(v Event) bool { return v.Snapshot.Loaded && v.Snapshot.Paused }).Snapshot.Position
	time.Sleep(180 * time.Millisecond)
	steady := waitEvent(t, e, func(v Event) bool { return v.Snapshot.Loaded && v.Snapshot.Paused }).Snapshot.Position
	if steady-paused > .15 {
		t.Fatalf("pause moved clock: %f -> %f", paused, steady)
	}
	send(1, "seek", "2.5", "absolute+exact")
	waitEvent(t, e, func(v Event) bool {
		return v.Snapshot.Loaded && v.Snapshot.Position >= 2.4 && v.Snapshot.Position <= 2.7
	})
	send(1, "set", "volume", "35")
	send(1, "set", "speed", "1.5")
	send(1, "set", "mute", "yes")
	settings := waitEvent(t, e, func(v Event) bool {
		return v.Snapshot.Loaded && v.Snapshot.Volume == 35 && v.Snapshot.Speed == 1.5 && v.Snapshot.Muted
	})
	if len(settings.Snapshot.Tracks) < 2 {
		t.Fatalf("missing audio/video track catalog: %+v", settings)
	}
	subtitle := filepath.Join(t.TempDir(), "a subtitle with spaces.srt")
	if err = os.WriteFile(subtitle, []byte("1\n00:00:00,000 --> 00:00:06,000\nSubtitle fixture\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	send(1, "sub-add", subtitle, "select")
	waitEvent(t, e, func(v Event) bool {
		for _, track := range v.Snapshot.Tracks {
			if track.Kind == "sub" && track.Selected {
				return true
			}
		}
		return false
	})
	send(1, "set", "sid", "no")
	waitEvent(t, e, func(v Event) bool {
		found := false
		for _, track := range v.Snapshot.Tracks {
			if track.Kind == "sub" {
				found = true
				if track.Selected {
					return false
				}
			}
		}
		return found
	})
	// Replacement and Stop are not natural EOF, even for the same pathname.
	send(2, "loadfile", path, "replace")
	waitEvent(t, e, func(v Event) bool { return v.Snapshot.Generation == 2 && v.Snapshot.Loaded })
	send(2, "stop")
	timer := time.NewTimer(300 * time.Millisecond)
	defer timer.Stop()
	for {
		select {
		case event := <-e.Events():
			if event.Ended {
				t.Fatal("stop or replacement reported natural EOF")
			}
		case <-timer.C:
			return
		}
	}
}

func TestNaturalEOFAndMissingFile(t *testing.T) {
	path := videoFixture(t, "0.6")
	e, err := New(Options{AudioOutput: "null"})
	if err != nil {
		t.Skip(err)
	}
	defer e.Close()
	if err = e.Send(Command{Generation: 41, Args: []string{"loadfile", path, "replace"}}); err != nil {
		t.Fatal(err)
	}
	waitEvent(t, e, func(v Event) bool { return v.Ended && v.Snapshot.Generation == 41 })
	missing := filepath.Join(t.TempDir(), "missing.mp4")
	if err = e.Send(Command{Generation: 42, Args: []string{"loadfile", missing, "replace"}}); err != nil {
		t.Fatal(err)
	}
	for {
		select {
		case v := <-e.Events():
			if v.Snapshot.Generation == 42 && v.Snapshot.Error != "" {
				return
			}
		case <-time.After(5 * time.Second):
			t.Fatal("missing file did not report a load error")
		}
	}
}

func TestBadLibrary(t *testing.T) {
	if _, err := New(Options{Library: filepath.Join(t.TempDir(), "not-a-library")}); err == nil {
		t.Fatal("missing library accepted")
	}
	if os.Getenv("VIDEO_PLAYER_MPV_LIBRARY") != "" {
		t.Log("explicit library paths take precedence over the environment")
	}
}

func TestRapidReplacementUsesLatestGeneration(t *testing.T) {
	path := videoFixture(t, "3")
	e, err := New(Options{AudioOutput: "null"})
	if err != nil {
		t.Skip(err)
	}
	defer e.Close()
	for generation := uint64(1); generation <= 20; generation++ {
		if err = e.Send(Command{Generation: generation, Args: []string{"loadfile", path, "replace"}}); err != nil {
			t.Fatal(err)
		}
	}
	waitEvent(t, e, func(v Event) bool { return v.Snapshot.Generation == 20 && v.Snapshot.Loaded })
}
