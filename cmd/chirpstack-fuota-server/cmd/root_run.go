package cmd

import (
	"errors"
	"fmt"
	"net"
	"strconv"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/chirpstack/chirpstack-fuota-server/v4/internal/api"
	"github.com/chirpstack/chirpstack-fuota-server/v4/internal/client/as"
	"github.com/chirpstack/chirpstack-fuota-server/v4/internal/config"
	"github.com/chirpstack/chirpstack-fuota-server/v4/internal/storage"
)

func run(cmd *cobra.Command, args []string) error {
	tasks := []func() error{
		setLogLevel, //0
		setSyslog,   //1
		// setupStorage,
		printStartMessage, //2
		// setupEventHandler,
		setupApplicationServerClient, //3
		setupAPI,                     //4
	}

	for i, t := range tasks {
		if err := t(); err != nil {
			log.Error(err)
			if i == 3 {
				// SendUdpMessage("mgfuota,mgmonitor,appinitfail")
				// SendUdpMessage("mgfuota,mgmonitor,2002:Chirpstack connection failed")
				return errors.New("chirpstack connection failed")
			} else if i == 4 {
				// SendUdpMessage("mgfuota,mgmonitor,appinitfail")
				// SendUdpMessage("mgfuota,mgmonitor,2003:Fuota grpc setup failed")
				return errors.New("fuota grpc setup failed")
			}
			// log.Error(err)
		}
	}
	return nil
}

func setLogLevel() error {
	log.SetLevel(log.Level(uint8(config.C.General.LogLevel)))
	return nil
}

func printStartMessage() error {
	log.WithFields(log.Fields{
		"version": version,
	}).Info("starting ChirpStack FUOTA Server")
	return nil
}

func SetupStorage() error {
	if err := storage.Setup(&config.C); err != nil {
		return fmt.Errorf("setup storage error: %w", err)
		// return err
	}
	return nil
}

func setupEventHandler() error {

	// if err := eventhandler.Setup(&config.C); err != nil {
	// 	return fmt.Errorf("setup event-handler error: %w", err)
	// 	// return err
	// }
	return nil
}

func setupApplicationServerClient() error {
	if err := as.Setup(&config.C); err != nil {
		return fmt.Errorf("setup application-server client error: %w", err)
		// return err
	}
	return nil
}

func setupAPI() error {
	if err := api.Setup(&config.C); err != nil {
		return fmt.Errorf("setup api error: %w", err)
		// return err
	}
	return nil
}

func SendUdpMessage(message string) {
	var C2config api.C2Config = api.GetC2ConfigFromToml()
	multicastip := C2config.MulticastIP
	multicastport := C2config.MulticastPort
	multicastAddrStr := multicastip + ":" + strconv.Itoa(multicastport)

	multicastAddr, err := net.ResolveUDPAddr("udp", multicastAddrStr)
	if err != nil {
		log.Error("Error resolving UDP address:", err)
	}
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
