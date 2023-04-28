// Copyright 2023 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package newrelic_security_agent

import (
	"fmt"
	"path/filepath"

	"os"
	"runtime"
	"time"

	logging "github.com/newrelic/csec-go-agent/internal/security_logs"
	secUtils "github.com/newrelic/csec-go-agent/internal/security_utils"
	secConfig "github.com/newrelic/csec-go-agent/security_config"
	"github.com/sirupsen/logrus"
)

var logger *logrus.Entry

const (
	LOG_FILE        = "go-security-collector.log"
	INIT_LOG_FILE   = "go-security-collector-init.log"
	STATUS_LOG_FILE = "go-security-collector-status-%s.log"
	SECURITY_HOME   = "nr-security-home"
)

func checkDefaultConfig() {
	if secConfig.GlobalInfo.Security.Validator_service_url == "" {
		secConfig.GlobalInfo.Security.Validator_service_url = "wss://csec.nr-data.net"
	}
}

func initLogger(logFilePath string, isDebugLog bool) {
	logFilePath = filepath.Join(logFilePath, SECURITY_HOME, "logs")
	logLevel := "INFO"
	if isDebugLog {
		logLevel = "DEBUG"
	}
	logging.SetLogLevel(logLevel)
	logging.Init(LOG_FILE, INIT_LOG_FILE, logFilePath, os.Getpid())
	logger = logging.GetLogger("Init")
}

func initApplicationInfo(appName string) {
	secConfig.GlobalInfo.ApplicationInfo.AppUUID = secUtils.GetUniqueUUID()
	secConfig.GlobalInfo.ApplicationInfo.AppName = appName
	secConfig.GlobalInfo.ApplicationInfo.Pid = secUtils.IntToString(os.Getpid())
	binaryPath, err := os.Executable()
	if err != nil {
		binaryPath = os.Args[0]
	}
	secConfig.GlobalInfo.ApplicationInfo.BinaryPath = binaryPath
	secConfig.GlobalInfo.ApplicationInfo.Sha256 = secUtils.CalculateSha256(binaryPath)
	secConfig.GlobalInfo.ApplicationInfo.Cmd = os.Args[0]
	secConfig.GlobalInfo.ApplicationInfo.Cmdline = os.Args[0:]
	startTime := time.Now().Unix() * 1000
	secConfig.GlobalInfo.ApplicationInfo.Starttimestr = secUtils.Int64ToString(startTime)
	secConfig.GlobalInfo.ApplicationInfo.Size = secUtils.CalculateFileSize(binaryPath)

	logger.Infoln("Collector is now inactive for ", secConfig.GlobalInfo.ApplicationInfo.AppUUID)
	printlogs := fmt.Sprintf("go secure agent attached to process: PID = %s, with generated applicationUID = %s by STATIC attachment", secUtils.IntToString(os.Getpid()), secConfig.GlobalInfo.ApplicationInfo.AppUUID)
	logging.NewStage("3", "PROTECTION", printlogs)
	logging.EndStage("3", "PROTECTION")
}

func initEnvironmentInfo() {

	secConfig.GlobalInfo.EnvironmentInfo.CollectorIp = secUtils.FindIpAddress()
	secConfig.GlobalInfo.EnvironmentInfo.Wd = secUtils.GetCurrentWorkingDir()
	secConfig.GlobalInfo.EnvironmentInfo.Goos = runtime.GOOS
	secConfig.GlobalInfo.EnvironmentInfo.Goarch = runtime.GOARCH
	secConfig.GlobalInfo.EnvironmentInfo.Gopath = secUtils.GetGoPath()
	secConfig.GlobalInfo.EnvironmentInfo.Goroot = secUtils.GetGoRoot()

	env_type, cid, err := secUtils.GetContainerId()
	if err != nil {
		logger.Errorln(err)
	}
	if !env_type {
		secConfig.GlobalInfo.EnvironmentInfo.RunningEnv = "HOST"
	} else {
		secConfig.GlobalInfo.EnvironmentInfo.ContainerId = cid
		if secUtils.IsKubernetes() {
			secConfig.GlobalInfo.EnvironmentInfo.RunningEnv = "KUBERNETES"
			secConfig.GlobalInfo.EnvironmentInfo.Namespaces = secUtils.GetKubernetesNS()
			secConfig.GlobalInfo.EnvironmentInfo.PodId = secUtils.GetPodId()
		} else if secUtils.IsECS() {
			secConfig.GlobalInfo.EnvironmentInfo.RunningEnv = "ECS"
			secConfig.GlobalInfo.EnvironmentInfo.EcsTaskId = secUtils.GetEcsTaskId()
			err, ecsData := secUtils.GetECSInfo()
			if err == nil {
				secConfig.GlobalInfo.EnvironmentInfo.ImageId = ecsData.ImageID
				secConfig.GlobalInfo.EnvironmentInfo.Image = ecsData.Image
				secConfig.GlobalInfo.EnvironmentInfo.ContainerName = ecsData.Labels.ComAmazonawsEcsContainerName
				secConfig.GlobalInfo.EnvironmentInfo.EcsTaskDefinition = ecsData.Labels.ComAmazonawsEcsTaskDefinitionFamily + ":" + ecsData.Labels.ComAmazonawsEcsTaskDefinitionVersion
			} else {
				logger.Errorln(err)
			}
		} else {
			secConfig.GlobalInfo.EnvironmentInfo.RunningEnv = "CONTAINER"
		}
	}
	logging.NewStage("2", "ENV", "Current environment variables")
	logging.PrintInitlog("Current environment variables : "+secUtils.StructToString(secConfig.GlobalInfo.EnvironmentInfo), "ENV")
	//logging.PrintInitlog("Current securety config/policy : "+secUtils.StructToString(security), "ENV")
	logging.EndStage("2", "ENV")
}

func initSecurityAgent(applicationName, licenseKey string, isDebugLog bool, securityAgentConfig secConfig.Security) {
	if secConfig.GlobalInfo.IsForceDisable {
		return
	}
	secConfig.GlobalInfo.Security = securityAgentConfig
	secConfig.GlobalInfo.ApplicationInfo.ApiAccessorToken = licenseKey
	secConfig.GlobalInfo.Security.SecurityHomePath = secUtils.GetCurrentWorkingDir()
	checkDefaultConfig()
	initLogger(secConfig.GlobalInfo.Security.SecurityHomePath, isDebugLog)
	initEnvironmentInfo()
	initApplicationInfo(applicationName)

}