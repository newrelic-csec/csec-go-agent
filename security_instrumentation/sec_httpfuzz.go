// Copyright 2023 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: New Relic Pre-Release

package security_instrumentation

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	secConfig "github.com/newrelic/csec-go-agent/security_config"
	eventGeneration "github.com/newrelic/csec-go-agent/security_event_generation"
	sechandler "github.com/newrelic/csec-go-agent/security_handlers"
	secIntercept "github.com/newrelic/csec-go-agent/security_intercept"
)

var (
	clientMu    sync.Mutex
	httpClient  *http.Client
	httpsClient *http.Client
)

const (
	HTTP  = "http://"
	HTTPS = "https://"
	GET   = "GET"
	POST  = "POST"
)

type SecHttpFuzz struct {
}

func getHttpClient() *http.Client {
	clientMu.Lock()
	defer clientMu.Unlock()
	if httpClient == nil {
		httpClient = &http.Client{Timeout: time.Second * 10}
	}
	return httpClient
}

func getHttpsClient() *http.Client {
	clientMu.Lock()
	defer clientMu.Unlock()
	if httpsClient == nil {
		httpsClient = &http.Client{Timeout: time.Second * 10}
	}
	return httpsClient
}

func (httpFuzz SecHttpFuzz) ExecuteFuzzRequest(fuzzRequest *sechandler.FuzzRequrestHandler, caseType string, fuzzId string) {
	fuzzRequestID := fmt.Sprintf("%v", fuzzRequest.Headers[secIntercept.NR_CSEC_FUZZ_REQUEST_ID])
	applicationPort := ":" + strconv.Itoa(fuzzRequest.ServerPort)
	fuzzRequestURL := secConfig.GlobalInfo.ApplicationInfo.GetServerIp() + applicationPort + fuzzRequest.Url
	var fuzzRequestClient *http.Client

	if fuzzRequest.Protocol == "https" {
		fuzzRequestURL = HTTPS + fuzzRequestURL
		fuzzRequestClient = getHttpsClient()
		fuzzRequestClient.Transport = getTransport(fuzzRequest.ServerName)
	} else {
		fuzzRequestURL = HTTP + fuzzRequestURL
		fuzzRequestClient = getHttpClient()
	}

	var req *http.Request = nil
	var err error = nil

	switch fuzzRequest.Method {

	case GET:
		req, err = http.NewRequest(GET, fuzzRequestURL, nil)
	case POST:
		req, err = http.NewRequest(POST, fuzzRequestURL, strings.NewReader(fuzzRequest.Body))
	default:
		logger.Errorln("Unimplemented request type", fuzzRequest.Method)
	}
	if req == nil || err != nil {
		eventGeneration.SendFuzzFailEvent(fuzzRequestID)
		return
	}
	req.URL.RawQuery = req.URL.Query().Encode()

	for headerKey, headerValue := range fuzzRequest.Headers {
		value := fmt.Sprintf("%v", headerValue)
		req.Header.Set(headerKey, value)
	}
	sechandler.FuzzHandler.AppendCompletedRequestIds(fuzzId, "")
	req.Header.Set("Content-Type", fuzzRequest.ContentType)
	req.Header.Set("nr-csec-parent-id", fuzzId)

	if fuzzRequestClient == nil {
		logger.Debugln("Blackops client = nil")
	}
	response, err := fuzzRequestClient.Do(req)
	if err != nil {
		logger.Debugln("ERROR: fuzz request fail : ", fuzzRequestID, err.Error())
		eventGeneration.SendFuzzFailEvent(fuzzRequestID)
	} else {
		defer response.Body.Close()
		logger.Debugln("fuzz request successful : ", fuzzRequestID)
	}
}

func getTransport(serverName string) *http.Transport {
	httpTransport := &http.Transport{
		Dial:                (&net.Dialer{Timeout: 5 * time.Second}).Dial,
		TLSHandshakeTimeout: 6 * time.Second,
		MaxIdleConns:        10,
		DisableCompression:  true}
	if serverName == "" {
		httpTransport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	} else {
		//https servers with certificate lazy loading
		httpTransport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true, ServerName: serverName}
	}
	return httpTransport
}
