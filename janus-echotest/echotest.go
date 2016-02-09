package main

import (
	"fmt"
	"github.com/nowylie/go-janus/janus"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Printf("usage: janus-echotest </path/to/socket>\n")
		return
	}

	gateway, err := janus.Connect(os.Args[1])
	if err != nil {
		fmt.Printf("Connect: %s\n")
		return
	}

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
	if handle != nil {
		fmt.Printf("handle created\n")
	}

	//err = handle.Detach()
	//if err != nil {
	//	fmt.Printf("Detach: %s\n", err)
	//	return
	//}

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
