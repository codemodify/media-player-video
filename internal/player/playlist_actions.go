package player

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/codemodify/uitoolkit/widget"
	"github.com/codemodify/uitoolkit/widgets"
)

func (p *Player) sortQueue() {
	m := p.Model
	order := make([]int, len(m.Queue))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(i, j int) bool {
		return strings.ToLower(m.Queue[order[i]].Title()) < strings.ToLower(m.Queue[order[j]].Title())
	})
	queue := make([]Video, len(order))
	selected := p.body.queue.Selected
	oldCurrent := m.Index
	for i, old := range order {
		queue[i] = m.Queue[old]
		if old == oldCurrent {
			m.Index = i
		}
		if old == selected {
			p.body.queue.Selected = i
		}
	}
	m.Queue = queue
	p.refresh()
}

func (p *Player) moveQueue(delta int) {
	m := p.Model
	from := p.body.queue.Selected
	to := from + delta
	if from < 0 || to < 0 || to >= len(m.Queue) {
		return
	}
	m.Queue[from], m.Queue[to] = m.Queue[to], m.Queue[from]
	if m.Index == from {
		m.Index = to
	} else if m.Index == to {
		m.Index = from
	}
	p.body.queue.Selected = to
	p.refresh()
}

func (p *Player) savePlaylist(from widget.Component) {
	widgets.ShowFileDialog(from, widgets.FileDialogOptions{Title: "Save playlist", Mode: widgets.FileSave, Name: "playlist.m3u8", Filter: "*.m3u8 *.m3u", Native: true, OnPick: func(path string) {
		var contents strings.Builder
		contents.WriteString("#EXTM3U\n")
		for _, v := range p.Model.Queue {
			if strings.ContainsAny(v.Path, "\r\n") {
				p.fail(fmt.Errorf("a playlist path contains a newline"))
				return
			}
			contents.WriteString(v.Path)
			contents.WriteByte('\n')
		}
		if err := os.WriteFile(path, []byte(contents.String()), 0644); err != nil {
			p.fail(err)
		}
	}})
}

func (p *Player) loadPlaylist(from widget.Component) {
	widgets.ShowFileDialog(from, widgets.FileDialogOptions{Title: "Load playlist", Mode: widgets.FileOpen, Filter: "*.m3u8 *.m3u", Native: true, OnPick: func(path string) {
		paths, err := readPlaylist(path)
		if err != nil {
			p.fail(err)
			return
		}
		p.queuePaths(paths, false)
	}})
}

func readPlaylist(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var paths []string
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(strings.TrimPrefix(scanner.Text(), "\ufeff"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.Contains(line, "://") {
			return nil, fmt.Errorf("this player accepts local playlist files: %s", line)
		}
		if !filepath.IsAbs(line) {
			line = filepath.Join(filepath.Dir(path), line)
		}
		paths = append(paths, line)
		if len(paths) > 100000 {
			return nil, fmt.Errorf("playlist is too large")
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("playlist has no local videos")
	}
	return paths, nil
}
