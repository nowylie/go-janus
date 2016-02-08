package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"os"
)

func main() {
	upath := fmt.Sprintf("/tmp/janus-info.%d", os.Getpid())

	// Address of local socket
	laddr := &net.UnixAddr{upath, "unixgram"}
	// Address of janus socket
	raddr := &net.UnixAddr{os.Args[1], "unixgram"}

	// Create unix datagram socket
	conn, _ := net.DialUnix("unixgram", laddr, raddr)

	// Create info request
	info := make(map[string]string)
	info["janus"] = "info"
	info["transaction"] = "1234"

	// Marshal request to json and sent to Janus
	req, _ := json.Marshal(info)
	conn.Write(req)

	// Receive response
	res := make([]byte, 8192)
	n, _ := conn.Read(res)

	// Format output
	var out bytes.Buffer
	json.Indent(&out, res[:n], "", "\t")
	out.Write([]byte("\n"))

	// Write to stdout
	out.WriteTo(os.Stdout)

	// Cleanup local socket
	conn.Close()
	os.Remove(upath)
}
