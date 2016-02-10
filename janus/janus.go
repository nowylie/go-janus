// Package janus is a Golang implementation of the Janus API, used to interact
// with the Janus WebRTC Gateway.
package janus

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
)

var debug = false

func init() {
	if os.Getenv("DEBUG") != "" {
		debug = true
	}
}

func newRequest(method string) (map[string]interface{}, chan map[string]interface{}) {
	req := make(map[string]interface{})
	req["janus"] = method
	return req, make(chan map[string]interface{})
}

func checkError(msg map[string]interface{}) error {
	if msg["janus"].(string) == "error" {
		err := msg["error"].(map[string]interface{})
		return errors.New(err["reason"].(string))
	}
	return nil
}

// Gateway represents a connection to an instance of the Janus Gateway.
type Gateway struct {
	conn            net.Conn
	nextTransaction uint64
	replyChans      map[uint64]chan map[string]interface{}
	sessions        map[uint64]*Session
	sync.Mutex
}

// Connect creates a new Gateway instance, connected to the Janus Gateway.
// rpath should be a filesystem path to the Unix Socket that the Unix transport
// is bound to.
func Connect(rpath string) (*Gateway, error) {
	lpath := fmt.Sprintf("/tmp/janus-echotest.%d", os.Getpid())
	laddr := &net.UnixAddr{lpath, "unixgram"}
	raddr := &net.UnixAddr{rpath, "unixgram"}
	conn, err := net.DialUnix("unixgram", laddr, raddr)
	if err != nil {
		return nil, err
	}

	gw := new(Gateway)
	gw.conn = conn
	gw.replyChans = make(map[uint64]chan map[string]interface{})
	gw.sessions = make(map[uint64]*Session)

	go gw.recv()
	return gw, nil
}

func (gw *Gateway) Close() error {
	return gw.conn.Close()
}

func (gw *Gateway) send(msg map[string]interface{}, replyChan chan map[string]interface{}) {
	id := atomic.AddUint64(&gw.nextTransaction, 1)

	msg["transaction"] = strconv.FormatUint(id, 10)
	gw.Lock()
	gw.replyChans[id] = replyChan
	gw.Unlock()

	data, err := json.Marshal(msg)
	if err != nil {
		fmt.Printf("json.Marshal: %s\n", err)
		return
	}

	if debug {
		// log message being sent
		var log bytes.Buffer
		json.Indent(&log, data, ">", "   ")
		log.Write([]byte("\n"))
		log.WriteTo(os.Stdout)
	}

	_, err = gw.conn.Write(data)
	if err != nil {
		fmt.Printf("conn.Write: %s\n", err)
		return
	}
}

func (gw *Gateway) recv() {
	var log bytes.Buffer
	buffer := make([]byte, 8192)

	for {
		// Read message from Gateway
		n, err := gw.conn.Read(buffer)
		if err != nil {
			fmt.Printf("conn.Read: %s\n", err)
			return
		}

		if debug {
			// Log received message
			json.Indent(&log, buffer[:n], "<", "   ")
			log.Write([]byte("\n"))
			log.WriteTo(os.Stdout)
		}

		// Decode message
		var data interface{}
		err = json.Unmarshal(buffer[:n], &data)
		if err != nil {
			fmt.Printf("json.Unmarshal: %s\n", err)
			return
		}
		msg := data.(map[string]interface{})

		// Look up replyChan
		transactionStr, ok := msg["transaction"].(string)
		if !ok {
			session_id := uint64(msg["session_id"].(float64))
			if session_id == 0 {
				// error, no session_id
				continue
			}

			handle_id := uint64(msg["sender"].(float64))
			if handle_id == 0 {
				// error, no handle_id
				continue
			}

			// Lookup session
			gw.Lock()
			session := gw.sessions[session_id]
			gw.Unlock()
			if session == nil {
				// error, invalid session_id
				continue
			}

			session.Lock()
			handle := session.handles[handle_id]
			session.Unlock()
			if handle == nil {
				// error, invalid handle_id
				continue
			}

			handle.Events <- msg
			continue
		}

		transaction, err := strconv.ParseUint(transactionStr, 10, 64)
		gw.Lock()
		replyChan := gw.replyChans[transaction]
		gw.Unlock()

		if replyChan != nil {
			go func() {
				replyChan <- msg
			}()
		}
	}
}

// Info sends an info request to the Gateway.
// If a successful response is received, it is returned. Otherwise an error
// is returned.
func (gw *Gateway) Info() (map[string]interface{}, error) {
	info, ch := newRequest("info")
	gw.send(info, ch)
	response := <-ch

	if err := checkError(response); err != nil {
		return nil, err
	}

	return response, nil
}

