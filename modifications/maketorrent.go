package maketorrent

import (
	"github.com/Jailman/BJTorrentDistributionTool/torrent"
	"log"
	"os"
)

var (
	createTorrent = "sbt_client.with-apt-boost" //If not empty, creates a torrent file from the given root. Writes to stdout
	createTracker = "192.168.64.131:6969"       //Creates a tracker serving the given torrent file on the given address. Example --createTracker=:8080 to serve on port 8080.
)

func main() {

	logFile, err := os.OpenFile("test.torrent", os.O_RDWR|os.O_CREATE, 0)
	if err != nil {
		log.Fatalln("Failed to open file!", err)
	}
	defer logFile.Close()
	terr := torrent.WriteMetaInfoBytes(createTorrent, createTracker, logFile)
	if terr != nil {
		log.Fatal("Could not create torrent file:", terr)
	}

}
