package domain

type IPMode string

const (
	IPModeStatic IPMode = "static"
	IPModeDHCP   IPMode = "dhcp"
)

type NetworkPlan struct {
	Mode            IPMode
	Bridge          string
	VlanTag         int
	CIDR            string
	Gateway         string
	Nameservers     []string
	NodePoolRange   string
	ControlPlaneVIP string
	SSH             SSHConfig
}

type SSHConfig struct {
	PublicKeyPath  string
	PrivateKeyPath string
	Port           int
	PublicKey      string
}