// Create sends a create request to the Gateway.
// If a successful response is received, a local Session instance is setup
// with the returned session_id. Otherwise an error is returned.
func (gw *Gateway) Create() (*Session, error) {
	create, ch := newRequest("create")
	gw.send(create, ch)
	response := <-ch

	if err := checkError(response); err != nil {
		return nil, err
	}
	data := response["data"].(map[string]interface{})

	// Create new session
	session := new(Session)
	session.gateway = gw
	session.id = uint64(data["id"].(float64))
	session.handles = make(map[uint64]*Handle)

	// Store this session
	gw.Lock()
	gw.sessions[session.id] = session
	gw.Unlock()

	return session, nil
}

// Session represents a session instance on the Janus Gateway.
type Session struct {
	gateway *Gateway
	id      uint64
	handles map[uint64]*Handle
	sync.Mutex
}

func (s *Session) send(msg map[string]interface{}, replyChan chan map[string]interface{}) {
	msg["session_id"] = s.id
	s.gateway.send(msg, replyChan)
}

// Attach sends an attach request to the Gateway within this session.
// plugin should be the unique string of the plugin to attach to.
// If a successful response is received, a local Handle instance is created
// with the returned handle_id. Otherwise an error is returned.
func (s *Session) Attach(plugin string) (*Handle, error) {
	attach, ch := newRequest("attach")
	attach["plugin"] = plugin
	s.send(attach, ch)
	response := <-ch

	if err := checkError(response); err != nil {
		return nil, err
	}

	data := response["data"].(map[string]interface{})
	handle := new(Handle)
	handle.session = s
	handle.id = uint64(data["id"].(float64))
	handle.Events = make(chan map[string]interface{}, 8)
	s.Lock()
	s.handles[handle.id] = handle
	s.Unlock()
	return handle, nil
}

// KeepAlive sends a keep-alive request to the Gateway.
func (s *Session) KeepAlive() (map[string]interface{}, error) {
	keepalive, ch := newRequest("keepalive")
	s.send(keepalive, ch)
	response := <-ch

	if err := checkError(response); err != nil {
		return nil, err
	}

	return response, nil
}

// Destroy sends a destroy request to the Gateway.
// If a successful response is received, the local Session instance is cleaned
// up. Otherwise an error is returned.
func (s *Session) Destroy() error {
	destroy, ch := newRequest("destroy")
	s.send(destroy, ch)

	response := <-ch
	if err := checkError(response); err != nil {
		return err
	}

	// Remove this session from the gateway
	s.gateway.Lock()
	delete(s.gateway.sessions, s.id)
	s.gateway.Unlock()

	return nil
}

// Handle represents a handle to a plugin instance on the Gateway.
type Handle struct {
	session *Session
	id      uint64
	Events  chan map[string]interface{}
}

func (h *Handle) send(msg map[string]interface{}, replyChan chan map[string]interface{}) {
	msg["handle_id"] = h.id
	h.session.send(msg, replyChan)
}

// Message sends a message request to a plugin handle on the Gateway.
// body should be the plugin data to be passed to the plugin, and jsep should
// contain an optional SDP offer/answer to establish a WebRTC PeerConnection.
// If a successful response is received, it is returned. Othwerise an error is
// returned.
func (h *Handle) Message(body, jsep interface{}) (map[string]interface{}, error) {
	message, ch := newRequest("message")
	if body != nil {
		message["body"] = body
	}
	if jsep != nil {
		message["jsep"] = jsep
	}
	h.send(message, ch)

	// FIXME: handle 'ack' messages
GetResponse:
	response := <-ch
	if err := checkError(response); err != nil {
		return nil, err
	}

	if response["janus"].(string) == "ack" {
		goto GetResponse
	}

	return response, nil
}

// Trickle sends a trickle request to the Gateway as part of the ICE process.
// candidate should be a single ICE candidate, an Array of ICE candidates, or
// a completed object: {"completed": true}.
// If an error is received in response to this request, it is returned.
func (h *Handle) Trickle(candidate interface{}) error {
	trickle, ch := newRequest("trickle")
	trickle["candidate"] = candidate
	h.send(trickle, ch)

	response := <-ch
	if err := checkError(response); err != nil {
		return err
	}

	return nil
}

func (h *Handle) TrickleMany(candidate interface{}) error {
	trickle, ch := newRequest("trickle")
	trickle["candidates"] = candidate
	h.send(trickle, ch)

	response := <-ch
	if err := checkError(response); err != nil {
		return err
	}

	return nil
}

// Detach sends a detach request to the Gateway.
// If a successful response is received, the local Handle instance is cleaned
// up. Otherwise an error is returned.
func (h *Handle) Detach() error {
	detach, ch := newRequest("detach")
	h.send(detach, ch)

	response := <-ch
	if err := checkError(response); err != nil {
		return err
	}

	// Remove this handle from the session
	h.session.Lock()
	delete(h.session.handles, h.id)
	h.session.Unlock()

	return nil
}
