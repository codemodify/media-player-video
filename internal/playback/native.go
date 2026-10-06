//go:build cgo

package playback

/*
#cgo linux LDFLAGS: -ldl
#include "native.h"
*/
import "C"

import (
	"fmt"
	"image"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"
)

type native struct {
	api                     *C.vp_api
	handle                  *C.mpv_handle
	commands                chan Command
	events                  chan Event
	frames                  chan Frame
	quit                    chan struct{}
	controlDone, renderDone chan struct{}
	renderError             chan error
	once                    sync.Once
	size                    atomic.Uint64
	aspect                  atomic.Uint64
	renderGeneration        atomic.Uint64
}

func New(opts Options) (Engine, error) {
	paths := libraryPaths(opts.Library)
	var api *C.vp_api
	var last string
	for _, path := range paths {
		name := C.CString(path)
		var message [512]C.char
		api = C.vp_open(name, &message[0], C.int(len(message)))
		C.free(unsafe.Pointer(name))
		if api != nil {
			break
		}
		last = C.GoString(&message[0])
	}
	if api == nil {
		return nil, fmt.Errorf("libmpv is unavailable: %s. Install libmpv (mpv on macOS), or set -mpv-library to its full path", last)
	}
	h := C.vp_create(api)
	if h == nil {
		C.vp_close(api)
		return nil, fmt.Errorf("libmpv: could not create decoder")
	}
	p := &native{api: api, handle: h, commands: make(chan Command, 128), events: make(chan Event, 128), frames: make(chan Frame, 1), quit: make(chan struct{}), controlDone: make(chan struct{}), renderDone: make(chan struct{}), renderError: make(chan error, 1)}
	options := [][2]string{{"config", "no"}, {"vo", "libmpv"}, {"idle", "yes"}, {"keep-open", "no"}, {"terminal", "no"}, {"input-default-bindings", "no"}, {"osc", "no"}, {"load-scripts", "no"}, {"sub-auto", "fuzzy"}, {"audio-display", "no"}, {"volume", "70"}}
	if opts.AudioOutput != "" {
		options = append(options, [2]string{"ao", opts.AudioOutput})
	}
	for _, option := range options {
		n, v := C.CString(option[0]), C.CString(option[1])
		code := C.vp_option(api, h, n, v)
		C.free(unsafe.Pointer(n))
		C.free(unsafe.Pointer(v))
		if code < 0 {
			message := C.GoString(C.vp_error(api, code))
			C.vp_destroy(api, h)
			C.vp_close(api)
			return nil, fmt.Errorf("libmpv option %s: %s", option[0], message)
		}
	}
	if code := C.vp_initialize(api, h); code < 0 {
		message := C.GoString(C.vp_error(api, code))
		C.vp_destroy(api, h)
		C.vp_close(api)
		return nil, fmt.Errorf("libmpv initialization: %s", message)
	}
	p.Resize(960, 540)
	ready := make(chan error, 1)
	go p.render(ready)
	if err := <-ready; err != nil {
		<-p.renderDone
		C.vp_destroy(api, h)
		C.vp_close(api)
		return nil, err
	}
	go p.control()
	return p, nil
}

func libraryPaths(explicit string) []string {
	if explicit != "" {
		return []string{explicit}
	}
	if path := os.Getenv("VIDEO_PLAYER_MPV_LIBRARY"); path != "" {
		return []string{path}
	}
	switch runtime.GOOS {
	case "windows":
		paths := []string{"mpv-2.dll", "libmpv-2.dll", "mpv-1.dll"}
		if exe, err := os.Executable(); err == nil {
			for _, n := range append([]string(nil), paths...) {
				paths = append(paths, filepath.Join(filepath.Dir(exe), n))
			}
		}
		return paths
	case "darwin":
		return []string{"libmpv.2.dylib", "/opt/homebrew/lib/libmpv.2.dylib", "/usr/local/lib/libmpv.2.dylib"}
	default:
		return []string{"libmpv.so.2", "libmpv.so.1", "libmpv.so"}
	}
}

