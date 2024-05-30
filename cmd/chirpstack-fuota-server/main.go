package main

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/chirpstack/chirpstack-fuota-server/v4/cmd/chirpstack-fuota-server/cmd"
	"github.com/chirpstack/chirpstack-fuota-server/v4/internal/api"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/grpclog"
)

// grpcLogger implements a wrapper around the logrus Logger to make it
// compatible with the grpc LoggerV2. It seems that V is not (always)
// called, therefore the Info* methods are overridden as we want to
// log these as debug info.
type grpcLogger struct {
	*log.Logger
}

func (gl *grpcLogger) V(l int) bool {
	level, ok := map[log.Level]int{
		log.DebugLevel: 0,
		log.InfoLevel:  1,
		log.WarnLevel:  2,
		log.ErrorLevel: 3,
		log.FatalLevel: 4,
	}[log.GetLevel()]
	if !ok {
		return false
	}

	return l >= level
}

func (gl *grpcLogger) Info(args ...interface{}) {
	if log.GetLevel() == log.DebugLevel {
		log.Debug(args...)
	}
}

func (gl *grpcLogger) Infoln(args ...interface{}) {
	if log.GetLevel() == log.DebugLevel {
		log.Debug(args...)
	}
}

func (gl *grpcLogger) Infof(format string, args ...interface{}) {
	if log.GetLevel() == log.DebugLevel {
		log.Debugf(format, args...)
	}
}

func init() {
	grpclog.SetLoggerV2(&grpcLogger{log.StandardLogger()})
}

type UDPResponse struct {
	Src  string `json:"src"`
	Msg  string `json:"msg"`
	Dest string `json:"dest"`
}

var conn *net.UDPConn
var multicastAddr *net.UDPAddr
var err error

var version string // set by the compiler

// func main() {

// 	cmd.Execute(version)

// 	api.InitWSConnection()
// 	api.InitGrpcConnection()

// 	// api.Scheduler()
// 	api.CheckForFirmwareUpdate()

// 	sigChan := make(chan os.Signal)
// 	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
// 	log.WithField("signal", <-sigChan).Info("signal received, stopping")
// }

func main() {

	InitUdpConnection()

	go ReceiveUdpMessages()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	log.WithField("signal", <-sigChan).Info("signal received, stopping")
}

func InitUdpConnection() {
	multicastAddrStr := "224.1.1.1:7002"

	multicastAddr, err = net.ResolveUDPAddr("udp", multicastAddrStr)
	if err != nil {
		fmt.Println("Error resolving UDP address:", err)
	}
}

func ReceiveUdpMessages() {
	conn, err := net.ListenMulticastUDP("udp", nil, multicastAddr)
	if err != nil {
		fmt.Println("Error listening:", err)
		return
	}
	defer conn.Close()

	fmt.Println("Listening for UDP packets on", multicastAddr)

	buffer := make([]byte, 1024)
	for {
		n, src, err := conn.ReadFromUDP(buffer)
		if err != nil {
			fmt.Println("Error reading from UDP:", err)
			continue
		}
		message := string(buffer[:n])
		fmt.Printf("Received from %v: %s\n", src, message)

		// var response UDPResponse
		// if err := json.Unmarshal(buffer[:n], &response); err != nil {
		// 	fmt.Println("Error unmarshalling response:", err)
		// 	return
		// }

		go handleUdpMessage(message)
	}
}

func handleUdpMessage(message string) {
	if message == "db_init" {
		cmd.Execute(version)
		SendUdpMessage("db_ready")
	} else if message == "init" {
		api.InitWSConnection()
		api.InitGrpcConnection()
		// api.Scheduler()
		// api.CheckForFirmwareUpdate()
		SendUdpMessage("init_ready")
	} else {
		fmt.Println("Received message is not valid")
	}
}

func SendUdpMessage(message string) {
	// ack := UDPResponse{
	// 	Src:  "mg_fuota",
	// 	Msg:  "init_ready",
	// 	Dest: "all",
	// }

	// ackStr, err := json.Marshal(ack)
	// if err != nil {
	// 	fmt.Println("Error marshalling response:", err)
	// 	return
	// }
	// conn.WriteToUDP([]byte(ackStr), multicastAddr)
	// fmt.Println("sent:", ackStr)

	conn, err := net.DialUDP("udp", nil, multicastAddr)
	if err != nil {
		fmt.Println("Error connecting:", err)
		return
	}
	defer conn.Close()

	_, err = conn.Write([]byte(message))
	if err != nil {
		fmt.Println("Error sending message:", err)
		return
	}

	fmt.Println("Message sent:", string(message))
}

func CloseUdpConnection() {
	conn.Close()
}
