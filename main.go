package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/codemodify/media-player-video/internal/appicon"
	"github.com/codemodify/media-player-video/internal/player"
	"github.com/codemodify/media-player-video/internal/skins"
	"github.com/codemodify/uitoolkit"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}
func run() error {
	log.SetFlags(0)
	log.SetPrefix("media-player-video: ")
	skin := flag.String("skin", "", "appearance: "+skins.Choices()+" (last used, or zoom-player)")
	scale := flag.Float64("scale", 0, "display scale (0: desktop or UITK_SCALE)")
	shot := flag.String("shot", "", "write a deterministic headless screenshot and exit")
	at := flag.Duration("at", 97*time.Second, "position in screenshot mode")
	library := flag.String("mpv-library", "", "full path to libmpv shared library (auto-detected by default)")
	noSession := flag.Bool("no-session", false, "do not read or write a saved session")
	flag.Parse()
	if *scale < 0 || *scale > 4 {
		return fmt.Errorf("scale must be between 0 and 4")
	}
	headless := *shot != ""
	session := player.SessionPath()
	if headless || *noSession {
		session = ""
	}
	icons, err := appicon.Images()
	if err != nil {
		return err
	}
	a := uitoolkit.New(uitoolkit.Options{Headless: headless, Scale: float32(*scale)})
	a.SetIcon(icons...)
	p, err := player.New(a, player.Options{Headless: headless, Skin: *skin, SessionPath: session, Library: *library})
	if err != nil {
		return err
	}
	defer p.Close()
	if headless {
		p.Pose(at.Seconds())
		a.PumpOnce()
		if err = os.MkdirAll(*shot, 0o755); err != nil {
			return err
		}
		path := filepath.Join(*shot, p.Skin.ID+"-main.png")
		if err = p.WriteScreenshots(*shot); err != nil {
			return err
		}
		fmt.Println("wrote", path)
		return nil
	}
	if len(flag.Args()) > 0 {
		if err = p.LoadPaths(flag.Args(), false); err != nil {
			return err
		}
	}
	stop := make(chan os.Signal, 1)
	done := make(chan struct{})
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(stop)
	defer close(done)
	go func() {
		select {
		case <-stop:
			a.Quit()
		case <-done:
		}
	}()
	return a.Run()
}