func (p *native) Send(c Command) error {
	if len(c.Args) == 0 {
		return fmt.Errorf("empty decoder command")
	}
	for _, arg := range c.Args {
		if strings.IndexByte(arg, 0) >= 0 {
			return fmt.Errorf("decoder argument contains a NUL byte")
		}
	}
	c.Args = append([]string(nil), c.Args...)
	select {
	case <-p.quit:
		return ErrClosed
	default:
	}
	select {
	case <-p.quit:
		return ErrClosed
	case p.commands <- c:
		return nil
	default:
		return fmt.Errorf("decoder command queue is full")
	}
}
func (p *native) Events() <-chan Event { return p.events }
func (p *native) Frames() <-chan Frame { return p.frames }
func (p *native) Resize(w, h int) {
	w, h = max(16, min(w, 3840)), max(16, min(h, 2160))
	// The toolkit scales this software-rendered surface. Bound per-frame CPU cost.
	scale := min(1.0, min(1280.0/float64(w), 720.0/float64(h)))
	w, h = max(16, int(float64(w)*scale)), max(16, int(float64(h)*scale))
	p.size.Store(uint64(uint32(w))<<32 | uint64(uint32(h)))
}
func (p *native) Close() error {
	p.once.Do(func() {
		close(p.quit)
		<-p.controlDone
		<-p.renderDone
		C.vp_destroy(p.api, p.handle)
		C.vp_close(p.api)
	})
	return nil
}
func (p *native) command(args []string) error {
	ptr := C.calloc(C.size_t(len(args)+1), C.size_t(unsafe.Sizeof(uintptr(0))))
	if ptr == nil {
		return fmt.Errorf("decoder: out of memory")
	}
	defer C.free(ptr)
	values := unsafe.Slice((**C.char)(ptr), len(args)+1)
	for i, arg := range args {
		values[i] = C.CString(arg)
		defer C.free(unsafe.Pointer(values[i]))
	}
	if code := C.vp_command(p.api, p.handle, (**C.char)(ptr)); code < 0 {
		return fmt.Errorf("%s: %s", args[0], C.GoString(C.vp_error(p.api, code)))
	}
	return nil
}
func (p *native) number(name string, fallback float64) float64 {
	n := C.CString(name)
	defer C.free(unsafe.Pointer(n))
	return float64(C.vp_number(p.api, p.handle, n, C.double(fallback)))
}
func (p *native) integer(name string, fallback int64) int64 {
	n := C.CString(name)
	defer C.free(unsafe.Pointer(n))
	return int64(C.vp_integer(p.api, p.handle, n, C.int64_t(fallback)))
}
func (p *native) flag(name string, fallback bool) bool {
	n := C.CString(name)
	defer C.free(unsafe.Pointer(n))
	v := 0
	if fallback {
		v = 1
	}
	return C.vp_flag(p.api, p.handle, n, C.int(v)) != 0
}
func (p *native) text(name string) string {
	n := C.CString(name)
	defer C.free(unsafe.Pointer(n))
	v := C.vp_string(p.api, p.handle, n)
	if v == nil {
		return ""
	}
	defer C.vp_free(p.api, unsafe.Pointer(v))
	return C.GoString(v)
}
func (p *native) tracks() []Track {
	n := C.vp_tracks(p.api, p.handle)
	if n == nil {
		return nil
	}
	defer C.vp_tracks_free(p.api, n)
	var result []Track
	field := func(i C.int, name string) string {
		f := C.CString(name)
		defer C.free(unsafe.Pointer(f))
		return C.GoString(C.vp_track_text(n, i, f))
	}
	for i := C.int(0); i < C.vp_track_count(n); i++ {
		result = append(result, Track{ID: int(C.vp_track_id(n, i)), Kind: field(i, "type"), Title: field(i, "title"), Language: field(i, "lang"), Selected: C.vp_track_selected(n, i) != 0})
	}
	return result
}
func (p *native) snapshot(generation uint64, loaded bool) Snapshot {
	w, h := int(p.number("video-params/dw", 0)), int(p.number("video-params/dh", 0))
	if loaded && w > 0 && h > 0 {
		p.aspect.Store(uint64(uint32(w))<<32 | uint64(uint32(h)))
	}
	return Snapshot{Generation: generation, Path: p.text("path"), Position: p.number("time-pos", 0), Duration: p.number("duration", 0), Volume: p.number("volume", 70), Speed: p.number("speed", 1), Paused: p.flag("pause", false), Idle: p.flag("idle-active", true), Muted: p.flag("mute", false), Loaded: loaded, Width: w, Height: h, Tracks: p.tracks()}
}
func (p *native) publish(event Event, critical bool) {
	if critical {
		select {
		case p.events <- event:
		case <-p.quit:
		}
		return
	}
	select {
	case p.events <- event:
	default:
	}
}

