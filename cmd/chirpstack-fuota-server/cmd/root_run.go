package cmd

import (
	"fmt"
	"net"
	"strconv"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/chirpstack/chirpstack-fuota-server/v4/internal/api"
	"github.com/chirpstack/chirpstack-fuota-server/v4/internal/client/as"
	"github.com/chirpstack/chirpstack-fuota-server/v4/internal/config"
	"github.com/chirpstack/chirpstack-fuota-server/v4/internal/eventhandler"
	"github.com/chirpstack/chirpstack-fuota-server/v4/internal/storage"
)

func run(cmd *cobra.Command, args []string) error {
	tasks := []func() error{
		setLogLevel,                  //0
		setSyslog,                    //1
		setupStorage,                 //2
		printStartMessage,            //3
		setupEventHandler,            //4
		setupApplicationServerClient, //5
		setupAPI,                     //6
	}

	for i, t := range tasks {
		if err := t(); err != nil {
			log.Error(err)
			if i == 2 {
				SendUdpMessage("mgfuota,all,2001:DB connection failed")
			} else if i == 4 {
				SendUdpMessage("mgfuota,all,2002:Event handler setup failed")
			} else if i == 5 {
				SendUdpMessage("mgfuota,all,2003:Chirpstack connection failed")
			} else if i == 6 {
				SendUdpMessage("mgfuota,all,2004:Fuota grpc setup failed")
			}
			// log.Error(err)
			return nil
		}
	}
	SendUdpMessage("mgfuota,all,appinitsuccess")

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

func setupStorage() error {
	if err := storage.Setup(&config.C); err != nil {
		return fmt.Errorf("setup storage error: %w", err)
		// return err
	}
	return nil
}

func setupEventHandler() error {
	return nil
	if err := eventhandler.Setup(&config.C); err != nil {
		return fmt.Errorf("setup event-handler error: %w", err)
		// return err
	}
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
	multicastip := api.C2config.MulticastIP
	multicastport := api.C2config.MulticastPort
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
