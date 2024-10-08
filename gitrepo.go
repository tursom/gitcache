package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/robfig/cron"
)

type LocalMirrorsInfo struct {
	Count    int64  `json:"count"`
	Progress string `json:"progress"`
	Size     int64  `json:"size"`
	Nodes    string `json:"nodes"`
}

type RemoteRepsInfo struct {
	Stargazers_count int64  `json:"stargazers_count"`
	Language         string `json:"language"`
	Description      string `json:"description"`
	Updated_at       string `json:"updated_at"`
}

var _PATH_DEPTH = 2
var _REPO_COUNT int64 = 0
var _REPO_ALL_COUNT int64 = 0
var _SYNC_PROGRESS = 0

func fetchMirrorFromRemoteUnshallow(repository string) {
	_SYNC_PROGRESS = _SYNC_PROGRESS + 1
	//avoid devide by zero
	if _REPO_COUNT > 0 {
		log.Printf("git remote update: %v of %v , %.2f%%\n", _SYNC_PROGRESS, _REPO_COUNT, float64(_SYNC_PROGRESS)/float64(_REPO_COUNT)*100.00)
	}
	remote := "https:/" + strings.Replace(repository, g_Basedir, "", -1)
	//avoid public repository change to private,git remote update will be hung
	if global_ssh == "0" {
		if !httpHead(remote) {
			log.Printf("git remote update: %s %s\n", remote, "remote not exists")
			return
		}
	}
	local := repository
	//remove expire repo  only make a log , do real remove at next commit!!!!!
	if CacheIsExpire(remote) {
		log.Printf("!!!!!!!!remove expire repo: %s\n", remote)
		if strings.Contains(local, g_Basedir) && !(local == g_Basedir) {
			os.RemoveAll(local)
			RemoveCacheFromDb(remote)
		}
		return
	}
	log.Printf("git remote update: %s begin\n", local)
	err := fetchMirrorFromRemote(remote, local, "update")
	if err == "" {
		err = "ok"
	}
	log.Printf("git remote update: %s %s\n", local, err)
}

func fetchMirrorFromRemoteUnshallowA(repository string) {
	remote := "https:/" + strings.Replace(repository, g_Basedir, "", -1)
	//avoid public repository change to private,git remote update will be hung
	if global_ssh == "0" {
		if !httpHead(remote) {
			log.Printf("git remote update: %s %s\n", remote, "remote not exists")
			return
		}
	}
	local := repository
	log.Printf("git remote update: %s begin\n", local)
	err := fetchMirrorFromRemote(remote, local, "update")
	if err == "" {
		err = "ok"
	}
	log.Printf("git remote update: %s %s\n", local, err)
}

func countCacheRepository(repository string) {
	_REPO_COUNT++
}

func walkDir(dirpath string, depth int, f func(string)) {
	if depth > _PATH_DEPTH {
		return
	}
	files, err := ioutil.ReadDir(dirpath)
	if err != nil {
		return
	}
	for _, file := range files {
		if file.IsDir() {
			walkDir(dirpath+"/"+file.Name(), depth+1, f)
			headExist, _ := PathExists(dirpath + "/" + file.Name() + "/HEAD")
			if headExist && (!strings.HasSuffix(file.Name(), "logs")) {
				f(dirpath + "/" + file.Name())
			}
			continue
		}
	}
}

func SyncLocalMirrorFromRemote() {
	defer func() {
		if err := recover(); err != nil {
			log.Printf("process recover: %s\n", err)
		}
	}()
	log.Println("sync local mirror from remote begin")
	_SYNC_PROGRESS = 0
	walkDir(g_Basedir, 0, fetchMirrorFromRemoteUnshallow)
	log.Println("sync local mirror from remote end")
}

func GetOutboundIP() net.IP {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()
	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP
}

