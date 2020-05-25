package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"sort"
	"text/tabwriter"

	"github.com/spf13/pflag"
	"golang.org/x/xerrors"
	"software.sslmate.com/src/go-pkcs12"
)

const (
	apiClientUserAgent = "unissh/0.1"
	RCOk               = "ok"
	RCError            = "error"
)

type Meta struct {
	RC      string `json:"rc"`
	Message string `json:"msg"`
}

type ListClientResponse struct {
	Meta Meta          `json:"meta"`
	Data []*SiteClient `json:"data"`
}

type ListDeviceResponse struct {
	Meta Meta          `json:"meta"`
	Data []*SiteDevice `json:"data"`
}

type SiteClient struct {
	Name                  string `json:"name"`       // The name of client
	Hostname              string `json:"hostname"`   // (Optional) Hostname of client
	IP                    string `json:"ip"`         // IP Address
	MAC                   string `json:"mac"`        // MAC Address
	Network               string `json:"network"`    // The name of network which is the client connected
	NetworkId             string `json:"network_id"` // The id of network which is the client connected
	SiteId                string `json:"site_id"`
	AssociationTime       int    `json:"assoc_time"`
	LatestAssociationTime int    `json:"latest_assoc_time"`
	OUI                   string `json:"oui"`
	UserId                string `json:"user_id"`
	IsGuest               bool   `json:"is_guest"`
	FirstSeen             int    `json:"first_seen"`
	LastSeen              int    `json:"last_seen"`
	IsWired               bool   `json:"is_wired"`
	Noted                 bool   `json:"noted"`
	APMAC                 string `json:"ap_mac"`
	Channel               int    `json:"channel"`
	Radio                 string `json:"radio"`
	RadioName             string `json:"radio_name"`
	ESSID                 string `json:"essid"`
	BSSID                 string `json:"bssid"`
	PowersaveEnabled      bool   `json:"powersave_enabled"`
	Is11r                 bool   `json:"is_11r"`
	CCQ                   int    `json:"ccq"`
	RSSI                  int    `json:"rssi"`
	Noise                 int    `json:"noise"`
	Signal                int    `json:"signal"`
	TXRate                int    `json:"tx_rate"`
	RXRate                int    `json:"rx_rate"`
	TXPower               int    `json:"tx_power"`
	IdleTime              int    `json:"idletime"`
	DHCPEndTime           int    `json:"dhcpend_time"`
	Satisfaction          int    `json:"satisfaction"`
	VLAN                  int    `json:"vlan"`
	Uptime                int    `json:"uptime"`
	RadioProto            string `json:"radio_proto"`
	TXBytes               int64  `json:"tx_bytes"`
	TXPackets             int64  `json:"tx_packets"`
	TXRetries             int64  `json:"tx_retries"`
	RXBytes               int64  `json:"rx_bytes"`
	RXPackets             int64  `json:"rx_packets"`
	WiFiTXAttempts        int    `json:"wifi_tx_attempts"`
	Authorized            bool   `json:"authorized"`
}

type SiteDeviceType string

const (
	SiteDeviceTypeUSG SiteDeviceType = "usg"
	SiteDeviceTypeUSW SiteDeviceType = "usw"
	SiteDeviceTypeUAP SiteDeviceType = "uap"
)

type Radio struct {
	Name                  string      `json:"name"`
	Radio                 string      `json:"radio"`
	HT                    string      `json:"ht"`
	Channel               interface{} `json:"channel"`
	TXPowerMode           string      `json:"tx_power_mode"`
	AntennaGain           int         `json:"antenna_gain"`
	MinRSSIEnabled        bool        `json:"min_rssi_enabled"`
	HardNoiseFloorEnabled bool        `json:"hard_noise_floor_enabled"`
	SensLevelEnabled      bool        `json:"sens_level_enabled"`
	VWireEnabled          bool        `json:"vwire_enabled"`
	MaxTXPower            int         `json:"max_tx_power"`
	MinTXPower            int         `json:"min_tx_power"`
	NSS                   int         `json:"nss"`
	RadioCaps             int         `json:"radio_caps"`
	BuiltinAntenna        bool        `json:"builtin_antenna"`
	BuiltinAntennaGain    int         `json:"builtin_ant_gain"`
	CurrentAntennaGain    int         `json:"current_antenna_gain"`
}

type Ethernet struct {
	Name         string `json:"name"`
	MAC          string `json:"mac"`
	NumberOfPort int    `json:"num_port"`
}

type Port struct {
	Name          string `json:"name"`
	Enable        bool   `json:"enable"`
	Index         int    `json:"port_idx"`
	Media         string `json:"media"`
	PoE           bool   `json:"port_poe"`
	PoECaps       int    `json:"poe_caps"`
	SpeedCaps     int    `json:"speed_caps"`
	OpMode        string `json:"op_mode"`
	AutoNeg       bool   `json:"auto_neg"`
	FlowControlRX bool   `json:"flowctrl_rx"`
	FlowControlTX bool   `json:"flowctrl_tx"`
	FullDuplex    bool   `json:"full_duplex"`
	IsUplink      bool   `json:"is_uplink"`
	Jumbo         bool   `json:"jumbo"`
	RXBroadcast   int64  `json:"rx_broadcast"`
	RXBytes       int64  `json:"rx_bytes"`
	RXDrops       int64  `json:"rx_drops"`
	RXErrors      int64  `json:"rx_errors"`
	RXMulticast   int64  `json:"rx_multicast"`
	RXPackets     int64  `json:"rx_packets"`
	Satisfaction  int    `json:"satisfaction"`
	STPPathCost   int    `json:"stp_pathcost"`
	STPState      string `json:"stp_state"`
	TXBroadcast   int64  `json:"tx_broadcast"`
	TXBytes       int64  `json:"tx_bytes"`
	TXDrops       int64  `json:"tx_drops"`
	TXErrors      int64  `json:"tx_errors"`
	TXMulticast   int64  `json:"tx_multicast"`
	TXPackets     int64  `json:"tx_packets"`
	Up            bool   `json:"up"`
	RXByteR       int    `json:"rx_byte-r"`
	TXByteR       int    `json:"tx_byte-r"`
	Masked        bool   `json:"masked"`
	AggregatedBy  bool   `json:"aggregated_by"`
}

