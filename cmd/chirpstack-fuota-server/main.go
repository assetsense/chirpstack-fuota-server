package main

import (
	"io"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/chirpstack/chirpstack-fuota-server/v4/cmd/chirpstack-fuota-server/cmd"
	"github.com/chirpstack/chirpstack-fuota-server/v4/internal/api"
	"github.com/chirpstack/chirpstack-fuota-server/v4/internal/client/as"
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

var state int = 0

// func main() {
// 	ConnectToC2()
// 	var payload []byte = api.GetFirmwarePayload(7, "1.0.10")
// 	// fmt.Println(payload[:10])
// 	cnt := 0
// 	for _, i := range payload {
// 		fmt.Printf("%02x ", i)
// 		cnt++
// 		if cnt > 50 {
// 			break
// 		}
// 	}
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

	// ConnectToC2()
	// StartScheduler()
	// return

	InitUdpConnection()
	// state := api.LoadState()

	// if state == "dbready" {
	// 	log.Println("Setting to previous state:", state)
	// 	InitializeDB()
	// }
	// ConnectToC2()
	// InitializeDB()
	// StartScheduler()
	go ReceiveUdpMessages()
	time.Sleep(10 * time.Second)
	SendUdpMessage("mgfuota,mgmonitor,started")

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	log.WithField("signal", <-sigChan).Info("signal received, stopping")
}

func ConnectToC2() error {
	return api.InitWSConnection()

}

func InitializeApp() error {
	err := cmd.Execute(version)
	if err != nil {
		return err
	}
	return nil
}

func InitializeDB() error {
	return api.InitializeDB()
}

func StartScheduler() {
	api.InitGrpcConnection()

	api.Scheduler()
	// api.CheckForFirmwareUpdate()
}

func InitUdpConnection() {
	var C2config api.C2Config = api.GetC2ConfigFromToml()
	if C2config.MulticastIP == "" {
		for {
			time.Sleep(1 * time.Hour)
		}
	}
	multicastip := C2config.MulticastIP
	multicastport := C2config.MulticastPort
	multicastAddrStr := multicastip + ":" + strconv.Itoa(multicastport)

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

	// 0 -> STARTED
	// 1 -> C2CONNECTED
	// 2 -> DBREADY
	// 3 -> DBSYSNCED
	// 4 -> RUNNING

	if destination == "mgfuota" || destination == "all" {

		if msg == "c2connect" {
			if state == 0 {
				state = 1
				err = ConnectToC2()
				if err != nil {
					SendUdpMessage("mgfuota,mgmonitor,c2connectfail")
					state = 0
				} else {
					SendUdpMessage("mgfuota,mgmonitor,c2connectsuccess")
					state = 1
				}
			}

		} else if msg == "appinit" {
			// if true {
			if state == 1 {
				state = 2
				err = InitializeApp()
				if err != nil {
					state = 1
				} else {
					state = 2
				}
			}
			// api.SaveState("dbready")

		} else if msg == "dbready" {
			// if true {
			if state == 2 {
				state = 3
				err = InitializeDB()
				if err != nil {
					SendUdpMessage("mgfuota,mgmonitor,dbreadyfail")
					// SendUdpMessage("mgfuota,mgmonitor,2004: DB connection failed")
					state = 2
				} else {
					SendUdpMessage("mgfuota,mgmonitor,dbreadysuccess")
					state = 3
				}
			}

		} else if msg == "sysready" {
			if state == 3 {
				state = 4
				StartScheduler()
			}
			// api.SaveState("sysready")
			// state := api.LoadState()
			// if state == "dbready" {
			// 	StartScheduler()
			// } else {
			// 	log.Error("Initialise DB to Start Scheduler")
			// }

		} else if msg == "reset" {
			//reste logic

			err := ResetFuota()
			if err != nil {
				log.Info("Failed to reset database")
			} else {
				state = 0
				log.Info("Fuota reset is successfull")
				SendUdpMessage("mgfuota,mgmonitor,started")
			}
		} else if msg == "configchange" {
			//reste logic

			err := RefreshFuota()
			if err != nil {
				log.Info("Failed to config change")
			} else {
				log.Info("config change is successfull")
				SendUdpMessage("mgfuota,mgmonitor,configchangesuccess")
			}
		} else if msg == "hello" {
			SendUdpMessage("mgfuota,mgmonitor,hello")
		}
	}

}

func ResetFuota() error {
	api.CloseWSConnection()
	api.CloseGrpcConnection()
	err := api.ResetStorage()
	api.CloseApiServer()
	as.CloseClientConn()
	api.StopScheduler()
	if err != nil {
		return err
	}
	return nil
}

func RefreshFuota() error {
	api.CloseWSConnection()
	api.CloseGrpcConnection()
	err := api.CloseDBConn()
	api.CloseApiServer()
	as.CloseClientConn()
	api.StopScheduler()
	if err != nil {
		return err
	}

	if state == 1 {
		ConnectToC2()
	} else if state == 2 {
		ConnectToC2()
		InitializeApp()
	} else if state == 3 {
		ConnectToC2()
		InitializeApp()
		InitializeDB()
	} else if state == 4 {
		ConnectToC2()
		InitializeApp()
		InitializeDB()
		StartScheduler()
	}
	return nil
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
