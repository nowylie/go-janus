package janus

type Msg struct {
	Type    string `json:"janus"`
	Id      string `json:"transaction"`
	Session uint64 `json:"session_id"`
	Handle  uint64 `json:"sender"`
}

type ErrorMsg struct {
	Code   int
	Reason string
}

type SuccessMsg struct {
	Id uint64
}

type InfoMsg struct {
	Name          string
	Version       int
	VersionString string `json:"version_string"`
	Author        string
	DataChannels  string `json:"data_channels"`
	IPv6          string `json:"ipv6"`
	ICE_TCP       string `json:"ice-tcp"`
	Transports    map[string]PluginInfo
	Plugins       map[string]PluginInfo
}

type PluginInfo struct {
	Name          string
	Author        string
	Description   string
	Version       int
	VersionString string `json:"version_string"`
}

type AckMsg struct{}

type EventMsg struct {
	Plugin string
	Data   map[string]interface{}
	Jsep   JSEP
}

type JSEP struct {
	Type string
	SDP  string
}

type WebRTCUpMsg struct{}

type MediaMsg struct {
	Type      string
	Receiving bool
}

type HangupMsg struct {
	Reason string
}

var msgtypes = map[string]func() interface{}{
	"error":       func() interface{} { return &ErrorMsg{} },
	"success":     func() interface{} { return &SuccessMsg{} },
	"server_info": func() interface{} { return &InfoMsg{} },
	"ack":         func() interface{} { return &AckMsg{} },
	"event":       func() interface{} { return &EventMsg{} },
	"webrtcup":    func() interface{} { return &WebRTCUpMsg{} },
	"media":       func() interface{} { return &MediaMsg{} },
	"hangup":      func() interface{} { return &HangupMsg{} },
}
