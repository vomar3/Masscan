package banner

import (
	"fmt"
	"net"
	"time"
)

func GrabBanner(ip string, port int) string {
	address := net.JoinHostPort(ip, fmt.Sprintf("%d", port))

	conn, err := net.DialTimeout("tcp", address, 3*time.Second)
	if err != nil {
		return ""
	}
	defer conn.Close()

	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))

	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		return ""
	}

	return string(buf[:n])
}