func countAllCacheRepository() {
	//delay 30 second,get all repo count from db
	//if not use db,then use local cache count
	time.Sleep(time.Duration(30) * time.Second)
	var ct = CacheCount()
	if ct == 0 {
		ct = _REPO_COUNT
	}
	_REPO_ALL_COUNT = ct
	log.Printf("sync all cache repository : %v\n", _REPO_ALL_COUNT)
}

func SyncCountCacheRepository() {
	defer func() {
		if err := recover(); err != nil {
			log.Printf("process recover: %s\n", err)
		}
	}()
	_REPO_COUNT = 0
	walkDir(g_Basedir, 0, countCacheRepository)
	log.Printf("sync local cache repository : %v\n", _REPO_COUNT)
	if _REPO_COUNT > 0 {
		log.Printf("git remote sync: %v of %v , %.2f%%\n", _SYNC_PROGRESS, _REPO_COUNT, float64(_SYNC_PROGRESS)/float64(_REPO_COUNT)*100.00)
	}
	//delay 30 second
	go countAllCacheRepository()
}

func httpGet(url string) string {
	resp, err := http.Get(url)
	if err != nil {
		log.Println(err)
		return ""
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	return string(body)
}

func httpHead(url string) bool {
	resp, err := http.Head(url)
	if err != nil {
		log.Println(err)
		return false
	}
	if resp.StatusCode == 200 {
		return true
	} else {
		return false
	}
}

func GetLocalMirrorsInfo() string {
	if _REPO_COUNT == 0 {
		walkDir(g_Basedir, 0, countCacheRepository)
	}
	info := LocalMirrorsInfo{}
	info.Count = _REPO_COUNT
	var ip net.IP = GetOutboundIP()
	var sip = "node:0"
	str := strings.Split(ip.String(), ".")
	if len(str) == 4 {
		sip = "node" + str[3]
	}
	info.Nodes = sip
	info.Progress = ""
	if _REPO_ALL_COUNT > 0 {
		info.Size = _REPO_ALL_COUNT
	} else {
		info.Size = _REPO_COUNT
	}
	data, _ := json.Marshal(info)
	return string(data)
}

func httpPost(url string, contentType string, body string) string {
	resp, err := http.Post(url, contentType, strings.NewReader(body))
	if err != nil {
		return err.Error()
	}
	defer resp.Body.Close()
	rbody, err1 := ioutil.ReadAll(resp.Body)
	if err1 != nil {
		return err1.Error()
	}
	return string(rbody)
}

func BroadCastGitCloneCommandToChain(repository string) {
	log.Println("broadcast git clone command to chain : " + repository)
	var msgtx MsgTx
	msgtx.PrivateKey = "f45b1d6e433195a0e70a09ffaf59d4c71bc9c49f84cfe63fd455b3c34a8fcd2d246ea5c7d47cf6027e4ec99b27dade8e23bb811a07b90228c3f27f744c4d1322"
	msgtx.PublicKey = "246EA5C7D47CF6027E4EC99B27DADE8E23BB811A07B90228C3F27F744C4D1322"
	msgtx.Msg = "git clone " + repository
	go BroadCastMsg(msgtx)
}

func SaveRepsInfoToDb(repository string) {
	path := strings.Replace("https:/"+strings.Replace(repository, g_Basedir, "", -1), ".git", "", -1)
	name := strings.Replace(filepath.Base(repository), ".git", "", -1)
	utime := GetFileModTime(repository)
	SaveRepsInfo(name, path, utime)
}

func DirSize(path string) (int64, error) {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			size += info.Size()
		}
		return err
	})
	return size, err
}

