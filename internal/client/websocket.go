package client

import (
	"encoding/json"
	"time"
	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
	"github.com/chirpstack/chirpstack-fuota-server/v4/internal/config"
	"database/sql"
)

var conn *websocket.Conn

type FirmwareUpdateResponse struct {
	IotModelId      int64  `json:"iotModelId"`
	FirmwareVersion string `json:"firmwareVersion"`
}

func SetupWebSocket(conf *config.Config) error {
	var err error
	conn, _, err = websocket.DefaultDialer.Dial(conf.WebSocketURL, nil)
	if err != nil {
		return err
	}
	go listenForMessages()
	return nil
}

func listenForMessages() {
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			log.Error("read message error: ", err)
			continue
		}
		handleMessage(message)
	}
}

func handleMessage(message []byte) {
	var response FirmwareUpdateResponse
	if err := json.Unmarshal(message, &response); err != nil {
		log.Error("JSON unmarshal error: ", err)
		return
	}

	log.Infof("Received update request for model %d with firmware %s", response.IotModelId, response.FirmwareVersion)

	db, err := sql.Open("postgres", config.C.PostgreSQL.DSN)
	if err != nil {
		log.Error("Failed to connect to database: ", err)
		return
	}
	defer db.Close()

	devices, err := getDevicesByModelId(db, response.IotModelId)
	if err != nil {
		log.Error("Failed to retrieve devices: ", err)
		return
	}

	//TODO: using these devices we should update the firmware
}

func SchedulePeriodicChecks(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		if err := conn.WriteMessage(websocket.TextMessage, []byte(`{"action":"checkUpdates"}`)); err != nil {
			log.Error("Failed to send check updates request: ", err)
		}
	}
}
