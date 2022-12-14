package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/aws/aws-lambda-go/lambda"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"tailscale.com/tsnet"
	"time"
)

func HandleRequest(ctx context.Context, event json.RawMessage) (json.RawMessage, error) {
	url := os.Getenv("BASE_URL")
	token_env, token_present := os.LookupEnv("TOKEN")
	sleep_env, sleep_present := os.LookupEnv("TS_DELAY")

	// allow setting tailscale startup delay via environment variable
	var ts_delay time.Duration
	if sleep_present {
		sleep_env_int, _ := strconv.Atoi(sleep_env)
		ts_delay = time.Duration(sleep_env_int) * time.Millisecond
	} else {
		ts_delay = 1500 * time.Millisecond
	}

	auth := "Bearer "
	// use token from environment if present
	if token_present {
		auth += token_env
	} else {
		var f interface{}
		_ = json.Unmarshal(event, &f)
		m := f.(map[string]interface{})["directive"].(map[string]interface{})["endpoint"].(map[string]interface{})["scope"].(map[string]interface{})
		auth += fmt.Sprint(m["token"])
	}

	// collect connection details for Home Assistant
	url += "/api/alexa/smart_home"
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(event))
	req.Header.Set("Authorization", auth)
	req.Header.Set("Content-Type", "application/json")

	// initiate tailscale connection
	s := &tsnet.Server{
		Dir:       "/tmp",
		Hostname:  "hass-alexa",
		Ephemeral: true,
	}
	defer s.Close()

	_, _ = s.LocalClient()
	client := &http.Client{
		Transport: &http.Transport{
			DialContext: s.Dial, // use the tailscale dialer
		},
	}
	time.Sleep(ts_delay) // wait for tailnet connection to come up

	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)

	return json.RawMessage(body), err
}

func main() {
	lambda.Start(HandleRequest)
}
