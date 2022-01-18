package main

import (
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/jackpal/Taipei-Torrent/torrent"
	"github.com/jackpal/Taipei-Torrent/tracker"
	"golang.org/x/net/proxy"
	"io"
	"io/ioutil"
	"log"
	"math"
	"net"
	"os"
	"os/signal"
	"path"
	"protocol"
	"gopkg.in/yaml.v2"
)

// 包括tracker，bt和种子及命令下发
// 1. 制作文件种子
// 2. 启动tracker
// 3. 启动上传bt
// 4. 开启会话
// 5. 向slave下发种子
// 6. 向slave下发开始下载命令
// 7. 根据tracker报告统计slave下载完成情况
// 8. 完成则下发命令终止bt下载任务

// 制作文件种子
func Make_Torrent(Filename string, Tracker string) (string, string, string) {

	torrentFileN := Filename + ".torrent"
	createTorrent := Filename
	createTracker := Tracker
	torrentFile, err := os.OpenFile(torrentFileN, os.O_RDWR|os.O_CREATE, 0)
	if err != nil {
		log.Fatalln("Failed to open file!", err)
	}
	defer torrentFile.Close()
	wterr := torrent.WriteMetaInfoBytes(createTorrent, createTracker, torrentFile)
	if wterr != nil {
		log.Fatal("Could not create torrent file:", wterr)
	}
	// 计算torrent文件base64编码
	torrentbytes, _ := ioutil.ReadFile(torrentFileN)
	torrentbase64 := base64.StdEncoding.EncodeToString(torrentbytes)
	// 计算torrent文件md5
	filemd5, _ := GetFileMd5(torrentFileN)
	return torrentFileN, torrentbase64, filemd5

}

// 文件md5值计算
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

// 启动tracker
func Start_Tracker(addr string, torrentFiles []string) (err error) {
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

// 启动上传bt
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

// BT上传启动函数
func Start_BT(torrentFile string) {
	torrentFiles = []string{torrentFile}

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

// 功能函数
// 判断文件是否存在
func PathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// 手动中断
func listenSigInt() chan os.Signal {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, os.Kill)
	return c
}

// Socket通信函数
// 下发任务
func handleConnection_SendMission(conn net.Conn, mission string) {
	conn.Write(protocol.Enpack([]byte(mission)))
	Log(mission)
	Log("Mission sent.")
	//defer conn.Close()
}

func handleConnection_getStatus(conn net.Conn, totalslave string, torrentfile string, torrentcontent string, torrentMD5 string) {

	// 缓冲区，存储被截断的数据
	tmpBuffer := make([]byte, 0)

	// 接收解包
	readerChannel := make(chan []byte, 16)
	go reader(readerChannel, conn, totalslave, torrentfile, torrentcontent, torrentMD5)

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
	//defer conn.Close()

}

// 读取channel中的消息并作出相应的操作和回应
func reader(readerChannel chan []byte, conn net.Conn, totalslave string, torrentfile string, torrentcontent string, torrentMD5 string) {
	for {
		select {
		case data := <-readerChannel:
			var dat map[string]interface{}
			if err := json.Unmarshal([]byte(string(data)), &dat); err == nil {

				if dat["Mission"].(string) == "Requiremission" {
					Log(conn.RemoteAddr().String(), "Requiremission")
					mission := "{\"Mission\":\"DownloadTorrent\", \"TorrentFile\":\"" + torrentfile + "\", \"TorrentContent\":\"" + torrentcontent + "\", \"TorrentMD5\": \"" + torrentMD5 + "\"}"
					handleConnection_SendMission(conn, mission)
				}

				// 接收消息确认BTslave是否下载种子完成，完成即通知启动BT下载任务
				if dat["Mission"].(string) == "DownloadTorrent" {
					status := dat["Status"].(string)
					Log("DownloadTorrent Mission Status: " + status)
					if status == "OK" {
						mission := "{\"Mission\":\"StartBT\", \"TorrentFile\":\"" + torrentfile + "\", \"TorrentContent\":\"\", \"TorrentMD5\": \"\"}"
						handleConnection_SendMission(conn, mission)
					} 
				}

				// 接收消息确认BTslave是否启动
				if dat["Mission"].(string) == "StartBT" {
					status := dat["Status"].(string)
					if status == "OK" {
						Log(conn.RemoteAddr().String(), "BT started.")
					}
				}

				// 接收消息确认BTslave是否关闭
				if dat["Mission"].(string) == "StopBT" {
					status := dat["Status"].(string)
					if status == "OK" {
						Log(conn.RemoteAddr().String(), "BT stopped.")
					}
				}
				// 接收tracker的消息，确认slave任务完成状态
				if dat["Mission"].(string) == "Tracker" {
					status := dat["Status"].(string)
					Log("Slave completed: ", status)
					if status == totalslave {
						Log("Slave completed.")
						handleConnection_SendMission(conn, "{\"Mission\":\"StopBT\"}")
						os.Exit(0)
					}
				}
			} else {
				log.Fatal(err)
			}
		}
	}
}

// log打印消息
func Log(v ...interface{}) {
	log.Println(v...)
}

// 错误检查
func CheckError(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "Fatal error: %s", err.Error())
		os.Exit(1)
	}
}

