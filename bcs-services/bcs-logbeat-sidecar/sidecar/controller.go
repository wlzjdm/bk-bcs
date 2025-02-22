/*
 * Tencent is pleased to support the open source community by making Blueking Container Service available.
 * Copyright (C) 2019 THL A29 Limited, a Tencent company. All rights reserved.
 * Licensed under the MIT License (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the License at
 * http://opensource.org/licenses/MIT
 * Unless required by applicable law or agreed to in writing, software distributed under
 * the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
 * either express or implied. See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package sidecar

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/Tencent/bk-bcs/bcs-common/common/blog"
	bcsv1 "github.com/Tencent/bk-bcs/bcs-k8s/kubebkbcs/apis/bkbcs/v1"
	bkbcsv1 "github.com/Tencent/bk-bcs/bcs-k8s/kubebkbcs/generated/listers/bkbcs/v1"
	"github.com/Tencent/bk-bcs/bcs-services/bcs-logbeat-sidecar/config"
	"github.com/Tencent/bk-bcs/bcs-services/bcs-logbeat-sidecar/metric"
	"github.com/Tencent/bk-bcs/bcs-services/bcs-logbeat-sidecar/types"

	docker "github.com/fsouza/go-dockerclient"
	"gopkg.in/yaml.v2"
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/apimachinery/pkg/labels"
	corev1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
)

const (
	// ContainerLabelK8sContainerName container name key in containers labels
	ContainerLabelK8sContainerName = "io.kubernetes.container.name"
	// ContainerLabelK8sPodName pod name key in containers labels
	ContainerLabelK8sPodName = "io.kubernetes.pod.name"
	// ContainerLabelK8sPodNameSpace pod namespace key in containers labels
	ContainerLabelK8sPodNameSpace = "io.kubernetes.pod.namespace"
)

// SidecarController controls the behaviour of BcsLogConfig CRD
type SidecarController struct {
	sync.RWMutex

	conf   *config.Config
	client *docker.Client
	//key = containerid, value = ContainerLogConf
	logConfs map[string]*ContainerLogConf
	//key = containerid, value = *docker.Container
	containerCache      map[string]*docker.Container
	containerCacheMutex sync.RWMutex
	//log config prefix file name
	prefixFile string

	//pod Lister
	podLister corev1.PodLister
	//apiextensions clientset
	extensionClientset *apiextensionsclient.Clientset
	//BcsLogConfig Lister
	bcsLogConfigLister   bkbcsv1.BcsLogConfigLister
	bcsLogConfigInformer cache.SharedIndexInformer
}

// ContainerLogConf record the log config for container
type ContainerLogConf struct {
	confPath string
	data     []byte
	yamlData *types.Yaml
}

// LogConfParameter is no longer used
type LogConfParameter struct {
	LogFile     string
	DataID      string
	ContainerID string
	ClusterID   string
	Namespace   string
	//application or deployment't name
	ServerName string
	//application or deployment
	ServerType string
	//custom label
	CustemLabel string

	stdout         bool
	nonstandardLog string
}

// NewSidecarController returns a new bcslogconfigs controller
func NewSidecarController(conf *config.Config) (*SidecarController, error) {
	var err error
	s := &SidecarController{
		conf:           conf,
		logConfs:       make(map[string]*ContainerLogConf),
		containerCache: make(map[string]*docker.Container),
		prefixFile:     conf.PrefixFile,
	}

	//init docker client
	s.client, err = docker.NewClient(conf.DockerSock)
	if err != nil {
		blog.Errorf("new dockerclient %s failed: %s", conf.DockerSock, err.Error())
		return nil, err
	}

	//mkdir logconfig dir
	err = os.MkdirAll(conf.LogbeatDir, os.ModePerm)
	if err != nil {
		blog.Errorf("mkdir %s failed: %s", conf.LogbeatDir, err.Error())
		return nil, err
	}
	s.initLogConfigs()
	s.syncContainerCache()
	//init kubeconfig
	err = s.initKubeconfig()
	if err != nil {
		return nil, err
	}
	return s, nil
}

// Start starts the controller
func (s *SidecarController) Start() {
	go s.listenerDockerEvent()
	//go s.tickerSyncContainerLogConfs()
}

//start listen docker api event
//when create container, and produce container log config
//when stop container, and delete container log config
func (s *SidecarController) listenerDockerEvent() {
	listener := make(chan *docker.APIEvents)
	err := s.client.AddEventListener(listener)
	if err != nil {
		blog.Errorf("listen docker event error %s", err.Error())
		os.Exit(1)
	}
	defer func() {
		err = s.client.RemoveEventListener(listener)
		if err != nil {
			blog.Errorf("remove docker event error  %s", err.Error())
		}
	}()

	for {
		var msg *docker.APIEvents
		select {
		case msg = <-listener:
			blog.V(3).Infof("receive docker event action %s container %s", msg.Action, msg.ID)
		}

		switch msg.Action {
		//start container
		case "start":
			c := s.inspectContainer(msg.ID)
			if c == nil {
				blog.Errorf("inspect container %s failed", msg.ID)
				break
			}
			s.containerCacheMutex.Lock()
			s.containerCache[msg.ID] = c
			s.containerCacheMutex.Unlock()
			s.produceContainerLogConf(c)
			err = s.reloadLogbeat()
			if err != nil {
				blog.Errorf("reload logbeat failed: %s", err.Error())
			}
			blog.V(3).Infof("reload logbeat succ")

		// destroy container
		case "destroy":
			s.containerCacheMutex.Lock()
			delete(s.containerCache, msg.ID)
			s.containerCacheMutex.Unlock()
			s.Lock()
			s.deleteContainerLogConf(msg.ID)
			s.Unlock()
			err = s.reloadLogbeat()
			if err != nil {
				blog.Errorf("reload logbeat failed: %s", err.Error())
			}
			blog.V(3).Infof("reload logbeat succ")
		// stop container
		case "stop":
			s.Lock()
			s.deleteContainerLogConf(msg.ID)
			s.Unlock()
			err = s.reloadLogbeat()
			if err != nil {
				blog.Errorf("reload logbeat failed: %s", err.Error())
			}
			blog.V(3).Infof("reload logbeat succ")
		}
	}
}

func (s *SidecarController) syncLogConfs() {
	var hostIP string
	//list all running containers
	apiContainers, err := s.client.ListContainers(docker.ListContainersOptions{All: true})
	if err != nil {
		blog.Errorf("docker ListContainers failed: %s", err.Error())
		return
	}
	// generate container log config
	for i, apiC := range apiContainers {
		blog.V(4).Infof("index: %d, containerID: %s", i, apiC.ID)
		s.containerCacheMutex.RLock()
		c, ok := s.containerCache[apiC.ID]
		s.containerCacheMutex.RUnlock()
		if !ok {
			blog.Errorf("No container info (%s) in containercache", apiC.ID)
			continue
		}
		if !c.State.Running {
			blog.Infof("container (%s) is in state of (%s), not in running/paused/restarting, skipped", apiC.ID, c.State.StateString())
			continue
		}
		s.produceContainerLogConf(c)
		//Get host IP
		if hostIP != "" {
			continue
		}
		name := c.Config.Labels[ContainerLabelK8sContainerName]
		if name == "POD" || name == "" {
			continue
		}
		podName := c.Config.Labels[ContainerLabelK8sPodName]
		podNameSpace := c.Config.Labels[ContainerLabelK8sPodNameSpace]
		pod, err := s.podLister.Pods(podNameSpace).Get(podName)
		if err != nil {
			blog.Errorf("list pod(%s/%s) failed: %s", podNameSpace, podName, err.Error())
			continue
		}
		hostIP = pod.Status.HostIP
	}

	// generate host log config
	bcsLogConfigs, err := s.bcsLogConfigLister.List(labels.Everything())
	if err != nil {
		blog.Errorf("list bcslogconfig error %s", err.Error())
		return
	}
	for _, conf := range bcsLogConfigs {
		if conf.Spec.ConfigType == bcsv1.HostConfigType {
			s.produceHostLogConf(conf, hostIP)
		}
	}

	//remove invalid logconfig file
	s.removeInvalidLogConfigFile()
	err = s.reloadLogbeat()
	if err != nil {
		blog.Errorf("reload logbeat failed: %s", err.Error())
	}
	blog.V(3).Infof("reload logbeat succ")
}

func (s *SidecarController) removeInvalidLogConfigFile() {
	files, err := ioutil.ReadDir(s.conf.LogbeatDir)
	if err != nil {
		blog.Warnf("ReadDir %s failed: %s", s.conf.LogbeatDir, err.Error())
		return
	}

	for _, o := range files {
		confKey := fmt.Sprintf("%s/%s", s.conf.LogbeatDir, o.Name())
		s.RLock()
		config, ok := s.logConfs[confKey]
		s.RUnlock()
		if ok && config.yamlData != nil {
			continue
		}
		err := os.Remove(confKey)
		if err != nil {
			blog.Errorf("remove invalid logconfig file %s error %s", confKey, err.Error())
		} else {
			blog.Infof("remove invalid logconfig file %s success", confKey)
		}
	}
}

func (s *SidecarController) initLogConfigs() {
	fileList, err := ioutil.ReadDir(s.conf.LogbeatDir)
	if err != nil {
		blog.Errorf("initLogConfigs readdir %s failed: %s", s.conf.LogbeatDir, err.Error())
		return
	}

	for _, f := range fileList {
		key := fmt.Sprintf("%s/%s", s.conf.LogbeatDir, f.Name())
		by, err := ioutil.ReadFile(key)
		if err != nil {
			blog.Errorf("read file %s failed: %s", key, err.Error())
			continue
		}

		conf := &ContainerLogConf{
			confPath: key,
			data:     by,
		}
		s.logConfs[key] = conf
	}
}

func (s *SidecarController) getContainerLogConfKey(containerID string) string {
	return fmt.Sprintf("%s/%s-%s.%s", s.conf.LogbeatDir, s.prefixFile, []byte(containerID)[:12], s.conf.FileExtension)
}

func (s *SidecarController) getHostLogConfKey(logConf *bcsv1.BcsLogConfig) string {
	return fmt.Sprintf("%s/%s-%s-%s.%s", s.conf.LogbeatDir, s.prefixFile, logConf.GetNamespace(), logConf.GetName(), s.conf.FileExtension)
}

func (s *SidecarController) getBCSLogConfigKey(logConf *bcsv1.BcsLogConfig) string {
	return fmt.Sprintf("%s/%s", logConf.Namespace, logConf.Name)
}

func (s *SidecarController) produceContainerLogConf(c *docker.Container) {
	key := s.getContainerLogConfKey(c.ID)
	y, ok := s.produceLogConfParameterV2(c)

	//the container don't match any BcsLogConfig
	if !ok {
		s.Lock()
		defer s.Unlock()
		_, ok := s.logConfs[key]
		//if the container have logconfig, then delete it
		if ok {
			s.deleteContainerLogConf(c.ID)
			delete(s.logConfs, key)
		}
		return
	}
	s.writeLogConfFile(key, y)
}

func (s *SidecarController) produceHostLogConf(logConf *bcsv1.BcsLogConfig, hostIP string) {
	if logConf.Spec.NonStdDataId == "" || len(logConf.Spec.LogPaths) == 0 {
		blog.Errorf("host logconfig(%+v) didn't set NonStdDataId or LogPaths", logConf)
		return
	}
	y := &types.Yaml{
		Local:           make([]types.Local, 0),
		BCSLogConfigKey: s.getBCSLogConfigKey(logConf),
	}
	para := types.Local{
		ToJSON:  true,
		ExtMeta: make(map[string]string),
		Paths:   make([]string, 0),
	}
	para.ExtMeta["io_tencent_bcs_cluster"] = logConf.Spec.ClusterId
	para.ExtMeta["io_tencent_bcs_appid"] = logConf.Spec.AppId
	//custom log tags
	for k, v := range logConf.Spec.LogTags {
		para.ExtMeta[k] = v
	}
	dataid, err := strconv.Atoi(logConf.Spec.NonStdDataId)
	if err != nil {
		blog.Warnf("logconfig(%+v) has wrong type of NonStdDataID(%s): %s", logConf, logConf.Spec.NonStdDataId, err.Error())
		return
	}
	para.DataID = dataid
	for _, f := range logConf.Spec.LogPaths {
		if !filepath.IsAbs(f) {
			blog.Errorf("host logconf path specified as \"%s\" is not an absolute path", f)
			continue
		}
		para.Paths = append(para.Paths, s.getCleanPath(f))
	}
	y.Local = append(y.Local, para)
	// construct log file metric info
	y.Metric = &metric.LogFileInfoType{
		ClusterID:    strings.ToLower(logConf.Spec.ClusterId),
		CRDName:      logConf.GetName(),
		CRDNamespace: logConf.GetNamespace(),
		HostIP:       hostIP,
	}
	s.writeLogConfFile(s.getHostLogConfKey(logConf), y)
}

func (s *SidecarController) writeLogConfFile(key string, y *types.Yaml) {
	by, _ := yaml.Marshal(y)
	// get container id
	var cid string
	if y.Metric != nil {
		cid = y.Metric.ContainerID
	}
	//if log config exist, and not changed
	s.RLock()
	logConf, _ := s.logConfs[key]
	s.RUnlock()
	if logConf != nil {
		if logConf.yamlData != nil && logConf.yamlData.BCSLogConfigKey != "" && logConf.yamlData.BCSLogConfigKey != y.BCSLogConfigKey {
			blog.Errorf("Unexpected conflict config detected: BcsLogConfig %s and %s define log config for the same container(%s)",
				logConf.yamlData.BCSLogConfigKey, y.BCSLogConfigKey, cid)
		}
		if string(by) == string(logConf.data) {
			blog.Infof("container %s or host log config %s not changed", cid, logConf.confPath)
			if logConf.yamlData == nil {
				logConf.yamlData = y
				if err := y.Metric.Renew(); err != nil {
					blog.Errorf("Renew metric with label (%+v) failed: %s", *y.Metric, err.Error())
				}
			}
			return
		}
		blog.Infof("container %s or host log config %s changed, from(%s)->to(%s)", cid, logConf.confPath, string(logConf.data), string(by))
	} else {
		blog.Infof("container %s or host log config %s will created, and LogConfig(%s)", cid, key, string(by))
	}

	newlogConf := &ContainerLogConf{
		confPath: key,
		data:     by,
		yamlData: y,
	}
	f, err := os.Create(newlogConf.confPath)
	if err != nil {
		blog.Errorf("container %s or host open file %s failed: %s", cid, newlogConf.confPath, err.Error())
		return
	}
	defer f.Close()

	_, err = f.Write(by)
	if err != nil {
		blog.Errorf("container %s or host template execute failed: %s", cid, err.Error())
		return
	}
	blog.Infof("produce container %s or host log config %s success", cid, newlogConf.confPath)
	// Set/Update metric
	if logConf == nil || logConf.yamlData == nil {
		err := y.Metric.Set(1)
		if err != nil {
			blog.Errorf("Set metric with label (%+v) with value (1) failed: %s", *logConf.yamlData.Metric, err.Error())
		}
	} else {
		err := logConf.yamlData.Metric.Update(y.Metric)
		if err != nil {
			blog.Errorf("Update metric from label (%+v) to label (%+v) failed: %s", *logConf.yamlData.Metric, *y.Metric, err.Error())
		}
	}
	s.Lock()
	s.logConfs[key] = newlogConf
	s.Unlock()
}

func (s *SidecarController) deleteContainerLogConf(containerID string) {
	key := s.getContainerLogConfKey(containerID)
	logConf, ok := s.logConfs[key]
	if !ok {
		blog.Infof("container %s don't have LogConfig, then ignore", containerID)
		return
	}
	err := os.Remove(logConf.confPath)
	if err != nil {
		blog.Errorf("remove log config %s error %s", logConf.confPath, err.Error())
		return
	}
	if logConf.yamlData != nil {
		logConf.yamlData.Metric.Delete()
	}
	delete(s.logConfs, key)
	blog.Infof("delete container %s log config success", containerID)
}

// if need to collect the container logs, return true
// else return false
func (s *SidecarController) produceLogConfParameterV2(container *docker.Container) (*types.Yaml, bool) {
	//if container is network, ignore
	name := container.Config.Labels[ContainerLabelK8sContainerName]
	if name == "POD" || name == "" {
		blog.Infof("container %s is network container, ignore", container.ID)
		return nil, false
	}
	podName := container.Config.Labels[ContainerLabelK8sPodName]
	podNameSpace := container.Config.Labels[ContainerLabelK8sPodNameSpace]
	pod, err := s.podLister.Pods(podNameSpace).Get(podName)
	if err != nil {
		blog.Errorf("container %s fetch pod(%s:%s) error %s", container.ID, podName, podNameSpace, err.Error())
		return nil, false
	}

	logConf := s.getPodLogConfigCrd(container, pod)
	//if logConf==nil, container not match BcsLogConfig
	if logConf == nil {
		return nil, false
	}

	var matchedLogConfigs = make([]*bcsv1.ContainerConf, 0)
	if len(logConf.Spec.ContainerConfs) > 0 {
		for _, conf := range logConf.Spec.ContainerConfs {
			if conf.ContainerName == name {
				matchedConf := conf.DeepCopy()
				matchedLogConfigs = append(matchedLogConfigs, matchedConf)
			}
		}
	} else {
		var matchedLogConfig bcsv1.ContainerConf
		matchedLogConfig.Stdout = logConf.Spec.Stdout
		matchedLogConfig.StdDataId = logConf.Spec.StdDataId
		matchedLogConfig.NonStdDataId = logConf.Spec.NonStdDataId
		matchedLogConfig.LogPaths = logConf.Spec.LogPaths
		matchedLogConfig.HostPaths = logConf.Spec.HostPaths
		matchedLogConfig.LogTags = logConf.Spec.LogTags
		matchedLogConfigs = append(matchedLogConfigs, &matchedLogConfig)
	}

	y := &types.Yaml{
		Local:           make([]types.Local, 0),
		BCSLogConfigKey: s.getBCSLogConfigKey(logConf),
	}
	var (
		stdoutDataid  = ""
		referenceKind = ""
		referenceName = ""
	)
	if len(pod.OwnerReferences) != 0 {
		referenceKind = pod.OwnerReferences[0].Kind
		referenceName = pod.OwnerReferences[0].Name
	} else {
		referenceKind = "StaticPod"
	}
	for _, conf := range matchedLogConfigs {
		var para = types.Local{
			ExtMeta:          make(map[string]string),
			NonstandardPaths: make([]string, 0),
			Paths:            make([]string, 0),
			ToJSON:           true,
			OutputFormat:     s.conf.LogbeatOutputFormat,
		}
		if logConf.Spec.PackageCollection {
			pack := new(bool)
			*pack = true
			para.Package = pack
		}
		para.ExtMeta["io_tencent_bcs_cluster"] = logConf.Spec.ClusterId
		para.ExtMeta["io_tencent_bcs_pod"] = pod.Name
		para.ExtMeta["io_tencent_bcs_pod_ip"] = pod.Status.PodIP
		para.ExtMeta["io_tencent_bcs_namespace"] = pod.Namespace
		para.ExtMeta["io_tencent_bcs_server_name"] = referenceName
		para.ExtMeta["io_tencent_bcs_type"] = referenceKind
		para.ExtMeta["io_tencent_bcs_appid"] = logConf.Spec.AppId
		para.ExtMeta["io_tencent_bcs_projectid"] = pod.Labels["io.tencent.paas.projectid"]
		para.ExtMeta["container_id"] = container.ID
		para.ExtMeta["container_hostname"] = container.Config.Hostname
		//whether report pod labels to log tags
		if logConf.Spec.PodLabels {
			for k, v := range pod.Labels {
				para.ExtMeta[k] = v
			}
		}
		//custom log tags
		for k, v := range conf.LogTags {
			para.ExtMeta[k] = v
		}
		// generate std output log collection config
		if stdoutDataid == "" && conf.Stdout && conf.StdDataId != "" {
			stdPara := para
			id, err := strconv.Atoi(conf.StdDataId)
			if err != nil {
				blog.Errorf("Convert dataid from string(%s) to int failed: %s, BcsLogConfig(%+v)", conf.StdDataId, err.Error(), logConf)
				continue
			} else {
				stdPara.DataID = id
				stdPara.Paths = []string{container.LogPath}
				y.Local = append(y.Local, stdPara)
				stdoutDataid = conf.StdDataId
			}
		}
		if conf.NonStdDataId == "" {
			continue
		}
		// generate non std output log collection config
		id, err := strconv.Atoi(conf.NonStdDataId)
		if err != nil {
			blog.Errorf("Convert dataid from string(%s) to int failed: %s, BcsLogConfig(%+v)", conf.NonStdDataId, err.Error(), logConf)
			continue
		}
		para.DataID = id
		for _, f := range conf.LogPaths {
			actualPath, err := s.getActualPath(f, container)
			if err != nil {
				blog.Errorf("get actual path of %s with container (%+v) failed: %s", f, container, err.Error())
				continue
			}
			para.Paths = append(para.Paths, actualPath)
		}
		if len(para.Paths) == 0 {
			continue
		}
		y.Local = append(y.Local, para)
	}

	// construct log file metric info
	y.Metric = &metric.LogFileInfoType{
		ClusterID:         strings.ToLower(logConf.Spec.ClusterId),
		CRDName:           logConf.GetName(),
		CRDNamespace:      logConf.GetNamespace(),
		HostIP:            pod.Status.HostIP,
		ContainerID:       container.ID,
		PodName:           pod.GetName(),
		PodNamespace:      pod.GetNamespace(),
		WorkloadType:      referenceKind,
		WorkloadName:      referenceName,
		WorkloadNamespace: pod.GetNamespace(),
	}

	return y, true
}

func (s *SidecarController) getCleanPath(path string) string {
	if !s.conf.EvalSymlink {
		return path
	}
	runes := []rune(path)
	wildcardPos := s.getFirstWildcardPos(path)
	slashPos := strings.LastIndex(string(runes[:wildcardPos]), string(os.PathSeparator))
	cleanPath, err := filepath.EvalSymlinks(string(runes[:(slashPos + 1)]))
	if err != nil {
		blog.Warnf("EvalSymlinks of path %s failed: %s", string(runes[:(slashPos+1)]), err.Error())
	} else {
		path = cleanPath + string(runes[slashPos:])
	}
	return path
}

func (s *SidecarController) getFirstWildcardPos(str string) int {
	var pos = len(str)
	ind := strings.Index(str, "*")
	if ind != -1 && ind < pos {
		pos = ind
	}
	ind = strings.Index(str, "[")
	if ind != -1 && ind < pos {
		pos = ind
	}
	ind = strings.Index(str, "?")
	if ind != -1 && ind < pos {
		pos = ind
	}
	return pos
}

// getContainerRootPath return the root path of the container
// Usually it begins with /data/bcs/lib/docker/overlay2/{hashid}/merged
// If the container does not use OverlayFS, it will return /proc/{procid}/root
func (s *SidecarController) getContainerRootPath(container *docker.Container) string {
	switch container.Driver {
	case "overlay2":
		return container.GraphDriver.Data["MergedDir"]
	default:
		// blog.Warnf("Container %s has driver %s not overlay2", container.ID, container.Driver)
		return fmt.Sprintf("/proc/%d/root", container.State.Pid)
	}
}
