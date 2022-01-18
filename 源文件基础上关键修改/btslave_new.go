package main

import (
	"encoding/json"
	"fmt"
	"github.com/jackpal/Taipei-Torrent/torrent"
	"golang.org/x/net/proxy"
	"io"
	"io/ioutil"
	"log"
	"math"
	"net"
	"os"
	"protocol"
	"encoding/base64"
	"crypto/md5"
	"encoding/hex"
	"gopkg.in/yaml.v2"
)

// 1. 开启对话
// 2. 接收master命令下载种子文件
// 3. 接收master命令开始bt下载
// 4. 接收master命令终止bt下载

//发送任务状态
func handleConnection_SendStatus(conn net.Conn, mission string, status string) {

	sendstatus := "{\"Mission\":\"" + mission + "\", \"Status\":\"" + status + "\"}"
	Log(sendstatus)
	conn.Write(protocol.Enpack([]byte(sendstatus)))

	Log("Status sent.")
	//defer conn.Close()

}

func handleConnection_getMission(conn net.Conn) {

	// 缓冲区，存储被截断的数据
	tmpBuffer := make([]byte, 0)

	//接收解包
	readerChannel := make(chan []byte, 16)
	go reader(readerChannel, conn)

	buffer := make([]byte, 1024)
	for {
		n, err := conn.Read(buffer)
		if err != nil {
			if err == io.EOF {
				Log("Client disconnected.")
			} else {
				Log(conn.RemoteAddr().String(), " connection error: ", err)
			}
			return
		}

		tmpBuffer = protocol.Depack(append(tmpBuffer, buffer[:n]...), readerChannel)
	}

}

func Log(v ...interface{}) {
	log.Println(v...)
}

//读取channel中的消息并作出相应的操作和回应
func reader(readerChannel chan []byte, conn net.Conn) {
	quit := make(chan bool)
	for {
		select {
		case data := <-readerChannel:
			Log("received: ", string(data))
			var dat map[string]interface{}
			if err := json.Unmarshal(data, &dat); err == nil {

				// 建立对话，获取任务
				if dat["Mission"].(string) == "TriggerConversation" {
					Log("Conversation established.")
					mission := "Requiremission"
					status := "NULL"
					handleConnection_SendStatus(conn, mission, status)
				}

				// 获取任务种子文件
				if dat["Mission"].(string) == "DownloadTorrent" {
					Log("Received Mission: DownloadTorrent")
					mission := "DownloadTorrent"
					//接收种子，写入文件，校验md5
					torrentFilename := dat["TorrentFile"].(string)
					decodeBytes, err := base64.StdEncoding.DecodeString(dat["TorrentContent"].(string))
					if err != nil {
						log.Fatalln(err)
					}
					torrentMD5 := dat["TorrentMD5"].(string)
					torrentFile, err := os.OpenFile(torrentFilename, os.O_RDWR|os.O_CREATE, 0)
					if err != nil {
						log.Fatalln("Failed to open file!", err)
					}
					defer torrentFile.Close()
					torrentFile.Write(decodeBytes)
					torrentFilemd5, _ := GetFileMd5(torrentFilename)
					if torrentMD5 == torrentFilemd5 {
						status := "OK"
						Log("Torrent downloaded.")
						handleConnection_SendStatus(conn, mission, status)
					}
				}

				// 获取开启BT任务命令
				if dat["Mission"].(string) == "StartBT" {
					go func() {
						for {
							select {
							case <- quit:
								return
							default:
								Start_BT(dat["TorrentFile"].(string))
							}
						}
					}()
					Log("BT started.")
					mission := "StartBT"
					status := "OK"
					handleConnection_SendStatus(conn, mission, status)
				}

				// 获取停止BT任务命令
				if dat["Mission"].(string) == "StopBT" {
					quit <- true
					Log("BT stopped.")
					mission := "StopBT"
					status := "OK"
					handleConnection_SendStatus(conn, mission, status)
					os.Exit(0)
				}
			} else {
				Log(err, "Json parse failed!")
			}
		}
	}
}

//文件md5值计算
func GetFileMd5(filename string) (string, error) {
	file, err := os.Open(filename)
	if err != nil {
		log.Fatal("os Open error")
		return "", err
	}
	md5 := md5.New()
	_, err = io.Copy(md5, file)
	if err != nil {
		log.Fatal("io copy error")
		return "", err
	}
	md5Str := hex.EncodeToString(md5.Sum(nil))
	return md5Str, nil
}

//bt客户端
var (
	cpuprofile    = "" //If not empty, collects CPU profile samples and writes the profile to the given file before the program exits
	memprofile    = "" //If not empty, writes memory heap allocations to the given file before the program exits
	createTorrent = "" //If not empty, creates a torrent file from the given root. Writes to stdout
	createTracker = "" //Creates a tracker serving the given torrent file on the given address. Example --createTracker=:8080 to serve on port 8080.

	port                = 7779        //Port to listen on. 0 means pick random port. Note that 6881 is blacklisted by some trackers.
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

//bt启动函数
func Start_BT(torrentFile string) {
	torrentFiles = []string{torrentFile}

	torrentFlags, err := parseTorrentFlags()
	if err != nil {
		log.Fatal("Could not parse flags:", err)
	}

	Log("Starting.")

	err = torrent.RunTorrents(torrentFlags, torrentFiles)
	if err != nil {
		log.Fatal("Could not run torrents", err)
	}

}

//yaml文件内容影射的结构体，注意结构体成员要大写开头
type Config struct {
	MasterAddr string `yaml:"MasterAddr"`
}

func ReadYaml(configPath string) (config Config, err error) {
	data, err := ioutil.ReadFile(configPath)
	if err != nil {
		log.Fatal("Read file error:", err)
		return
	}
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		log.Fatal("Unmarshal error:", err)
		return
	}
	return
}


//main函数
func main() {
	configPath := "./btslaveconfig.yaml"
	config, err := ReadYaml(configPath)
	if err != nil {
		log.Fatal("Read yaml error:", err)
		return
	}
	master := config.MasterAddr
	tcpAddr, err := net.ResolveTCPAddr("tcp4", master)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Fatal error: %s", err.Error())
		os.Exit(1)
	}

	conn, err := net.DialTCP("tcp", nil, tcpAddr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Fatal error: %s", err.Error())
		os.Exit(1)
	}

	Log("connect to server success")

	handleConnection_getMission(conn)
}