// yaml文件内容影射的结构体，注意结构体成员要大写开头
type Config struct {
	TotalSlaves string `yaml:"TotalSlaves"`
	MasterAddr string `yaml:"MasterAddr"`
	TrackerAddr string `yaml:"TrackerAddr"`
}

// yaml配置文件读取
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

// main函数
func main() {

	configPath := "./btmasterconfig.yaml"
	config, err := ReadYaml(configPath)
	if err != nil {
		log.Fatal("Read yaml error:", err)
		return
	}
	totalslaves := config.TotalSlaves
	masterlisten := config.MasterAddr
	trackeraddr := config.TrackerAddr

	// 建立socket，监听端口
	netListen, err := net.Listen("tcp", masterlisten)
	CheckError(err)
	defer netListen.Close()

	Log("Waiting for clients")

	tracker := trackeraddr
	dir := ""
	file := dir + os.Args[1]
	torrent := file + ".torrent"
	hastorrent, _ := PathExists(torrent)
	quit := make(chan bool)
	if !hastorrent {
		torrentfile, torrentcontent, torrentMD5 := Make_Torrent(file, tracker)
		// 启动tracker
		go func() {
			for {
				select {
				case <- quit:
					return
				default:
					Start_Tracker(tracker, []string{torrentfile})
				}
			}
		}()
		
		// 启动上传bt
		go func() {
			for {
				select {
				case <- quit:
					return
				default:
					Start_BT(torrentfile)
				}
			}
		}()
		// 向btslave发送命令和文件
		for {
			conn, err := netListen.Accept()
			if err != nil {
				continue
			}

			Log(conn.RemoteAddr().String(), " tcp connect success")
			mission := "{\"Mission\":\"TriggerConversation\"}"
			go handleConnection_SendMission(conn, mission)
			go handleConnection_getStatus(conn, totalslaves, torrentfile, torrentcontent, torrentMD5)
		}
	} else {
		os.Remove(torrent)
		torrentfile, torrentcontent, torrentMD5 := Make_Torrent(file, tracker)
		// 启动tracker
		go func() {
			for {
				select {
				case <- quit:
					return
				default:
					Start_Tracker(tracker, []string{torrentfile})
				}
			}
		}()
		
		// 启动上传bt
		go func() {
			for {
				select {
				case <- quit:
					return
				default:
					Start_BT(torrentfile)
				}
			}
		}()

		// 向btslave发送命令和文件
		for {
			conn, err := netListen.Accept()
			if err != nil {
				continue
			}

			Log(conn.RemoteAddr().String(), " tcp connect success")
			mission := "{\"Mission\":\"TriggerConversation\"}"
			go handleConnection_SendMission(conn, mission)
			go handleConnection_getStatus(conn, totalslaves, torrentfile, torrentcontent, torrentMD5)
		}
	}
}




//ToDO:
//添加使用说明
//解决文件夹传输循环写入问题（问题：文件夹总是在当前创建的同种子名字一致的文件夹下）


// 使用：
// btmaster.exe file
// 没有开启上传bt的时候，下载bt会先在本地创建文件，但是实际上并未传输完成
// 注意传输目录时目录名后一定不要加/