func SaveRepsDetailToDb(repository string) {
	path := strings.Replace("https:/"+strings.Replace(repository, g_Basedir, "", -1), ".git", "", -1)
	url := strings.Replace(path, "https://github.com", "http://plus.gitclone.com:5001/gitcache/star", -1)
	size, _ := DirSize(repository)
	//url := strings.Replace(path, "https://github.com", "http://127.0.0.1:5001/gitcache/star", -1)
	remoteDetail := httpGet(url)
	if len(remoteDetail) > 0 {
		var remoteRepsInfo RemoteRepsInfo
		json.Unmarshal([]byte(remoteDetail), &remoteRepsInfo)
		updated_at, _ := time.Parse("2006-01-02T15:04:05Z", remoteRepsInfo.Updated_at)
		UpdateReposDetail(path, remoteRepsInfo.Stargazers_count, remoteRepsInfo.Language, remoteRepsInfo.Description, updated_at, size)
	}
	log.Println("sync repo star info : " + url)
}

func SyncLocalMirrorInfoToDB() {
	defer func() {
		if err := recover(); err != nil {
			log.Printf("process recover: %s\n", err)
		}
	}()
	log.Println("sync local mirror to db begin")
	walkDir(g_Basedir, 0, SaveRepsInfoToDb)
	log.Println("sync local mirror to db end")
}

func SyncRepoDetailToDB() {
	defer func() {
		if err := recover(); err != nil {
			log.Printf("process recover: %s\n", err)
		}
	}()
	log.Println("sync repo star info begin")
	walkDir(g_Basedir, 0, SaveRepsDetailToDb)
	log.Println("sync repo star info end")
}

func GetFileModTime(path string) time.Time {
	hh, _ := time.ParseDuration("1h")
	f, err := os.Open(path)
	if err != nil {
		log.Println("open file error")
		return time.Now().Add(8 * hh)
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		log.Println("stat fileinfo error")
		return time.Now().Add(8 * hh)
	}
	return fi.ModTime().Add(8 * hh)
}

func Cron() {
	c := cron.New()
	var ip net.IP = GetOutboundIP()
	//get last part from ip
	var str = "54"
	sstr := strings.Split(ip.String(), ".")
	if len(sstr) == 4 {
		str = sstr[3]
	}
	//sync local mirror from github.com every day
	var crontime = "0 0 4 * * *"
	if str == "54" {
		crontime = "0 0 22 * * *"
	} else if str == "55" {
		crontime = "0 0 1 * * *"
	} else if str == "56" {
		crontime = "0 0 4 * * *"
	} else if str == "57" {
		crontime = "0 0 7 * * *"
	} else if str == "58" {
		crontime = "0 0 10 * * *"
	} else if str == "42" {
		crontime = "0 0 13 * * *"
	} else {
		crontime = "0 0 16 * * *"
	}
	log.Println("node" + str + " sync from remote cron at :" + crontime)
	//sync mirror info from github.com api every week
	var startime = ""
	if str == "54" {
		startime = "0 0 18 * * 1"
	} else if str == "55" {
		startime = "0 0 18 * * 2"
	} else if str == "56" {
		startime = "0 0 18 * * 3"
	} else if str == "57" {
		startime = "0 0 18 * * 4"
	} else if str == "58" {
		startime = "0 0 18 * * 5"
	} else if str == "19" {
		startime = "0 0 18 * * 6"
	} else {
		startime = "0 0 18 * * 7"
	}
	log.Println("node" + str + " sync from api cron at :" + startime)
	c.AddFunc(crontime, func() {
		//c.AddFunc("0 */1 * * * *", func() { //test
		go SyncLocalMirrorFromRemote()
	})
	//calc local mirror count every 10 min
	c.AddFunc("0 */30 * * * *", func() {
		go SyncCountCacheRepository()
	})
	//sync local mirror info to db every day
	c.AddFunc("0 0 6 * * *", func() {
		//c.AddFunc("0 */1 * * * *", func() {
		go SyncLocalMirrorInfoToDB()
	})
	//sync repo star info to db every week
	c.AddFunc(startime, func() {
		//c.AddFunc("0 */1 * * * *", func() {
		go SyncRepoDetailToDB()
	})
	c.Start()
	log.Println("node" + str + " cron start")
	return
}
