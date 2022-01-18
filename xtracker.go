package xtracker

import (
	"log"
	"os"
	"os/signal"
	"path"

	"github.com/jackpal/Taipei-Torrent/torrent"
	"github.com/jackpal/Taipei-Torrent/tracker"
	"golang.org/x/net/proxy"
)

func main() {

	err := startTracker("0.0.0.0:6969", []string{"test.torrent"})
	if err != nil {
		log.Fatal("Tracker returned error:", err)
	}
	return

}

func startTracker(addr string, torrentFiles []string) (err error) {
	t := tracker.NewTracker()
	// TODO(jackpal) Allow caller to choose port number
	t.Addr = addr
	dial := proxy.FromEnvironment()
	for _, torrentFile := range torrentFiles {
		var metaInfo *torrent.MetaInfo
		metaInfo, err = torrent.GetMetaInfo(dial, torrentFile)
		if err != nil {
			return
		}
		name := metaInfo.Info.Name
		if name == "" {
			name = path.Base(torrentFile)
		}
		err = t.Register(metaInfo.InfoHash, name)
		if err != nil {
			return
		}
	}
	go func() {
		quitChan := listenSigInt()
		select {
		case <-quitChan:
			log.Printf("got control-C")
			t.Quit()
		}
	}()

	err = t.ListenAndServe()
	if err != nil {
		return
	}
	return
}

func listenSigInt() chan os.Signal {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, os.Kill)
	return c
}
