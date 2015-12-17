package airbrake

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"runtime"
	"strings"
	"time"

	"github.com/davecgh/go-spew/spew"
	"golang.org/x/net/context"
)

type Airbrake struct {
	appID     string
	appKey    string
	transport *http.Transport
}

type notifier struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	URL     string `json:"url"`
}

const airbrakeEndpoint = "https://airbrake.io/api/v3/projects/%s/notices?key=%s"

var (
	not = notifier{
		"Goware Airbrake",
		"0.0.1",
		"https://github.com/goware/airbrake",
	}
)

type AirbrakeError struct {
	Type      string             `json:"type"`
	Message   string             `json:"message,omitempty"`
	Backtrace []*StacktraceFrame `json:"backtrace"`
}

type AirbrakeContext struct {
	OS            string `json:"os,omitempty"`
	Language      string `json:"language,omitempty"`
	Environment   string `json:"environment,omitempty"`
	Version       string `json:"version,omitempty"`
	URL           string `json:"url,omitempty"`
	Action        string `json:"action,omitempty"`
	RootDirectory string `json:"rootDirectory,omitempty"`
	UserID        string `json:"userId,omitempty"`
	UserName      string `json:"userName,omitempty"`
	UserEmail     string `json:"userEmail,omitempty"`
	UserAgent     string `json:"userAgent,omitempty"`
}

type AirbrakeNotification struct {
	Notifier    notifier          `json:"notifier"`
	Errors      []AirbrakeError   `json:"errors"`
	Context     AirbrakeContext   `json:"context,omitempty"`
	Environment map[string]string `json:"environment,omitempty"`
	Session     map[string]string `json:"session,omitempty"`
	Params      map[string]string `json:"params,omitempty"`
}

func NewAirbrake(appID, appKey string) *Airbrake {
	a := Airbrake{
		appID:  appID,
		appKey: appKey,
		transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			Dial: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).Dial,
			TLSHandshakeTimeout: 10 * time.Second,
			TLSClientConfig: &tls.Config{
				ClientSessionCache: tls.NewLRUClientSessionCache(1024),
			},
			MaxIdleConnsPerHost:   100,
			ResponseHeaderTimeout: 10 * time.Second,
		},
	}
	return &a
}

func (a *Airbrake) Send(e *Event, timeout time.Duration) error {
	httpClient := &http.Client{
		Transport: a.transport,
		Timeout:   timeout,
	}

	b, err := json.Marshal(event2Airbrake(e))
	if err != nil {
		return err
	}

	response, err := httpClient.Post(fmt.Sprintf(airbrakeEndpoint, a.appID, a.appKey), "application/json", bytes.NewReader(b))

	if err != nil {
		defer response.Body.Close()
		body, _ := ioutil.ReadAll(response.Body)
		log.Printf("response: %s", body)

	}
	return err
}

func event2Airbrake(e *Event) *AirbrakeNotification {
	aErr := AirbrakeError{
		Type:      e.Type,
		Message:   e.Stacktrace.Culprit(),
		Backtrace: e.Stacktrace.Frames,
	}

	n := AirbrakeNotification{
		Notifier: not,
		Errors:   []AirbrakeError{aErr},
		Context: AirbrakeContext{
			OS:       runtime.GOOS + " " + runtime.GOARCH,
			Language: runtime.Version(),
		},
		Params: make(map[string]string),
	}
	if req, ok := e.Context["http.request"].(*Http); ok {
		if params, err := url.ParseQuery(req.Query); err != nil {
			n.Params = make(map[string]string)
			for param, vals := range params {
				n.Params[param] = strings.Join(vals, ",")
			}
		}
		n.Context.UserAgent = req.UserAgent
		n.Context.Action = req.Method

		if len(req.Headers) > 0 {
			for header, val := range req.Headers {
				n.Params["request.header: "+header] = val
			}
		}

		if len(req.Cookies) > 0 {
			for _, c := range req.Cookies {
				n.Params["request.cookie: "+c.Name] = c.String()
			}
		}
	}
	if ctx, ok := e.Context["http.ctx"].(context.Context); ok {
		n.Params["request.context"] = spew.Sdump(ctx)
	}

	return &n
}
