package main

import (
	"io"
	"net"
	"os"
	"os/signal"
	"strings"
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
	logFile, err := os.OpenFile("mg_fuota.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Fatalf("Failed to open log file: %v", err)
	}
	defer logFile.Close()

	// multiWriter := io.MultiWriter(os.Stdout, logFile)
	multiWriter := io.MultiWriter(os.Stdout)
	log.SetOutput(multiWriter)

	InitUdpConnection()
	// state := api.LoadState()

	// if state == "dbready" {
	// 	log.Println("Setting to previous state:", state)
	// 	InitializeDB()
	// }

	go ReceiveUdpMessages()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	log.WithField("signal", <-sigChan).Info("signal received, stopping")
}

func InitializeDB() {
	api.InitWSConnection()
	cmd.Execute(version)

}

func StartScheduler() {
	api.InitGrpcConnection()
	api.Scheduler()
	// api.CheckForFirmwareUpdate()
}

func InitUdpConnection() {
	multicastAddrStr := "224.1.1.1:7002"

	multicastAddr, err = net.ResolveUDPAddr("udp", multicastAddrStr)
	if err != nil {
		log.Error("Error resolving UDP address:", err)
	}
}

func ReceiveUdpMessages() {
	conn, err := net.ListenMulticastUDP("udp", nil, multicastAddr)
	if err != nil {
		log.Error("Error listening:", err)
		return
	}
	defer conn.Close()

	log.Info("Listening for UDP packets on ", multicastAddr)

	buffer := make([]byte, 1024)
	for {
		n, src, err := conn.ReadFromUDP(buffer)
		if err != nil {
			log.Error("Error reading from UDP:", err)
			continue
		}
		message := string(buffer[:n])
		log.Info("Received from ", src, ":", message)

		go handleUdpMessage(message)
	}
}

func handleUdpMessage(message string) {
	parts := strings.Split(message, ",")

	if len(parts) != 3 {
		log.Error("Invalid message format from udp")
		return
	}

	source := parts[0]
	destination := parts[1]
	msg := parts[2]

	if source == "mgfuota" {
		return
	}

	if destination == "mgfuota" || destination == "all" {
		if msg == "appinit" {

			InitializeDB()
			// api.SaveState("dbready")

		} else if msg == "sysready" {

			// api.SaveState("sysready")
			// state := api.LoadState()
			// if state == "dbready" {
			// 	StartScheduler()
			// } else {
			// 	log.Error("Initialise DB to Start Scheduler")
			// }
			StartScheduler()

		}
	}

}

func SendUdpMessage(message string) {

	conn, err := net.DialUDP("udp", nil, multicastAddr)
	if err != nil {
		log.Error("Error connecting:", err)
		return
	}
	defer conn.Close()

	_, err = conn.Write([]byte(message))
	if err != nil {
		log.Error("Error sending message:", err)
		return
	}

	log.Info("Message sent:", string(message))
}

func CloseUdpConnection() {
	conn.Close()
}
