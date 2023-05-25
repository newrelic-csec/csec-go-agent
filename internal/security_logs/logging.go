// Copyright 2023 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: New Relic Pre-Release

package security_logs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	secUtils "github.com/newrelic/csec-go-agent/internal/security_utils"
)

var agentLogger = DefaultLogger(false)
var isInitilized = false
var errorBuffer = secUtils.NewCring(5)

func Init(logFileName, initlogFileName, logFilepath string, pid int) {
	isInitilized = true
	syscall.Umask(0)
	err := os.MkdirAll(logFilepath, os.ModePerm)
	if err != nil {
		fmt.Println(err)
		return
	}

	err = os.Chmod(logFilepath, 0777)
	if err != nil {
		fmt.Println(err)
		return
	}

	rotateFileHook, writer, err := NewRotateFileHook(RotateFileConfig{
		Filename:        filepath.Join(logFilepath, logFileName),
		MaxSize:         50, // megabytes
		MaxBackups:      2,
		BaseLogFilename: logFileName,
	})

	if err != nil {
		fmt.Println(err)
		return
	}
	UpdateLogger(writer, "INFO", pid, agentLogger, rotateFileHook)

	init_initLogger(initlogFileName, logFilepath, pid)
}

func SetLogLevel(level string) {
	if os.Getenv("NR_CSEC_DEBUG_MODE") == "true" {
		agentLogger.setLevel("DEBUG")
	} else {
		agentLogger.setLevel(strings.ToUpper(level))
	}

}
func GetLogger(loggerName string) *logFile {
	return agentLogger
}

func trackError(err string) {
	errorBuffer.ForceInsert(err)
}
func GetErrorLogs() interface{} {
	return errorBuffer.Get()
}

func IsHooked(name string, e error) {
	var logger = GetLogger("")
	var initlogs = InitLogger()
	if e != nil {
		print := fmt.Sprintf("functionName=%s error=%s", name, "Not able to hook function "+e.Error())
		logger.Errorln(print)
		initlogs.Errorln(print)
	} else {
		print := fmt.Sprintf("functionName=%s Function successfully hooked", name)
		logger.Infoln(print)
		initlogs.Infoln(print)
	}
}
