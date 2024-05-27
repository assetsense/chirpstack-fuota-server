package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/spf13/viper"

	"github.com/gofrs/uuid"
	"github.com/golang/protobuf/ptypes"
	"google.golang.org/grpc"

	"github.com/brocaar/lorawan"
	"github.com/brocaar/lorawan/applayer/multicastsetup"
	fuota "github.com/chirpstack/chirpstack-fuota-server/v4/api/go"
	"github.com/chirpstack/chirpstack-fuota-server/v4/internal/storage"
	"github.com/jmoiron/sqlx"
)

// type Config struct {
// 	Username     string `toml:"username"`
// 	Password     string `toml:"password"`
// 	C2ServerWS   string `toml:"c2serverWS"`
// 	C2ServerREST string `toml:"c2serverREST"`
// 	Frequency    int    `toml:"frequency"`
// }

// var region = map[int]fuota.Region{
// 	6134: fuota.Region_AU915,
// 	6135: fuota.Region_CN779,
// 	6136: fuota.Region_EU868,
// 	6137: fuota.Region_IN865,
// 	6138: fuota.Region_EU433,
// 	6139: fuota.Region_ISM2400,
// 	6140: fuota.Region_KR920,
// 	6141: fuota.Region_AS923,
// 	6142: fuota.Region_US915,
// }

var regions = map[string]fuota.Region{
	"AU915":   fuota.Region_AU915,
	"CN779":   fuota.Region_CN779,
	"EU868":   fuota.Region_EU868,
	"IN865":   fuota.Region_IN865,
	"EU433":   fuota.Region_EU433,
	"ISM2400": fuota.Region_ISM2400,
	"KR920":   fuota.Region_KR920,
	"AS923":   fuota.Region_AS923,
	"US915":   fuota.Region_US915,
}

type C2Config struct {
	ServerURL    string
	Username     string
	Password     string
	Frequency    int
	LastSyncTime string
}

type FirmwareUpdateResponse struct {
	MsgType string `json:"msg_type"`
	Models  []struct {
		ModelId int    `json:"modelId"`
		Version string `json:"version"`
	} `json:"models"`
}

type FirmwareResponse struct {
	MsgType  string `json:"msg_type"`
	ModelId  int    `json:"modelId"`
	Version  string `json:"version"`
	Firmware []byte `json:"firmware"`
}

// var C2Config = OpenC2ConfigToml()
var WSConn *websocket.Conn
var GrpcConn *grpc.ClientConn
var err error

var c2config C2Config = getC2ConfigFromToml()

func InitGrpcConnection() {
	dialOpts := []grpc.DialOption{
		grpc.WithBlock(),
		grpc.WithInsecure(),
	}

	GrpcConn, err = grpc.Dial("localhost:8070", dialOpts...)
	if err != nil {
		panic(err)
	}
	log.Println("Grpc Connection Established")
}

func InitWSConnection() {
	//creating authentication string
	authString := fmt.Sprintf("%s:%s", c2config.Username, c2config.Password)
	encodedAuth := base64.StdEncoding.EncodeToString([]byte(authString))

	// Device authentication
	websocketURL := c2config.ServerURL + encodedAuth + "/true"
	headers := make(http.Header)
	headers.Set("Device", "Basic "+encodedAuth)

	// User authentication
	// websocketURL := getC2serverUrl()
	// headers.Set("Authorization", "Basic "+encodedAuth)

	for {
		WSConn, _, err = websocket.DefaultDialer.Dial(websocketURL, headers)
		if err != nil {
			log.Println("Error C2", err)
			time.Sleep(2 * time.Second)
			continue // Retry connection in
		}

		log.Println("Websocket Connection Established")
		break
	}

}

func CloseWSConnection() {
	err = WSConn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	if err != nil {
		log.Println("Write close error:", err)
	}

	WSConn.Close()
	log.Println("Websocket Connection Closed")
}

func SendWSMessage(message string) {
	err := WSConn.WriteMessage(websocket.TextMessage, []byte(message))
	if err != nil {
		log.Fatal("Write error:", err)
	}
	log.Println("Websocket Message Sent: " + message)
}

func ReceiveWSMessage() string {
	_, message, err := WSConn.ReadMessage()
	if err != nil {
		log.Fatal("Read error:", err)
	}
	messageStr := string(message)
	log.Println("Websocket Message received: " + messageStr)
	return messageStr
}

