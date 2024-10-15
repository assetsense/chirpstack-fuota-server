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

	// // ReceiveUdpMessages()
	// currentTime := time.Now()
	// var interval float64 = 0.0166667
	// targetTime := currentTime.Add(time.Duration(interval*3600) * time.Second)

	// // targetTimeEpoch := targetTime.Unix()

	// duration := targetTime.Sub(currentTime)
	// fmt.Println("duration:" + duration.String())

	// timer := time.NewTimer(duration)

	// fmt.Println("Firmware update request scheduled at ", targetTime)
	// <-timer.C
	SendUdpMessage(conn, "mgmonitor,all,c2connect")
	time.Sleep(3 * time.Second)

	SendUdpMessage(conn, "mgmonitor,all,appinit")
	time.Sleep(3 * time.Second)

	SendUdpMessage(conn, "mgmonitor,all,dbready")
	time.Sleep(3 * time.Second)

	SendUdpMessage(conn, "mgmonitor,all,sysready")
	time.Sleep(3 * time.Second)

	// SendUdpMessage(conn, "mgmonitor,all,reset")
	// time.Sleep(3 * time.Second)

	// SendUdpMessage(conn, "mgmonitor,all,configchange")

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

func ReceiveUdpMessages() {
	multicastAddrStr := "224.1.1.1:7003"

	multicastAddr, err := net.ResolveUDPAddr("udp", multicastAddrStr)
	if err != nil {
		fmt.Println("Error resolving UDP address:", err)
	}
	conn, err := net.ListenMulticastUDP("udp", nil, multicastAddr)
	if err != nil {
		fmt.Println("Error listening:", err)
		return
	}
	defer conn.Close()

	fmt.Println("Listening for UDP packets on ", multicastAddr)

	buffer := make([]byte, 1024)
	for {
		n, src, err := conn.ReadFromUDP(buffer)
		if err != nil {
			fmt.Println("Error reading from UDP:", err)
			continue
		}
		message := string(buffer[:n])
		fmt.Println("Received from ", src, ":", message)

		// go handleUdpMessage(message)
	}
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
