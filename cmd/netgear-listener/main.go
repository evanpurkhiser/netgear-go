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

var output = map[netgear.DeviceChange]string{
	netgear.DeviceAdded:   "Device Added",
	netgear.DeviceRemoved: "Device Removed",
}

func listener(change *netgear.ChangedDevice, err error) {
	if err != nil {
		return
	}

	mac := change.Device.MAC.String()
	fmt.Printf(output[change.Change]+": %s\n", mac)
}

func main() {
	flag.Parse()

	client := netgear.NewClient(*host, *username, *password)

	pollTime := time.Second * 10
	client.OnDeviceChanged(pollTime, listener)

	<-make(chan bool)
}
