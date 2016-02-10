package main

import (
	"encoding/json"
	"fmt"
	"github.com/nowylie/go-janus/janus"
	"net/http"
	"os"
	"time"
)

var gateway *janus.Gateway

func main() {
	var err error

	if len(os.Args) < 2 {
		fmt.Printf("usage: janus-echotest </path/to/socket>\n")
		return
	}

	gateway, err = janus.Connect(os.Args[1])
	if err != nil {
		fmt.Printf("Connect: %s\n")
		return
	}

	http.HandleFunc("/", EchoTest)
	http.ListenAndServe(":8080", nil)
}

type Request struct {
	Body       interface{}
	Offer      interface{}
	Candidates []interface{}
}

func EchoTest(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		_, err := w.Write([]byte(echoHtml))
		if err != nil {
			fmt.Printf("http.ResponseWriter.Write: %s\n", err)
			return
		}
		return
	}

	session, err := gateway.Create()
	if err != nil {
		fmt.Printf("gateway.Create: %s\n", err)
		return
	}

	stop := make(chan interface{})
	go keepalive(session, stop)

	handle, err := session.Attach("janus.plugin.echotest")
	if err != nil {
		fmt.Printf("session.Attach: %s\n", err)
		return
	}
	go watch(session, handle, stop)

	decoder := json.NewDecoder(r.Body)
	var request Request
	err = decoder.Decode(&request)
	if err != nil {
		fmt.Printf("json.Unmarshal: %s\n", err)
		return
	}

	response, err := handle.Message(request.Body, nil)
	if err != nil {
		fmt.Printf("handle.Message: %s\n", err)
		return
	}

	response, err = handle.Message(request.Body, request.Offer)
	if err != nil {
		fmt.Printf("handle.Message: %s\n", err)
		return
	}

	out, err := json.Marshal(response["jsep"])
	if err != nil {
		fmt.Printf("json.Marshal: %s\n", err)
		return
	}
	w.Write(out)

	err = handle.TrickleMany(request.Candidates)
	if err != nil {
		fmt.Printf("handle.Trickle: %s\n", err)
		return
	}
}

func watch(session *janus.Session, handle *janus.Handle, stop chan interface{}) {
	for {
		event := <-handle.Events
		if event["janus"] == "media" && event["receiving"] == "false" {
			handle.Detach()
			session.Destroy()
			close(stop)
		}
	}
}

func keepalive(session *janus.Session, stop chan interface{}) {
	ticker := time.NewTicker(time.Second * 30)

	for {
		select {
		case <-ticker.C:
			session.KeepAlive()
		case <-stop:
			ticker.Stop()
			return
		}
	}
}
