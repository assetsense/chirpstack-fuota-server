package main

import (
	"encoding/json"
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

type UDPResponse struct {
	Src  string `json:"src"`
	Msg  string `json:"msg"`
	Dest string `json:"dest"`
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

var version string // set by the compiler

func main() {
	port := "127.0.0.1:37020"

	addr, err := net.ResolveUDPAddr("udp", port)
	if err != nil {
		fmt.Println("Error resolving UDP address:", err)
		os.Exit(1)
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		fmt.Println("Error listening on UDP:", err)
		os.Exit(1)
	}
	defer conn.Close()

	fmt.Println("Listening for UDP packets on", conn.LocalAddr().String())

	buffer := make([]byte, 1024)

	for {
		n, src, err := conn.ReadFromUDP(buffer)
		if err != nil {
			fmt.Println("Error reading from UDP:", err)
			continue
		}

		fmt.Printf("Received from %v: %s\n", src, string(buffer[:n]))

		// messageBytes := []byte("Hi, sender!")
		// _, err = conn.WriteToUDP(messageBytes, addr)
		// if err != nil {
		// 	fmt.Println("Error sending message:", err)
		// 	return
		// }
		var response UDPResponse
		if err := json.Unmarshal(buffer[:n], &response); err != nil {
			fmt.Println("Error unmarshalling response:", err)
			return
		}

		if response.Msg == "db_init" {

			if err := cmd.SetupStorage(); err != nil {
				log.Fatal(err)
			}

			ack := UDPResponse{
				Src:  addr.String(),
				Msg:  string(buffer[:n]),
				Dest: "127.0.0.1:8080",
			}

			ackStr, err := json.Marshal(ack)
			if err != nil {
				fmt.Println("Error marshalling response:", err)
				return
			}
			conn.WriteToUDP([]byte(ackStr), addr)

		} else if response.Msg == "init" {

			go cmd.Execute(version)

			api.InitWSConnection()
			api.InitGrpcConnection()

			// api.Scheduler()
			api.CheckForFirmwareUpdate()

			sigChan := make(chan os.Signal)
			signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
			log.WithField("signal", <-sigChan).Info("signal received, stopping")
		}
	}

}
