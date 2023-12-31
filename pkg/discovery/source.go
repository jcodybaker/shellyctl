package discovery

type discoverySource string

const (
	sourceMDNS   discoverySource = "mdns"
	sourceManual discoverySource = "manual"
)
