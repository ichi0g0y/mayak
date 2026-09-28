package app

import (
	"net"
	"time"
)

// requestLocalNetwork asks macOS for local network access while nothing is
// waiting on it. Since macOS 15 an app's first packet to the LAN brings up
// "Allow MAYAK to find devices on local networks?", and until it is answered
// the LAN stays closed: a pairing started before the answer found no route
// to the Host on the same network and failed, while the next one connected.
// One mDNS query (224.0.0.251:5353, for a service nobody offers) is enough to
// bring the question up at start, before a pairing code is typed.
func requestLocalNetwork() {
	conn, err := net.DialTimeout("udp4", "224.0.0.251:5353", 2*time.Second)
	if err != nil {
		return
	}
	defer conn.Close()
	// A DNS query: id 0, no flags, one question (_mayak._udp.local PTR, IN).
	query := []byte{0, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0}
	for _, label := range []string{"_mayak", "_udp", "local"} {
		query = append(query, byte(len(label)))
		query = append(query, label...)
	}
	query = append(query, 0, 0, 12, 0, 1)
	_, _ = conn.Write(query)
}
