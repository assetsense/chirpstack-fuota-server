package main

import (
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"time"

	pb "github.com/chirpstack/chirpstack-fuota-server/v4/internal/fuota/proto"
	"github.com/golang/protobuf/proto"
	"github.com/gorilla/websocket"
	"github.com/spf13/viper"
)

type C2Config struct {
	ServerURL    string
	Username     string
	Password     string
	Frequency    string
	LastSyncTime string
}

func main() {
	SendFailedDevicesStatusToC2("1450303692325820", "1.0.0", "2.0.0")
}

func SendFailedDevicesStatusToC2(deviceCode string, deviceVersion string, modelVersion string) {

	fuotaUpdate := &pb.FuotaUpdate{
		DeviceCode:      deviceCode,
		Timestamp:       time.Now().Unix(),
		FirmwareUpdated: modelVersion,
		Success:         false,
	}

	// Marshal the FuotaUpdate message to bytes
	fuotaUpdateBytes, err := proto.Marshal(fuotaUpdate)
	if err != nil {
		log.Fatalf("Failed to marshal FuotaUpdate message: %v", err)
	}

	typeArr := [1]int32{7202}
	fuotaUpdateBytesArr := [1][]byte{fuotaUpdateBytes}

	// Create the UniversalProto message and set its fields
	universalProto := &pb.Universal{
		Type:     typeArr[:],
		Messages: fuotaUpdateBytesArr[:],
	}

	// Marshal the UniversalProto message to bytes
	universalProtoBytes, err := proto.Marshal(universalProto)
	if err != nil {
		log.Fatalf("Failed to marshal UniversalProto message: %v", err)
	}

	var c2config C2Config = getC2ConfigFromToml()
	// Establish a WebSocket connection
	authString := fmt.Sprintf("%s:%s", c2config.Username, c2config.Password)
	encodedAuth := base64.StdEncoding.EncodeToString([]byte(authString))

	// // Device authentication
	websocketURL := c2config.ServerURL + encodedAuth + "/true/proto"
	log.Println(websocketURL)
	headers := make(http.Header)
	headers.Set("Device", "Basic "+encodedAuth)
	conn, _, err := websocket.DefaultDialer.Dial(websocketURL, nil)
	if err != nil {
		log.Fatalf("Failed to connect to WebSocket: %v", err)
	}
	// defer conn.Close()

	// Send the UniversalProto message over the WebSocket
	err = conn.WriteMessage(websocket.BinaryMessage, universalProtoBytes)
	if err != nil {
		log.Fatalf("Failed to send message over WebSocket: %v", err)
	}

	log.Println("Device firmware update failed message sent to C2 for device:", deviceCode)

}

func getC2ConfigFromToml() C2Config {

	viper.SetConfigName("c2intbootconfig")
	viper.SetConfigType("toml")
	viper.AddConfigPath("/usr/local/bin")

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			log.Fatalf("c2intbootconfig.toml file not found: %v", err)
		} else {
			log.Fatalf("Error reading c2intbootconfig.toml file: %v", err)
		}
	}

	var c2config C2Config

	c2config.Username = viper.GetString("c2App.username")
	if c2config.Username == "" {
		log.Fatal("username not found in c2intbootconfig.toml file")
	}

	c2config.Password = viper.GetString("c2App.password")
	if c2config.Password == "" {
		log.Fatal("password not found in c2intbootconfig.toml file")
	}

	c2config.ServerURL = viper.GetString("c2App.serverUrl")
	if c2config.ServerURL == "" {
		log.Fatal("serverUrl not found in c2intbootconfig.toml file")
	}

	c2config.Frequency = viper.GetString("c2App.frequency")

	return c2config
}
