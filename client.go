package netgear

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"text/template"
)

const soapLogin = `
<?xml version="1.0" encoding="utf-8" ?>
<SOAP-ENV:Envelope xmlns:SOAP-ENV="http://schemas.xmlsoap.org/soap/envelope/">
<SOAP-ENV:Header>
<SessionID xsi:type="xsd:string"
  xmlns:xsi="http://www.w3.org/1999/XMLSchema-instance">{{.sessionID}}</SessionID>
</SOAP-ENV:Header>
<SOAP-ENV:Body>
<Authenticate>
  <NewUsername>{{.username}}</NewUsername>
  <NewPassword>{{.password}}</NewPassword>
</Authenticate>
</SOAP-ENV:Body>
</SOAP-ENV:Envelope>`

const soapAttachedDev = `
<?xml version="1.0" encoding="utf-8" standalone="no"?>
<SOAP-ENV:Envelope xmlns:SOAPSDK1="http://www.w3.org/2001/XMLSchema"
  xmlns:SOAPSDK2="http://www.w3.org/2001/XMLSchema-instance"
  xmlns:SOAPSDK3="http://schemas.xmlsoap.org/soap/encoding/"
  xmlns:SOAP-ENV="http://schemas.xmlsoap.org/soap/envelope/">
<SOAP-ENV:Header>
<SessionID>{{.sessionID}}</SessionID>
</SOAP-ENV:Header>
<SOAP-ENV:Body>
<M1:GetAttachDevice xmlns:M1="urn:NETGEAR-ROUTER:service:DeviceInfo:1">
</M1:GetAttachDevice>
</SOAP-ENV:Body>
</SOAP-ENV:Envelope>`

// DefaultSessionID is  taken from the pynetgear library. Apparently it's
// unknown how to generate this
const DefaultSessionID = "A7D88AE69687E58D9A00"

type soapAction string

const (
	loginAction       soapAction = "urn:NETGEAR-ROUTER:service:ParentalControl:1#Authenticate"
	attachedDevAction soapAction = "urn:NETGEAR-ROUTER:service:DeviceInfo:1#GetAttachDevice"
)

var (
	loginTemplate, _       = template.New("login").Parse(soapLogin)
	attachedDevTemplate, _ = template.New("attachedDev").Parse(soapAttachedDev)
)

// Map actions to the templates they should render
var soapTemplates = map[soapAction]*template.Template{
	loginAction:       loginTemplate,
	attachedDevAction: attachedDevTemplate,
}

type soapResponseCode struct {
	ResponseCode int `xml:"ResponseCode"`
}

// AttachedDevice represents a device attached to the router
type AttachedDevice struct {
	IP       net.IP
	Name     string
	MAC      net.HardwareAddr
	Type     string
	LinkRate int
	Signal   int
}

// Client is a API client used to talk to a netgear router
type Client struct {
	SessionID string
	Host      string
	Port      int
	Username  string
	Password  string
}

// NewClient constructs a new netgear.Client initalized with default values
func NewClient(host, username, password string) *Client {
	return &Client{
		SessionID: DefaultSessionID,
		Host:      host,
		Port:      5000,
		Username:  username,
		Password:  password,
	}
}

func (c *Client) soap(action soapAction, params interface{}) (*http.Response, error) {
	templateBody := &bytes.Buffer{}
	soapTemplates[action].Execute(templateBody, params)

	url := fmt.Sprintf("http://%s:%d/soap/server_sa", c.Host, c.Port)
	req, err := http.NewRequest("POST", url, templateBody)
	if err != nil {
		return nil, err
	}

	req.Header.Add("SOAPAction", string(action))

	return http.DefaultClient.Do(req)
}

// Login authenticates the client session to the router
func (c *Client) Login() error {
	resp, err := c.soap(loginAction, map[string]string{
		"sessionID": c.SessionID,
		"username":  c.Username,
		"password":  c.Password,
	})
	if err != nil {
		return err
	}

	type soapBody struct {
		soapResponseCode
	}

	type soapEnvelope struct {
		Body soapBody `xml:"Body"`
	}

	envelope := soapEnvelope{}
	if err := xml.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return err
	}

	respCode := envelope.Body.ResponseCode
	if respCode != 0 {
		return fmt.Errorf("Unable to login, got status code %d", respCode)
	}

	return nil
}

// Devices gets a list of devices attached to the router
func (c *Client) Devices() ([]AttachedDevice, error) {
	resp, err := c.soap(attachedDevAction, map[string]string{"sessionID": c.SessionID})
	if err != nil {
		return nil, err
	}

	type soapDevices struct {
		AttachedDevices string `xml:"NewAttachDevice"`
	}

	type soapBody struct {
		soapResponseCode
		Devices soapDevices `xml:"GetAttachDeviceResponse"`
	}

	type soapEnvelope struct {
		Body soapBody `xml:"Body"`
	}

	envelope := soapEnvelope{}
	if err := xml.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return nil, err
	}

	respCode := envelope.Body.ResponseCode
	if respCode != 0 {
		return nil, fmt.Errorf("Unable to get devices, got status code %d", respCode)
	}

	return parseDevicesString(envelope.Body.Devices.AttachedDevices)
}

func parseDevicesString(devices string) ([]AttachedDevice, error) {
	// Each device in the list is separated by a '@' character.
	// We trim the first two characters as it is just the total number of
	// devices followed by a '@'.
	devStrs := strings.Split(devices[2:], "@")
	devList := make([]AttachedDevice, len(devStrs))

	// Each device contains eight properties separaterd by a ';' character
	for i, devStr := range devStrs {
		parts := strings.Split(devStr, ";")

		if len(parts) != 8 {
			return nil, fmt.Errorf("Device string does not contain enough parts: %q", devStr)
		}

		mac, err := net.ParseMAC(parts[3])
		if err != nil {
			return nil, err
		}

		signal, err := strconv.Atoi(parts[5])
		if err != nil && parts[5] != "" {
			return nil, err
		}

		linkRate, err := strconv.Atoi(parts[6])
		if err != nil && parts[6] != "" {
			return nil, err
		}

		devList[i] = AttachedDevice{
			IP:       net.ParseIP(parts[1]),
			Name:     parts[2],
			MAC:      mac,
			Type:     parts[4],
			Signal:   signal,
			LinkRate: linkRate,
		}
	}

	return devList, nil
}
