package playregister

import (
	"encoding/json"
	"fmt"
	"github.com/leochen2038/play"
	"github.com/leochen2038/play/config"
	"github.com/leochen2038/play/middleware/etcd"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
)

var (
	buildId       = ""
	intranetIp    = ""
	exePath       = ""
	lastConfigVer = ""
	socketListen  = ""
	httpListen    = ""
)

func SetBuildId(id string) {
	buildId = id
}

func EtcdWithUrl(configUrl string) (err error) {
	var configKey string
	var runningKey string
	var crontabKey string
	var endpoints []string

	if configKey, runningKey, crontabKey, endpoints, err = getEtcdKeyAndEndpoints(configUrl); err == nil {
		return EtcdWithArgs(configKey, runningKey, crontabKey, endpoints)
	}

	return
}

func EtcdWithArgs(configKey, runningKey, crontabKey string, endpoints []string) (err error) {
	var etcdAgent *etcd.EtcdAgent
	if etcdAgent, err = etcd.NewEtcdAgent(endpoints); err != nil {
		return
	}

	// step 1. 获取配置信息
	var configParser play.Parser
	if configParser, err = config.NewEtcdParser(etcdAgent, configKey); err != nil {
		return
	}
	config.InitConfig(configParser)

	// step 3. 注册运行时状态
	intranetIp = play.GetIntranetIp()
	exePath, _ = os.Executable()
	socketListen, _ = config.String("listen.socket")
	httpListen, _ = config.String("listen.http")

	// step 2. 开始定时任务
	play.CronStartWithEtcd(etcdAgent, crontabKey, exePath+".cron")

	etcdAgent.StartKeepAlive(runningKey, 3, func() (newVal string, isChange bool, err error) {
		var version string
		if version, err = config.String("version"); err != nil {
			return
		} else if version != lastConfigVer {
			isChange = true
			lastConfigVer = version
			newVal = etcdRunningStatus(lastConfigVer, buildId, intranetIp, exePath, socketListen, httpListen, os.Getpid())
			return
		}
		return
	})

	return
}

func etcdRunningStatus(configver, buildId, intranetIp, exePath, socketListen, httpListen string, pid int) string {
	return fmt.Sprintf(`{"configVer":"%s", "buildId":"%s", "ip":"%s", "pid":%d, "path":"%s", "socketListen":"%s", "httpListen":"%s"}`,
		configver, buildId, intranetIp, os.Getpid(), exePath, socketListen, httpListen)
}

func getEtcdKeyAndEndpoints(configUrl string) (configKey, runningKey, crontabKey string, endpoints []string, err error) {
	var ip string
	var path string
	var resp *http.Response
	var responseByte []byte
	var responseMap map[string]interface{}

	ip = play.GetIntranetIp()
	if path, err = os.Executable(); err != nil {
		return
	}

	if resp, err = http.PostForm(configUrl, url.Values{"ip": []string{ip}, "path": []string{path}}); err != nil {
		return
	}

	if responseByte, err = ioutil.ReadAll(resp.Body); err != nil {
		return
	}

	if err = json.Unmarshal(responseByte, &responseMap); err != nil {
		return
	}

	if key, ok := responseMap["configKey"]; ok {
		configKey = key.(string)
	}
	if key, ok := responseMap["serviceKey"]; ok {
		runningKey = key.(string)
	}
	if key, ok := responseMap["crontabKey"]; ok {
		crontabKey = key.(string)
	}

	for _, v := range responseMap["endpoints"].([]interface{}) {
		endpoints = append(endpoints, v.(string))
	}
	return
}
