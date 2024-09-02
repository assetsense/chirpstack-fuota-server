package main

import (
	"fmt"
	"net"
	"time"
)

func main() {
	// multicastAddrStr := "224.1.1.1:7002"
	multicastAddrStr := "127.0.0.1:7003"

	multicastAddr, err := net.ResolveUDPAddr("udp", multicastAddrStr)
	if err != nil {
		fmt.Println("Error resolving UDP address:", err)
	}

	conn, err := net.DialUDP("udp", nil, multicastAddr)
	if err != nil {
		fmt.Println("Error connecting:", err)
		return
	}
	defer conn.Close()

	// SendUdpMessage(conn, "mgmonitor,all,appinit")
	time.Sleep(3 * time.Second)

	// SendUdpMessage(conn, "mgmonitor,all,dbready")

	SendUdpMessage(conn, "mgmonitor,all,reset")

}

func SendUdpMessage(conn *net.UDPConn, msg string) {
	message := []byte(msg)
	_, err := conn.Write(message)
	if err != nil {
		fmt.Println("Error sending message:", err)
		return
	}
	fmt.Println("Message sent:", string(message))
}

// func main() {
// 	serverAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:37020")
// 	if err != nil {
// 		fmt.Println("Error resolving address:", err)
// 		return
// 	}

// 	conn, err := net.DialUDP("udp", nil, serverAddr)
// 	if err != nil {
// 		fmt.Println("Error connecting:", err)
// 		return
// 	}
// 	defer conn.Close()

// 	message := []byte("Hello, receiver!")
// 	_, err = conn.Write(message)
// 	if err != nil {
// 		fmt.Println("Error sending message:", err)
// 		return
// 	}

// 	fmt.Println("Message sent:", string(message))
// }
