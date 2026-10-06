package player

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/codemodify/media-player-video/internal/playback"
	"github.com/codemodify/media-player-video/internal/skins"
	"github.com/codemodify/paintengine2d"
	"github.com/codemodify/uitoolkit/app"
	"github.com/codemodify/uitoolkit/platform"
)

type Options struct {
	Headless                   bool
	Skin, SessionPath, Library string
	Engine                     playback.Engine // Test injection; all skins use this same engine.
}
type fileRequest struct {
	paths  []string
	append bool
}
type fileResult struct {
	videos []Video
	append bool
	err    error
}

type Player struct {
	App                  *app.Application
	Window               *app.Window
	Model                *Model
	Skin                 skins.Skin
	logo                 *paintengine2d.Image
	tray                 platform.StatusItem
	engine               playback.Engine
	body                 *body
	controller, playlist *skinPane
	opts                 Options
	stopTimer            func()
	closed               bool
	loadingEngine        bool
	engineReady          chan engineResult
	files                chan fileRequest
	fileResults          chan fileResult
	ctx                  context.Context
	cancel               context.CancelFunc
	workers              sync.WaitGroup
	seekOnLoad           float64
	seekPending          bool
	lastSave             time.Time
	fullscreen           bool
	hiddenToTray         bool
	fit                  int
}
type engineResult struct {
	engine playback.Engine
	err    error
}

func New(a *app.Application, opts Options) (*Player, error) {
	m := FreshModel()
	if !opts.Headless {
		var err error
		m, err = ReadSession(opts.SessionPath)
		if err != nil {
			log.Print(err)
		}
	}
	if opts.Skin != "" {
		m.Skin = opts.Skin
	}
	skin, ok := skins.Find(m.Skin)
	if !ok {
		return nil, fmt.Errorf("unknown skin %q; choose %s", m.Skin, skins.Choices())
	}
	m.Skin = skin.ID
	if err := skins.Register(); err != nil {
		return nil, err
	}
	a.SetTheme(skins.Theme(skin.ID))
	ctx, cancel := context.WithCancel(context.Background())
	p := &Player{App: a, Model: m, Skin: skin, opts: opts, engine: opts.Engine, ctx: ctx, cancel: cancel, engineReady: make(chan engineResult, 1), files: make(chan fileRequest, 32), fileResults: make(chan fileResult, 32)}
	for _, candidate := range a.Icon() {
		if candidate != nil && candidate.Width > 0 && candidate.Width == candidate.Height &&
			(p.logo == nil || candidate.Width > p.logo.Width) {
			p.logo = candidate
		}
	}
	if err := p.createPlayerWindow(); err != nil {
		cancel()
		return nil, err
	}
	if !opts.Headless && opts.Skin == "" {
		p.Window.SetSize(m.Width, m.Height)
	}
	p.refresh()
	p.workers.Add(1)
	go p.fileWorker()
	if !opts.Headless {
		p.openTray()
		p.lastSave = time.Now()
		p.arm()
	}
	return p, nil
}
func (p *Player) SetSkin(id string) error {
	skin, ok := skins.Find(id)
	if !ok {
		return fmt.Errorf("unknown skin %q", id)
	}
	if p.Skin.ID == skin.ID {
		return nil
	}
	frame, selected := p.body.video.frame, p.body.queue.Selected
	oldWindow := p.Window
	if p.stopTimer != nil {
		p.stopTimer()
	}
	p.closeSkinWindows()
	p.fullscreen = false
	p.Skin = skin
	p.Model.Skin = skin.ID
	p.App.SetTheme(skins.Theme(skin.ID))
	if err := p.createPlayerWindow(); err != nil {
		return err
	}
	p.body.video.frame, p.body.queue.Selected = frame, selected
	oldWindow.Close()
	if !p.opts.Headless {
		p.arm()
	}
	p.refresh()
	p.Window.RequestLayout()
	return nil
}
func (p *Player) refresh() {
	if p.closed {
		return
	}
	p.body.sync()
	p.body.Invalidate()
}
func (p *Player) arm() {
	p.stopTimer = p.Window.AfterFunc(16*time.Millisecond, func() {
		if p.closed {
			return
		}
		p.Pump()
		p.arm()
	})
}