func (p *native) control() {
	defer close(p.controlDone)
	defer close(p.events)
	tick := time.NewTicker(20 * time.Millisecond)
	defer tick.Stop()
	var generation uint64
	entries := map[int64]uint64{}
	loaded := false
	lastPoll := time.Time{}
	drain := func() {
		for {
			e := C.vp_event(p.api, p.handle)
			if e.event_id == C.MPV_EVENT_NONE {
				break
			}
			switch e.event_id {
			case C.MPV_EVENT_START_FILE:
				start := (*C.mpv_event_start_file)(e.data)
				generation = entries[int64(start.playlist_entry_id)]
				loaded = false
				p.aspect.Store(0)
				p.renderGeneration.Store(0)
			case C.MPV_EVENT_FILE_LOADED:
				loaded = true
				snapshot := p.snapshot(generation, true)
				p.renderGeneration.Store(generation)
				p.publish(Event{Snapshot: snapshot}, true)
			case C.MPV_EVENT_END_FILE:
				end := (*C.mpv_event_end_file)(e.data)
				id := int64(end.playlist_entry_id)
				gen := entries[id]
				delete(entries, id)
				if gen == generation {
					loaded = false
					p.renderGeneration.Store(0)
				}
				if end.reason == C.MPV_END_FILE_REASON_EOF {
					p.publish(Event{Snapshot: Snapshot{Generation: gen}, Ended: true}, true)
				}
				if end.reason == C.MPV_END_FILE_REASON_ERROR {
					p.publish(Event{Snapshot: Snapshot{Generation: gen, Idle: true, Error: C.GoString(C.vp_error(p.api, end.error))}}, true)
				}
			case C.MPV_EVENT_SHUTDOWN:
				p.publish(Event{Snapshot: Snapshot{Generation: generation, Error: "The video decoder shut down"}}, true)
			}
		}
	}
	for {
		select {
		case <-p.quit:
			return
		case c := <-p.commands:
			drain()
			loading := c.Args[0] == "loadfile"
			if loading {
				p.renderGeneration.Store(0)
			}
			if err := p.command(c.Args); err != nil {
				gen := generation
				if c.Generation != 0 {
					gen = c.Generation
				}
				p.publish(Event{Snapshot: Snapshot{Generation: gen, Error: err.Error()}}, true)
			} else if loading {
				// loadfile replace assigns the single queue entry synchronously.
				// Correlate its native ID, rather than counting start-file events:
				// rapid replacements can skip starting intermediate files entirely.
				id := p.integer("playlist/0/id", -1)
				if id < 0 {
					p.publish(Event{Snapshot: Snapshot{Generation: c.Generation, Error: "libmpv could not identify the loaded queue entry"}}, true)
				} else {
					entries[id] = c.Generation
				}
			}
		case err := <-p.renderError:
			p.publish(Event{Snapshot: Snapshot{Generation: generation, Error: err.Error()}}, true)
		case <-tick.C:
			drain()
			if time.Since(lastPoll) >= 100*time.Millisecond {
				lastPoll = time.Now()
				p.publish(Event{Snapshot: p.snapshot(generation, loaded)}, false)
			}
		}
	}
}

func (p *native) render(ready chan<- error) {
	// This thread only uses the render API. Normal client calls live in control.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	defer close(p.renderDone)
	defer close(p.frames)
	var context *C.mpv_render_context
	if code := C.vp_render_create(p.api, p.handle, &context); code < 0 {
		ready <- fmt.Errorf("libmpv software video renderer: %s", C.GoString(C.vp_error(p.api, code)))
		return
	}
	defer C.vp_render_free(p.api, context)
	ready <- nil
	var pixels unsafe.Pointer
	defer func() { C.free(pixels) }()
	var size, previousGeneration uint64
	var w, h, stride int
	tick := time.NewTicker(8 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-p.quit:
			return
		case <-tick.C:
		}
		gen := p.renderGeneration.Load()
		want := p.size.Load()
		if aspect := p.aspect.Load(); aspect != 0 {
			aw, ah := float64(aspect>>32), float64(uint32(aspect))
			tw, th := int(want>>32), int(uint32(want))
			scale := min(float64(tw)/aw, float64(th)/ah)
			tw, th = max(1, int(aw*scale)), max(1, int(ah*scale))
			want = uint64(uint32(tw))<<32 | uint64(uint32(th))
		}
		changed := want != size || gen != previousGeneration
		if want != size {
			C.free(pixels)
			w, h = int(want>>32), int(uint32(want))
			stride = (w*4 + 63) &^ 63
			pixels = C.malloc(C.size_t(stride * h))
			size = want
			if pixels == nil {
				select {
				case p.renderError <- fmt.Errorf("video renderer: out of memory"):
				default:
				}
				return
			}
		}
		previousGeneration = gen
		update := C.vp_render_update(p.api, context) != 0
		if !changed && !update {
			continue
		}
		if code := C.vp_render(p.api, context, C.int(w), C.int(h), C.size_t(stride), pixels); code < 0 {
			select {
			case p.renderError <- fmt.Errorf("video rendering: %s", C.GoString(C.vp_error(p.api, code))):
			default:
			}
			continue
		}
		if gen == 0 || gen != p.renderGeneration.Load() {
			continue
		}
		data := C.GoBytes(pixels, C.int(stride*h))
		for i := 3; i < len(data); i += 4 {
			data[i] = 255
		}
		frame := Frame{Generation: gen, Image: &image.NRGBA{Pix: data, Stride: stride, Rect: image.Rect(0, 0, w, h)}}
		select {
		case <-p.frames:
		default:
		}
		select {
		case p.frames <- frame:
		default:
		}
	}
}
