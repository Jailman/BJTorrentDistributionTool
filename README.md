# BJTorrentDistributionTool
BJTorrentDistributionTool is based on TaipeiTorrent and it is a torrent tool set for distributing app package among game servers.  
btmaster listens on 61111 which can be configured, when started with "./btmaster filename", it will start a tracker and an original upload bt client.  
When started, btslave will try to reach btmaster 61111 to start a conversation which will get a mission including the torrent file, then it starts a bt client to download the file.  
When the configured slaves are done, tracker will report to btmaster, btmaster will stop itself along with original bt client and tracker, thus the btslave will also stop itself and the download bt client.  
Many thanks to Taipei-Torrent author @jackpal.