type SiteDevice struct {
	Type                       SiteDeviceType `json:"type"`
	Name                       string         `json:"name"` // The name of device
	IP                         string         `json:"ip"`   // IP Address
	MAC                        string         `json:"mac"`  // MAC address
	Model                      string         `json:"model"`
	Serial                     string         `json:"serial"`  // Serial code of device
	Version                    string         `json:"version"` // Firmware version
	InformUrl                  string         `json:"inform_url"`
	InformIP                   string         `json:"inform_ip"`
	Adopted                    bool           `json:"adopted"`
	SiteID                     string         `json:"site_id"`
	SSHHostKeyFingerprint      string         `json:"x_ssh_hostkey_fingerprint"`
	Fingerprint                string         `json:"x_fingerprint"`
	Radio                      []*Radio       `json:"radio_table"`
	KernelVersion              string         `json:"kernel_version"`
	Architecture               string         `json:"architecture"`
	GatewayMAC                 string         `json:"gateway_mac"`
	Uptime                     int            `json:"uptime"`
	Ethernet                   []*Ethernet    `json:"ethernet"`
	Port                       []*Port        `json:"port_table"`
	HasFan                     bool           `json:"has_fan"`
	HasTemperature             bool           `json:"has_temperature"`
	LEDOverride                string         `json:"led_override"`
	LEDOverrideColor           string         `json:"led_override_color"`
	LEDOverrideColorBrightness int            `json:"led_override_color_brightness"`
	OutdoorModeOverride        string         `json:"outdoor_mode_override"`
	LastSeen                   int            `json:"last_seen"`
	Upgradable                 bool           `json:"upgradable"`
	Uplink                     *Port          `json:"uplink"`
}

type client struct {
	Name string
	IP   string
}

func getClient(client *http.Client, host, site string) ([]*SiteClient, error) {
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("https://%s/api/s/%s/stat/sta", host, site), nil)
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}
	req.Header.Add("User-Agent", apiClientUserAgent)
	res, err := client.Do(req)
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}

	resBody := &ListClientResponse{}
	if err := json.NewDecoder(res.Body).Decode(resBody); err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}
	if err := res.Body.Close(); err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}

	if resBody.Meta.RC != RCOk {
		return nil, xerrors.New(resBody.Meta.Message)
	}

	return resBody.Data, nil
}

func getDevice(client *http.Client, host, site string) ([]*SiteDevice, error) {
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("https://%s/api/s/%s/stat/device", host, site), nil)
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}
	req.Header.Add("User-Agent", apiClientUserAgent)
	res, err := client.Do(req)
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}

	resBody := &ListDeviceResponse{}
	if err := json.NewDecoder(res.Body).Decode(resBody); err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}
	if err := res.Body.Close(); err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}

	if resBody.Meta.RC != RCOk {
		return nil, xerrors.New(resBody.Meta.Message)
	}

	return resBody.Data, nil
}

func unissh(host, credentialFile, password, site string) error {
	b, err := ioutil.ReadFile(credentialFile)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	privateKey, certificate, _, err := pkcs12.DecodeChain(b, password)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}

	tlsCert := tls.Certificate{PrivateKey: privateKey, Certificate: [][]byte{certificate.Raw}}
	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				Certificates: []tls.Certificate{tlsCert},
			},
		},
	}

	clients, err := getClient(httpClient, host, site)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	devices, err := getDevice(httpClient, host, site)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}

	siteClients := make([]*client, 0)
	for _, v := range clients {
		name := v.Name
		if name == "" {
			name = v.Hostname
		}
		siteClients = append(siteClients, &client{Name: name, IP: v.IP})
	}
	for _, v := range devices {
		siteClients = append(siteClients, &client{Name: v.Name, IP: v.IP})
	}

	sort.Slice(siteClients, func(i, j int) bool {
		return siteClients[i].Name < siteClients[j].Name
	})

	w := new(tabwriter.Writer)
	w.Init(os.Stdout, 0, 8, 1, '\t', 0)
	fmt.Fprintln(w, "Name\tIP\t")
	for _, v := range siteClients {
		fmt.Fprintf(w, "%s\t%s\n", v.Name, v.IP)
	}
	w.Flush()

	return nil
}

func main() {
	host := ""
	credentialFile := ""
	password := ""
	site := "default"
	fs := pflag.NewFlagSet("unissh-list-machines", pflag.ContinueOnError)
	fs.StringVar(&host, "host", "127.0.0.1:8443", "Unifi Controller URL")
	fs.StringVar(&credentialFile, "credential", "", "Credential file (p12)")
	fs.StringVar(&password, "password", "", "Password of p12")
	fs.StringVar(&site, "site", site, "Site name")
	if err := fs.Parse(os.Args[1:]); err != nil {
		fs.PrintDefaults()
		os.Exit(1)
	}

	if credentialFile == "" {
		credentialFile = os.Getenv("UNISSH_CREDENTIAL_FILE")
	}
	if password == "" {
		password = os.Getenv("UNISSH_PASSWORD")
	}

	if err := unissh(host, credentialFile, password, site); err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(1)
	}
}
