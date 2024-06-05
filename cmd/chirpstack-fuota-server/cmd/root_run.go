package cmd

import (
	"fmt"
	"net"

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
		setLogLevel,
		setSyslog,
		setupStorage,
		printStartMessage,
		setupApplicationServerClient,
		setupEventHandler,
		setupAPI,
	}

	for i, t := range tasks {
		if err := t(); err != nil {
			if i == 2 {
				SendUdpMessage("mgfuota,all,2001:DB connection failed")
			}
			// log.Error(err)
			return nil
		}
	}
	SendUdpMessage("mgfuota,all,dbready")

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
		// return fmt.Errorf("setup storage error: %w", err)
		return err
	}
	return nil
}

func setupEventHandler() error {
	if err := eventhandler.Setup(&config.C); err != nil {
		return fmt.Errorf("setup event-handler error: %w", err)
	}
	return nil
}

func setupApplicationServerClient() error {
	if err := as.Setup(&config.C); err != nil {
		return fmt.Errorf("setup application-server client error: %w", err)
	}
	return nil
}

func setupAPI() error {
	if err := api.Setup(&config.C); err != nil {
		return fmt.Errorf("setup api error: %w", err)
	}
	return nil
}

func SendUdpMessage(message string) {
	multicastAddrStr := "224.1.1.1:7002"

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
