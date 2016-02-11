package main

import (
	"encoding/json"
	"fmt"
	"github.com/burntsushi/toml"
	"github.com/nowylie/go-janus/janus"
	"html/template"
	"net/http"
	"os"
	"time"
)

var gateway *janus.Gateway

type Config struct {
	Port    uint
	Html    string
	Path    string
	Servers []Server `toml:"iceServer"`
}

type Server struct {
	Urls       string `json:"urls"`
	Username   string `json:"username,omitempty"`
	Credential string `json:"password,omitempty"`
}

var config Config
var output *template.Template

func main() {
	var err error

	if len(os.Args) < 2 {
		fmt.Printf("usage: janus-echotest </path/to/config>\n")
		return
	}

	if _, err := toml.DecodeFile(os.Args[1], &config); err != nil {
		fmt.Printf("toml.Decode: %s\n", err)
		return
	}

	output, err = template.ParseFiles(config.Html)
	if err != nil {
		fmt.Printf("template.ParseFiles: %s\n", err)
		return
	}

	gateway, err = janus.Connect(config.Path)
	if err != nil {
		fmt.Printf("Connect: %s\n")
		return
	}

	http.HandleFunc("/", EchoTest)
	http.ListenAndServe(fmt.Sprintf(":%d", config.Port), nil)
}

type Request struct {
	Body       interface{}
	Offer      interface{}
	Candidates []interface{}
}

func EchoTest(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("%s %s %s\n", r.Method, r.URL, r.Proto)
	if r.Method == "GET" {
		servers, err := json.Marshal(config.Servers)
		if err != nil {
			fmt.Printf("json.Marshal: %s\n", err)
			return
		}

		err = output.Execute(w, template.JS(servers))
		if err != nil {
			fmt.Printf("template.Execute: %s\n", err)
			return
		}
		return
	}

	session, err := gateway.Create()
	if err != nil {
		fmt.Printf("gateway.Create: %s\n", err)
		return
	}

	stop := make(chan struct{})
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

	event, err := handle.Message(request.Body, nil)
	if err != nil {
		fmt.Printf("handle.Message: %s\n", err)
		return
	}

	event, err = handle.Message(request.Body, request.Offer)
	if err != nil {
		fmt.Printf("handle.Message: %s\n", err)
		return
	}

	out, err := json.Marshal(event.Jsep)
	if err != nil {
		fmt.Printf("json.Marshal: %s\n", err)
		return
	}
	w.Write(out)

	_, err = handle.TrickleMany(request.Candidates)
	if err != nil {
		fmt.Printf("handle.Trickle: %s\n", err)
		return
	}
}

func watch(session *janus.Session, handle *janus.Handle, stop chan struct{}) {
	for {
		msg := <-handle.Events
		switch msg := msg.(type) {
		case *janus.MediaMsg:
			if msg.Receiving == "false" {
				handle.Detach()
				session.Destroy()
				close(stop)
				return
			}
		}
	}
}

func keepalive(session *janus.Session, stop chan struct{}) {
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
