package netgear

import "time"

// DeviceChange repressents the change in the devices status
type DeviceChange int

// Device change statuses
const (
	DeviceAdded   DeviceChange = iota
	DeviceRemoved DeviceChange = iota
)

// ChangedDevice represents the device that has changed
type ChangedDevice struct {
	Device AttachedDevice
	Change DeviceChange
}

// DeviceListener is a callback for when a device is added or removed
type DeviceListener func(*ChangedDevice, error)

// OnDeviceChanged triggers a callback when a device is added or removed
func (c *Client) OnDeviceChanged(poll time.Duration, fn DeviceListener) *time.Ticker {
	ticker := time.NewTicker(poll)
	devices := []AttachedDevice{}

	getDevices := func() ([]AttachedDevice, error) {
		if err := c.Login(); err != nil {
			return nil, err
		}

		return c.Devices()
	}

	watcher := func() {
		for _ = range ticker.C {
			updatedDevices, err := getDevices()
			if err != nil {
				fn(nil, err)
				continue
			}

			changedDevices := getChangedDevices(devices, updatedDevices)
			for _, changedDevice := range changedDevices {
				fn(&changedDevice, nil)
			}

			devices = updatedDevices
		}
	}

	go watcher()

	return ticker
}

// Determine what devices were changed between two lists of attached devices
func getChangedDevices(oldDevices, newDevices []AttachedDevice) []ChangedDevice {
	change := []ChangedDevice{}
	diff := map[string]bool{}

	for _, dev := range oldDevices {
		diff[dev.MAC.String()] = true
	}

	// Find newly added devices
	for _, dev := range newDevices {
		if _, ok := diff[dev.MAC.String()]; !ok {
			change = append(change, ChangedDevice{dev, DeviceAdded})
			continue
		}

		// Device remains unchanged
		delete(diff, dev.MAC.String())
	}

	// Find removed devices
	for _, dev := range oldDevices {
		if _, ok := diff[dev.MAC.String()]; ok {
			change = append(change, ChangedDevice{dev, DeviceRemoved})
		}
	}

	return change
}
