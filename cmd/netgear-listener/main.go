package main

import (
	"flag"
	"fmt"
	"time"

	"go.evanpurkhiser.com/netgear"
)

var (
	host     = flag.String("host", "192.168.1.1", "Your netgear router address")
	username = flag.String("username", "admin", "Your netgear router username")
	password = flag.String("password", "", "Your netgear router password")
)

func main() {
	flag.Parse()

	client := netgear.NewClient(*host, *username, *password)

	listener := func(change *netgear.ChangedDevice, err error) {
		if err != nil {
			return
		}

		mac := change.Device.MAC.String()

		if change.Change == netgear.DeviceAdded {
			fmt.Printf("New device added: %s\n", mac)
		} else {
			fmt.Printf("Device removed: %s\n", mac)
		}
	}

	pollTime := time.Second * 10
	client.OnDeviceChanged(pollTime, listener)

	<-make(chan bool)
}