// Pump is called only on the UI thread. Worker channels never mutate widgets.
func (p *Player) Pump() {
	select {
	case ready := <-p.engineReady:
		p.loadingEngine = false
		if ready.err != nil {
			p.Model.Error = ready.err.Error()
			p.Model.Stopped = true
			p.refresh()
		} else {
			p.engine = ready.engine
			if !p.Model.Stopped {
				p.loadCurrent()
			}
		}
	default:
	}
	for n := 0; n < 32; n++ {
		select {
		case r := <-p.fileResults:
			if r.err != nil {
				p.fail(r.err)
			} else {
				p.commit(r.videos, r.append)
			}
		default:
			n = 32
		}
	}
	if p.engine != nil {
		for n := 0; n < 128; n++ {
			select {
			case event, ok := <-p.engine.Events():
				if !ok {
					p.fail(fmt.Errorf("video decoder stopped"))
					p.engine.Close()
					p.engine = nil
					n = 128
					break
				}
				m := p.Model
				if event.Snapshot.Generation != m.Generation {
					continue
				}
				if event.Ended {
					if !m.Stopped {
						if next := m.NextIndex(true); next >= 0 {
							p.PlayIndex(next)
						} else {
							m.Stopped = true
							m.Loaded = false
							m.Position = m.Current().Duration
							p.refresh()
						}
					}
					continue
				}
				if m.Apply(event.Snapshot) {
					if event.Snapshot.Error != "" {
						log.Print("playback: ", event.Snapshot.Error)
					}
					if event.Snapshot.Loaded && p.seekPending {
						position := p.seekOnLoad
						p.seekPending = false
						p.seekOnLoad = 0
						if position > 0 {
							p.send("seek", strconv.FormatFloat(position, 'f', 3, 64), "absolute+exact")
							m.Position = position
						}
					}
					p.refresh()
				}
			default:
				n = 128
			}
		}
		if p.engine != nil {
			select {
			case frame, ok := <-p.engine.Frames():
				if ok && frame.Generation == p.Model.Generation && !p.Model.Stopped {
					p.body.video.SetFrame(frame.Image)
				}
			default:
			}
			box := p.body.video.LocalBounds()
			if box.Dx() > 0 && box.Dy() > 0 {
				p.engine.Resize(int(box.Dx()), int(box.Dy()))
			}
		}
	}
	if !p.opts.Headless && time.Since(p.lastSave) > 5*time.Second {
		p.save()
		p.lastSave = time.Now()
	}
}
func (p *Player) Close() {
	if p.closed {
		return
	}
	p.closed = true
	p.closeTray()
	if p.stopTimer != nil {
		p.stopTimer()
	}
	p.cancel()
	p.workers.Wait()
	select {
	case ready := <-p.engineReady:
		if ready.engine != nil {
			ready.engine.Close()
		}
	default:
	}
	if p.engine != nil {
		p.engine.Close()
	}
	if !p.opts.Headless {
		p.save()
	}
	p.Window.Close()
	p.closeSkinWindows()
}
func (p *Player) save() {
	if p.opts.Headless {
		return
	}
	if !p.fullscreen && !p.Window.WindowState().Maximized {
		p.Model.Width, p.Model.Height = p.Window.Size()
	}
	if err := SaveSession(p.opts.SessionPath, p.Model); err != nil {
		log.Print("save session: ", err)
	}
}
func (p *Player) ensureEngine() {
	if p.engine != nil {
		p.loadCurrent()
		return
	}
	if p.loadingEngine || p.opts.Headless {
		return
	}
	p.loadingEngine = true
	p.workers.Add(1)
	go func() {
		defer p.workers.Done()
		engine, err := playback.New(playback.Options{Library: p.opts.Library})
		if p.ctx.Err() != nil {
			if engine != nil {
				engine.Close()
			}
			return
		}
		p.engineReady <- engineResult{engine, err}
	}()
}
func (p *Player) send(args ...string) bool {
	if p.engine == nil {
		return false
	}
	if err := p.engine.Send(playback.Command{Args: args, Generation: p.Model.Generation}); err != nil {
		p.fail(err)
		return false
	}
	return true
}
func (p *Player) fail(err error) { p.Model.Error = err.Error(); log.Print(err); p.refresh() }
func (p *Player) loadCurrent() {
	m := p.Model
	if m.Current().Path == "" {
		return
	}
	p.send("set", "volume", strconv.FormatFloat(m.Volume, 'f', 2, 64))
	p.send("set", "speed", strconv.FormatFloat(m.Speed, 'f', 2, 64))
	p.send("set", "mute", yesNo(m.Muted))
	p.send("set", "pause", yesNo(m.Paused))
	p.seekOnLoad = m.Position
	p.seekPending = true
	p.send("loadfile", m.Current().Path, "replace")
}
func yesNo(on bool) string {
	if on {
		return "yes"
	}
	return "no"
}
func (p *Player) PlayIndex(index int) {
	m := p.Model
	if index < 0 || index >= len(m.Queue) {
		return
	}
	m.Index = index
	m.Position = 0
	m.Generation++
	m.Stopped = false
	m.Paused = false
	m.Loaded = false
	m.Error = ""
	m.Tracks = nil
	m.VideoWidth = 0
	m.VideoHeight = 0
	p.body.video.Clear()
	p.body.queue.Selected = index
	p.ensureEngine()
	p.refresh()
}
func (p *Player) PlayPause() {
	m := p.Model
	if len(m.Queue) == 0 {
		p.OpenFiles(false, false)
		return
	}
	if m.Stopped {
		position := m.Position
		if duration := m.Current().Duration; duration > 0 && position >= duration {
			position = 0
		}
		index := m.Index
		if index < 0 {
			index = 0
		}
		m.Index = index
		m.Generation++
		m.Stopped = false
		m.Paused = false
		m.Loaded = false
		m.Error = ""
		m.Position = position
		p.ensureEngine()
	} else {
		m.Paused = !m.Paused
		p.send("set", "pause", yesNo(m.Paused))
	}
	p.refresh()
}
func (p *Player) Stop() {
	p.Model.Generation++
	p.send("stop")
	p.Model.Stopped = true
	p.Model.Loaded = false
	p.Model.Position = 0
	p.Model.Tracks = nil
	p.Model.Error = ""
	p.seekPending = false
	p.body.video.Clear()
	p.refresh()
}
func (p *Player) Seek(seconds float64) {
	m := p.Model
	if m.Stopped || !m.Loaded {
		return
	}
	m.Position = max(0, seconds)
	if d := m.Current().Duration; d > 0 {
		m.Position = min(d, m.Position)
	}
	p.send("seek", strconv.FormatFloat(m.Position, 'f', 3, 64), "absolute+exact")
	p.refresh()
}
func (p *Player) Volume(value float64) {
	p.Model.Volume = min(100, max(0, value))
	p.send("set", "volume", strconv.FormatFloat(p.Model.Volume, 'f', 2, 64))
	p.refresh()
}
func (p *Player) Mute() {
	p.Model.Muted = !p.Model.Muted
	p.send("set", "mute", yesNo(p.Model.Muted))
	p.refresh()
}
func (p *Player) Speed(value float64) {
	p.Model.Speed = min(4, max(0.25, value))
	p.send("set", "speed", strconv.FormatFloat(p.Model.Speed, 'f', 2, 64))
	p.refresh()
}
func (p *Player) Previous() {
	if p.Model.Position > 3 && !p.Model.Stopped {
		p.Seek(0)
		return
	}
	if len(p.Model.Queue) > 0 {
		p.PlayIndex((p.Model.Index - 1 + len(p.Model.Queue)) % len(p.Model.Queue))
	}
}
func (p *Player) Next()    { p.PlayIndex(p.Model.NextIndex(false)) }
func (p *Player) Repeat()  { p.Model.Repeat = (p.Model.Repeat + 1) % 3; p.refresh() }
func (p *Player) Shuffle() { p.Model.Shuffle = !p.Model.Shuffle; p.refresh() }
func (p *Player) Playlist() {
	p.Model.Playlist = !p.Model.Playlist
	if p.Skin.ID == "series-9" && !p.fullscreen {
		w, h := p.Window.Size()
		if p.Model.Playlist {
			w += 250
			p.Window.SetMinSize(596, 344)
		} else {
			w -= 250
			p.Window.SetMinSize(346, 344)
		}
		p.Window.SetSize(w, h)
	}
	p.body.queue.SetVisible(p.Model.Playlist && !p.fullscreen)
	p.refresh()
	p.Window.RequestLayout()
	p.Window.RequestFocus(p.body.video)
}
func (p *Player) Fullscreen() {
	on := !p.fullscreen
	if !p.Window.SetFullscreen(on) {
		p.fail(fmt.Errorf("fullscreen is unavailable on this window backend"))
		return
	}
	p.fullscreen = on
	p.refresh()
	p.Window.RequestLayout()
	p.Window.RequestFocus(p.body.video)
}
func (p *Player) Remove(index int) {
	m := p.Model
	if index < 0 || index >= len(m.Queue) {
		return
	}
	playing := index == m.Index && !m.Stopped
	if index == m.Index {
		p.Stop()
	}
	m.Queue = append(m.Queue[:index], m.Queue[index+1:]...)
	if index < m.Index {
		m.Index--
	}
	if m.Index >= len(m.Queue) {
		m.Index = len(m.Queue) - 1
	}
	p.body.queue.Selected = min(index, len(m.Queue)-1)
	if playing && len(m.Queue) > 0 {
		p.PlayIndex(max(0, m.Index))
	}
	p.refresh()
}
func (p *Player) ClearQueue() {
	p.Stop()
	p.Model.Queue = nil
	p.Model.Index = -1
	p.body.queue.Selected = -1
	p.refresh()
}
func (p *Player) LoadPaths(paths []string, appendToQueue bool) error {
	videos, err := Expand(p.ctx, paths)
	if err != nil {
		return err
	}
	p.commit(videos, appendToQueue)
	return nil
}
func (p *Player) commit(videos []Video, appendToQueue bool) {
	start := !appendToQueue || len(p.Model.Queue) == 0
	if appendToQueue {
		p.Model.Queue = append(p.Model.Queue, videos...)
	} else {
		p.Model.Queue = videos
	}
	p.Model.Error = ""
	if start {
		p.PlayIndex(0)
	}
	p.refresh()
}
func (p *Player) queuePaths(paths []string, appendToQueue bool) {
	select {
	case p.files <- fileRequest{append([]string(nil), paths...), appendToQueue}:
	default:
		p.fail(fmt.Errorf("too many pending folder requests"))
	}
}
func (p *Player) fileWorker() {
	defer p.workers.Done()
	for {
		select {
		case <-p.ctx.Done():
			return
		case r := <-p.files:
			videos, err := Expand(p.ctx, r.paths)
			select {
			case p.fileResults <- fileResult{videos, r.append, err}:
			case <-p.ctx.Done():
				return
			}
		}
	}
}