func ReceiveMessageDummyForModels() string {
	// {\"msg_type\":\"FIRMWARE_UPDATES_RES\", \"models\":[{\"modelId\":1234, \"version\":\"1.0.0\"}]}]};

	dummyResponseJson := `{"msg_type":"FIRMWARE_UPDATES_RES", "models":[{"modelId":1234, "version":"1.0.1"}]}`
	log.Println("Dummy Message received: \n" + dummyResponseJson)
	return dummyResponseJson
}

func ReceiveMessageDummyForFirmware() string {
	// {"msg_type":"FIRMWARE_FILE_RES", "modelId": 1234, "version": "1.0.1", "firmware":base64}

	dummyResponseJson := `{"msg_type": "FIRMWARE_FILE_RES","modelId": 1234,"version": "1.0.1","firmware": "SGVsbG8sIFdvcmxk"}`

	log.Println("Dummy Message received: " + dummyResponseJson)
	return dummyResponseJson
}

func Scheduler() {
	ticker := time.NewTicker(time.Duration(c2config.Frequency) * time.Hour)
	// ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			CheckForFirmwareUpdate()
		}
	}
}

func CheckForFirmwareUpdate() {
	log.Println("Checking for firmware updates")
	//Request format - {"msg_typ":"FIRMWARE_UPDATES", "ls":0}
	SendWSMessage("{\"msg_type\":\"FIRMWARE_UPDATE\", \"ls\":0}")

	//Respose format - {\"msg_type\":\"FIRMWARE_UPDATES_RES\", \"models\":[{"modelId":1234, \"version\":\"1.0.1\"}]}]};
	// response := ReceiveMessageDummyForModels()
	response := ReceiveWSMessage()

	handleMessage(response)
}

func handleMessage(message string) {
	var response FirmwareUpdateResponse
	err := json.Unmarshal([]byte(message), &response)
	if err != nil {
		log.Fatalf("Error unmarshalling FirmwareUpdateResponse: %v", err)
	}

	for _, model := range response.Models {
		// fmt.Printf("Model ID: %d, Version: %s\n", model.ModelId, model.Version)

		if err := storage.Transaction(func(tx sqlx.Ext) error {
			var devices []storage.Device
			devices, err := storage.GetDevicesByModelAndVersion(context.Background(), tx, model.ModelId, model.Version)
			if err != nil {
				return fmt.Errorf("GetDevicesByModelAndVersion error: %w", err)
			}
			if len(devices) == 0 {
				log.Println("No devices in DB of Model Id:", model.ModelId, "and Version <", model.Version)
				return nil
			} else {
				log.Println(len(devices), "devices in DB of Model Id:", model.ModelId, "and Version <", model.Version)
			}

			deviceMap := make(map[string][]storage.Device)

			// Separate devices based on region and store in the map
			for _, device := range devices {
				if err := storage.Transaction(func(tx sqlx.Ext) error {
					var region string
					region, err := storage.GetRegionByDeviceId(context.Background(), tx, device.DeviceId)
					if err != nil {
						return fmt.Errorf("GetRegionByDeviceId error: %w", err)
					}
					deviceMap[region] = append(deviceMap[region], device)
					return nil
				}); err != nil {
					log.Fatal(err)
				}
			}

			// Loop over the map and print the region and devices
			for region, devices := range deviceMap {
				go InitialiseDeploymentRequest(model.ModelId, model.Version, devices, region)
			}
			return nil
		}); err != nil {
			log.Fatal(err)
		}
	}
}

func InitialiseDeploymentRequest(modelId int, version string, devices []storage.Device, region string) {

	var applicationId string = getApplicationId()
	var payload []byte = getFirmwarePayload(modelId, version)

	createDeploymentRequest(version, devices, applicationId, region, payload)
}

