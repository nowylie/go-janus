package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
)

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

type Gateway struct {
	conn            net.Conn
	nextTransaction uint64
	replyChans      map[uint64]chan map[string]interface{}
	sessions        map[uint64]*Session
	sync.Mutex
}

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

	_, err = gw.conn.Write(data)
	if err != nil {
		fmt.Printf("conn.Write: %s\n", err)
		return
	}
}

func (gw *Gateway) recv() {
	buffer := make([]byte, 8192)

	for {
		// Read message from Gateway
		n, err := gw.conn.Read(buffer)
		if err != nil {
			fmt.Printf("conn.Read: %s\n", err)
			return
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
		// FIXME: log any messages without a 'transaction' field
		transactionStr := msg["transaction"].(string)
		transaction, err := strconv.ParseUint(transactionStr, 10, 64)
		gw.Lock()
		replyChan := gw.replyChans[transaction]
		gw.Unlock()

		if replyChan != nil {
			replyChan <- msg
		}
	}
}

func (gw *Gateway) Info() map[string]interface{} {
	info, ch := newRequest("info")
	gw.send(info, ch)
	return <-ch
}

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

func (s *Session) Attach(plugin string) (*Handle, error) {
	attach, ch := newRequest("attach")
	attach["plugin"] = plugin
	s.send(attach, ch)
	response := <-ch

	if err := checkError(response); err != nil {
		return nil, err
	}

	data := response["data"].(map[string]interface{})
	handle := &Handle{s, uint64(data["id"].(float64))}
	s.Lock()
	s.handles[handle.id] = handle
	s.Unlock()
	return handle, nil
}

func (s *Session) KeepAlive() (map[string]interface{}, error) {
	keepalive, ch := newRequest("keepalive")
	s.send(keepalive, ch)
	response := <-ch

	if err := checkError(response); err != nil {
		return nil, err
	}

	return response, nil
}

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

type Handle struct {
	session *Session
	id      uint64
}

func (h *Handle) send(msg map[string]interface{}, replyChan chan map[string]interface{}) {
	msg["handle_id"] = h.id
	h.session.send(msg, replyChan)
}

func (h *Handle) Message(body, jsep map[string]interface{}) (map[string]interface{}, error) {
	message, ch := newRequest("message")
	if body != nil {
		message["body"] = body
	}
	if jsep != nil {
		message["jsep"] = jsep
	}
	h.send(message, ch)

	response := <-ch
	if err := checkError(response); err != nil {
		return nil, err
	}
	return response["plugindata"].(map[string]interface{}), nil
}

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

func main() {
	if len(os.Args) < 2 {
		fmt.Printf("usage: janus-echotest </path/to/socket>\n")
		return
	}

	gateway, err := Connect(os.Args[1])
	if err != nil {
		fmt.Printf("Connect: %s\n")
		return
	}

	info := gateway.Info()
	fmt.Printf("Janus version %s, running on %s\n", info["version_string"], info["local-ip"])

	session, err := gateway.Create()
	if err != nil {
		fmt.Printf("Create: %s\n", err)
		return
	}

	handle, err := session.Attach("janus.plugin.echotest")
	if err != nil {
		fmt.Printf("Attach: %s\n", err)
		return
	}

	err = handle.Detach()
	if err != nil {
		fmt.Printf("Detach: %s\n", err)
		return
	}

	err = session.Destroy()
	if err != nil {
		fmt.Printf("Destroy: %s\n", err)
		return
	}

	err = gateway.Close()
	if err != nil {
		fmt.Printf("Close: %s\n", err)
		return
	}
}
