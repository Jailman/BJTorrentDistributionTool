package xbt

import (
	"log"
	"math"

	"github.com/jackpal/Taipei-Torrent/torrent"
	"golang.org/x/net/proxy"
)

var (
	cpuprofile    = "" //If not empty, collects CPU profile samples and writes the profile to the given file before the program exits
	memprofile    = "" //If not empty, writes memory heap allocations to the given file before the program exits
	createTorrent = "" //If not empty, creates a torrent file from the given root. Writes to stdout
	createTracker = "" //Creates a tracker serving the given torrent file on the given address. Example --createTracker=:8080 to serve on port 8080.

	port                = 7777        //Port to listen on. 0 means pick random port. Note that 6881 is blacklisted by some trackers.
	fileDir             = "."         //path to directory where files are stored
	seedRatio           = math.Inf(0) //Seed until ratio >= this value before quitting.
	useDeadlockDetector = false       //Panic and print stack dumps when the program is stuck.
	useLPD              = false       //Use Local Peer Discovery
	useUPnP             = false       //Use UPnP to open port in firewall.
	useNATPMP           = false       //Use NAT-PMP to open port in firewall.
	gateway             = ""          //IP Address of gateway.
	useDHT              = false       //Use DHT to get peers.
	trackerlessMode     = false       //Do not get peers from the tracker. Good for testing DHT mode.
	proxyAddress        = ""          //Address of a SOCKS5 proxy to use.
	initialCheck        = true        //Do an initial hash check on files when adding torrents.
	useSFTP             = ""          //SFTP connection string, to store torrents over SFTP. e.g. 'username:password@192.168.1.25:22/path/'
	useRamCache         = 0           //Size in MiB of cache in ram, to reduce traffic on torrent storage.
	useHdCache          = 0           //Size in MiB of cache in OS temp directory, to reduce traffic on torrent storage.
	execOnSeeding       = ""          //Command to execute when torrent has fully downloaded and has begun seeding.
	quickResume         = false       //Save torrenting data to resume faster. '-initialCheck' should be set to false, to prevent hash check on resume.
	maxActive           = 16          //How many torrents should be active at a time. Torrents added beyond this value are queued.
	memoryPerTorrent    = -1          //Maximum memory (in MiB) per torrent used for Active Pieces. 0 means minimum. -1 (default) means unlimited.
	torrentFiles        []string
)

func parseTorrentFlags() (flags *torrent.TorrentFlags, err error) {
	dialer := proxy.FromEnvironment()

	flags = &torrent.TorrentFlags{
		Dial:                dialer,
		Port:                port,
		FileDir:             fileDir,
		SeedRatio:           seedRatio,
		UseDeadlockDetector: useDeadlockDetector,
		UseLPD:              useLPD,
		UseDHT:              useDHT,
		UseUPnP:             useUPnP,
		UseNATPMP:           useNATPMP,
		TrackerlessMode:     trackerlessMode,
		// IP address of gateway
		Gateway:            gateway,
		InitialCheck:       initialCheck,
		FileSystemProvider: torrent.OsFsProvider{},
		Cacher:             nil,
		ExecOnSeeding:      execOnSeeding,
		QuickResume:        quickResume,
		MaxActive:          maxActive,
		MemoryPerTorrent:   memoryPerTorrent,
	}
	return
}

func main() {

	torrentFiles = []string{"test.torrent"}

	torrentFlags, err := parseTorrentFlags()
	if err != nil {
		log.Fatal("Could not parse flags:", err)
	}

	log.Println("Starting.")

	err = torrent.RunTorrents(torrentFlags, torrentFiles)
	if err != nil {
		log.Fatal("Could not run torrents", err)
	}
}