func createDeploymentRequest(firmwareVersion string, devices []storage.Device, applicationId string, region string, payload []byte) {
	mcRootKey, err := multicastsetup.GetMcRootKeyForGenAppKey(lorawan.AES128Key{0x09, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Creating deployement request for Region:", region, " FirmwareVersion:", firmwareVersion)

	client := fuota.NewFuotaServerServiceClient(GrpcConn)

	resp, err := client.CreateDeployment(context.Background(), &fuota.CreateDeploymentRequest{
		Deployment: &fuota.Deployment{
			ApplicationId:                     applicationId,
			Devices:                           GetDeploymentDevices(mcRootKey, devices),
			MulticastGroupType:                fuota.MulticastGroupType_CLASS_C,
			MulticastDr:                       5,
			MulticastFrequency:                868100000,
			MulticastGroupId:                  0,
			MulticastTimeout:                  6,
			MulticastRegion:                   regions[region],
			UnicastTimeout:                    ptypes.DurationProto(60 * time.Second),
			UnicastAttemptCount:               1,
			FragmentationFragmentSize:         50,
			Payload:                           payload,
			FragmentationRedundancy:           1,
			FragmentationSessionIndex:         0,
			FragmentationMatrix:               0,
			FragmentationBlockAckDelay:        1,
			FragmentationDescriptor:           []byte{0, 0, 0, 0},
			RequestFragmentationSessionStatus: fuota.RequestFragmentationSessionStatus_AFTER_SESSION_TIMEOUT,
		},
	})
	if err != nil {
		panic(err)
	}

	var id uuid.UUID
	copy(id[:], resp.GetId())

	// // log.Printf("deployment request sent: %s\n", id)

	// ticker := time.NewTicker(1 * time.Minute)
	// defer ticker.Stop()

	// for {
	// 	select {
	// 	case <-ticker.C:
	// 		GetStatus(id)
	// 	}
	// }
}

func GetDeploymentDevices(mcRootKey lorawan.AES128Key, devices []storage.Device) []*fuota.DeploymentDevice {

	var deploymentDevices []*fuota.DeploymentDevice
	for _, device := range devices {
		fmt.Println("	device eui: " + device.DeviceCode)
		deploymentDevices = append(deploymentDevices, &fuota.DeploymentDevice{
			DevEui:    device.DeviceCode,
			McRootKey: mcRootKey.String(),
		})
	}

	return deploymentDevices
}

func getFirmwarePayload(modelId int, version string) []byte {
	log.Println("Getting firmware file for Model Id:", modelId, "and Version:", version)
	//Request format - {“msg_type”:”FIRMWARE_FILE”, “filter”:{\”models\”:[\”modelId\”: 1234, \”version\”:”1.0.1”, \”latestVersion\”:false]}}
	request := fmt.Sprintf(`{"msg_type":"FIRMWARE_FILE", "filter":"{\"models\":[{\"modelId\": %d, \"version\": \"%s\", \"latestVersion\": false}]}"}`, modelId, version)
	SendWSMessage(request)

	//Respose format - {"msg_type":"FIRMWARE_FILE_RES", "modelId": 1234, "version": "1.0.1", "firmware":base64}
	// responseMessage := ReceiveMessageDummyForFirmware()
	responseMessage := ReceiveWSMessage()
	var response FirmwareResponse
	if err := json.Unmarshal([]byte(responseMessage), &response); err != nil {
		log.Fatalf("failed to unmarshal response: %v", err)
	}
	firmwareBytes, err := base64.StdEncoding.DecodeString(string(response.Firmware))
	if err != nil {
		fmt.Println("Error decoding Base64 firmware string:", err)
	}
	return firmwareBytes
}

func getC2ConfigFromToml() C2Config {

	viper.SetConfigName("c2int_boot_config")
	viper.SetConfigType("toml")
	viper.AddConfigPath("/usr/local/bin")

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			log.Fatalf("c2int_boot_config.toml file not found: %v", err)
		} else {
			log.Fatalf("Error reading c2int_boot_config.toml file: %v", err)
		}
	}

	var c2config C2Config

	c2config.Username = viper.GetString("c2App.username")
	if c2config.Username == "" {
		log.Fatal("username not found in c2int_boot_config.toml file")
	}

	c2config.Password = viper.GetString("c2App.password")
	if c2config.Password == "" {
		log.Fatal("password not found in c2int_boot_config.toml file")
	}

	c2config.ServerURL = viper.GetString("c2App.serverUrl")
	if c2config.ServerURL == "" {
		log.Fatal("serverUrl not found in c2int_boot_config.toml file")
	}

	c2config.Frequency = viper.GetInt("c2App.frequency")

	return c2config
}

func getApplicationId() string {

	viper.SetConfigName("c2int_runtime_config")
	viper.SetConfigType("toml")
	viper.AddConfigPath("/usr/local/bin")

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			log.Fatalf("c2int_runtime_config.toml file not found: %v", err)
		} else {
			log.Fatalf("Error reading c2int_runtime_config.toml file: %v", err)
		}
	}

	applicationId := viper.GetString("chirpstack.application.id")
	if applicationId == "" {
		log.Fatal("Application id not found in c2int_runtime_config.toml file")
	}

	return applicationId
}

func GetStatus(id uuid.UUID) {

	client := fuota.NewFuotaServerServiceClient(GrpcConn)

	resp, err := client.GetDeploymentStatus(context.Background(), &fuota.GetDeploymentStatusRequest{
		Id: id.String(),
	})

	if err != nil {
		panic(err)
	}

	log.Printf("deployment status: %s\n", resp.EnqueueCompletedAt)
}
