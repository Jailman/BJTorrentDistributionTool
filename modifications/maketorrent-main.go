package main

import (
	"github.com/jackpal/Taipei-Torrent/torrent"
	"log"
	"os"
)

var (
	createTorrent = os.Args[1] 		 //If not empty, creates a torrent file from the given root. Writes to stdout
	createTracker = os.Args[2]       //Creates a tracker serving the given torrent file on the given address. Example --createTracker=:8080 to serve on port 8080.
)

func main() {

	logFile, err := os.OpenFile(os.Args[3], os.O_RDWR|os.O_CREATE, 0)
	if err != nil {
		log.Fatalln("Failed to open file!", err)
	}
	defer logFile.Close()
	terr := torrent.WriteMetaInfoBytes(createTorrent, createTracker, logFile)
	if terr != nil {
		log.Fatal("Could not create torrent file:", terr)
	}

}

// help:
// arg1 filename
// arg2 tracker
// arg3 torrentfile
// maketorrent.exe file tracker_addr:port torrentfile