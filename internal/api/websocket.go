package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/gorilla/websocket"
	"github.com/spf13/viper"

	"github.com/gofrs/uuid"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"google.golang.org/grpc"

	"github.com/brocaar/lorawan"
	"github.com/brocaar/lorawan/applayer/multicastsetup"
	fuota "github.com/chirpstack/chirpstack-fuota-server/v4/api/go"
	"github.com/chirpstack/chirpstack-fuota-server/v4/internal/client/as"
	"github.com/chirpstack/chirpstack-fuota-server/v4/internal/config"
	"github.com/chirpstack/chirpstack-fuota-server/v4/internal/firmUpdateInfo"
	pb "github.com/chirpstack/chirpstack-fuota-server/v4/internal/fuota/proto"
	"github.com/chirpstack/chirpstack-fuota-server/v4/internal/storage"
	"github.com/chirpstack/chirpstack/api/go/v4/api"
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
	ServerURL     string
	Username      string
	Password      string
	Frequency     string
	LastSyncTime  string
	FuotaInterval int64
	SessionTime   int
	MulticastIP   string
	MulticastPort int
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

var stopScheduler bool = false

func InitGrpcConnection() {
	dialOpts := []grpc.DialOption{
		grpc.WithBlock(),
		grpc.WithInsecure(),
		grpc.WithDisableRetry(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	GrpcConn, err = grpc.DialContext(ctx, "localhost:8070", dialOpts...)
	if err != nil {
		panic(err)
	}
	log.Info("Grpc Connection Established")
}

func CloseGrpcConnection() {
	if GrpcConn != nil {
		GrpcConn.Close()
		log.Info("Grpc Connection Closed")
	} else {
		log.Info("Grpc is not connected")
	}
}

func InitWSConnection() error {
	var C2config C2Config = GetC2ConfigFromToml()
	//creating authentication string
	authString := fmt.Sprintf("%s:%s", C2config.Username, C2config.Password)
	encodedAuth := base64.StdEncoding.EncodeToString([]byte(authString))

	// Device authentication
	websocketURL := C2config.ServerURL + encodedAuth + "/true"
	headers := make(http.Header)
	headers.Set("Device", "Basic "+encodedAuth)

	// User authentication
	// websocketURL := getC2serverUrl()
	// headers.Set("Authorization", "Basic "+encodedAuth)

	// for {
	WSConn, _, err = websocket.DefaultDialer.Dial(websocketURL, headers)
	if err != nil {
		log.Error("Error C2", err)
		// time.Sleep(2 * time.Second)
		// continue // Retry connection in
		return err
	}
	// SendUdpMessage("mgfuota,all,c2connectsuccess")
	log.Info("Websocket Connection Established")
	// break
	// }
	return nil

}

func CloseWSConnection() {
	if WSConn == nil {
		log.Info("C2WS is not connected")
		return
	}
	err = WSConn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	if err != nil {
		log.Error("Write close error:", err)
	}

	WSConn.Close()
	log.Info("C2 Websocket Connection Closed")
}

func SendWSMessage(message string) {
	err := WSConn.WriteMessage(websocket.TextMessage, []byte(message))
	if err != nil {
		log.Fatal("Write error:", err)
	}
	log.Info("Websocket Message Sent: " + message)
}

func ReceiveWSMessage() string {
	_, message, err := WSConn.ReadMessage()
	if err != nil {
		log.Fatal("Read error:", err)
	}
	messageStr := string(message)
	log.Info("Websocket Message received: " + messageStr)
	return messageStr
}

func ReceiveWSMessageBinary() []byte {
	_, message, err := WSConn.ReadMessage()
	if err != nil {
		log.Fatal("Read error:", err)
	}
	log.Info("Websocket Message received: " + string(message)[:20])
	return message
}

func ReceiveMessageDummyForModels() string {
	// {\"msg_type\":\"FIRMWARE_UPDATES_RES\", \"models\":[{\"modelId\":1234, \"version\":\"1.0.0\"}]}]};

	dummyResponseJson := `{"msg_type":"FIRMWARE_UPDATES_RES", "models":[{"modelId":1234, "version":"1.0.1"}]}`
	log.Info("Dummy Message received: \n" + dummyResponseJson)
	return dummyResponseJson
}

func ReceiveMessageDummyForFirmware() []byte {
	// {"msg_type":"FIRMWARE_FILE_RES", "modelId": 1234, "version": "1.0.1", "firmware":base64}

	dummyResponseJson := `{"msg_type": "FIRMWARE_FILE_RES","modelId": 1234,"version": "1.0.1","firmware": "SGVsbG8sIFdvcmxk"}`

	log.Info("Dummy Message received: " + dummyResponseJson)
	return []byte(dummyResponseJson)
}

func Scheduler() {
	var C2config C2Config = GetC2ConfigFromToml()
	stopScheduler = false
	frequency, err := ParseFrequency(C2config.Frequency)
	if err != nil {
		log.Error(err)
		return
	}
	ticker := time.NewTicker(time.Duration(frequency) * time.Minute)

	defer ticker.Stop()

	retryTicker := time.NewTicker(time.Duration(frequency/4) * time.Minute)

	defer retryTicker.Stop()

	SendUdpMessage("mgfuota,all,sysreadysuccess")
	CheckForFirmwareUpdate()

	retries := 2
	for {
		if stopScheduler == true {
			log.Info("Scheduler is stopped")
			break
		}

		select {
		case <-ticker.C:
			retries = 2
			if stopScheduler == true {
				log.Info("Scheduler is stopped")
				break
			}
			CheckForFirmwareUpdate()
		case <-retryTicker.C:
			if retries > 0 {
				retries--
				if stopScheduler == true {
					log.Info("Scheduler is stopped")
					break
				}
				CheckForFirmwareUpdate()
			} else if retries == 0 {
				retries--
				// SendFailedDevicesStatus()
			}
		}
	}

}

func StopScheduler() {
	stopScheduler = true
}

func SendFailedDevicesStatus() {
	// log.Info("Checking for firmware updates")
	//Request format - {"msg_typ":"FIRMWARE_UPDATES", "ls":0}
	SendWSMessage("{\"msg_type\":\"FIRMWARE_UPDATE\", \"ls\":0}")

	//Respose format - {\"msg_type\":\"FIRMWARE_UPDATES_RES\", \"models\":[{"modelId":1234, \"version\":\"1.0.1\"}]}]};
	// response := ReceiveMessageDummyForModels()
	message := ReceiveWSMessage()

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
				// log.Info("No Active devices in DB of Model Id:", model.ModelId, " and Version <", model.Version)
				return nil
			} else {
				for _, device := range devices {
					SendFailedDevicesStatusToC2(device.DeviceCode, device.FirmwareVersion, model.Version)
				}

			}

			return nil
		}); err != nil {
			log.Fatal(err)
		}
	}
}

func SendFailedDevicesStatusToC2(deviceCode string, deviceVersion string, modelVersion string) {
	var C2config C2Config = GetC2ConfigFromToml()
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

	// var c2config C2Config = getC2ConfigFromToml()
	// Establish a WebSocket connection
	authString := fmt.Sprintf("%s:%s", C2config.Username, C2config.Password)
	encodedAuth := base64.StdEncoding.EncodeToString([]byte(authString))

	// Device authentication
	websocketURL := C2config.ServerURL + encodedAuth + "/true/proto"
	headers := make(http.Header)
	headers.Set("Device", "Basic "+encodedAuth)
	conn, _, err := websocket.DefaultDialer.Dial(websocketURL, nil)
	if err != nil {
		log.Fatalf("Failed to connect to WebSocket /proto: %v", err)
	}
	defer conn.Close()

	// Send the UniversalProto message over the WebSocket
	err = conn.WriteMessage(websocket.BinaryMessage, universalProtoBytes)
	if err != nil {
		log.Fatalf("Failed to send message over WebSocket /proto: %v", err)
	}

	log.Println("Device firmware update failed message sent to C2 for device:", deviceCode)

}

func ParseFrequency(frequency string) (int, error) {
	log.Println("Frequency is:", frequency)
	if len(frequency) == 0 {
		return 0, errors.New("frequency cannot be empty")
	}

	if frequency == "0" {
		return 0, errors.New("frequency cannot be 0")
	}

	numStr := frequency[:len(frequency)-1]
	unit := frequency[len(frequency)-1]

	num, err := strconv.Atoi(numStr)
	if err != nil {
		return 0, err
	}

	// Convert the frequency to minutes
	switch unit {
	case 'm':
		return num, nil // Already in minutes
	case 'h':
		return num * 60, nil // Convert hours to minutes
	case 'd':
		return num * 1440, nil // Convert days to minutes
	default:
		return 0, errors.New("frequency format is invalid (ex:1m,1h,1d)")
	}
}

func CheckForFirmwareUpdate() {

	log.Info("Checking for firmware updates")
	//Request format - {"msg_typ":"FIRMWARE_UPDATES", "ls":0}
	SendWSMessage("{\"msg_type\":\"FIRMWARE_UPDATE\", \"ls\":0}")

	//Respose format - {\"msg_type\":\"FIRMWARE_UPDATES_RES\", \"models\":[{"modelId":1234, \"version\":\"1.0.1\"}]}]};
	// response := ReceiveMessageDummyForModels()
	response := ReceiveWSMessage()

	go handleMessage(response)
}

func handleMessage(message string) {
	var C2config C2Config = GetC2ConfigFromToml()
	var applicationId string = getApplicationId()
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
				log.Info("No Active devices in DB of Model Id:", model.ModelId, " and Version <", model.Version)
				return nil
			} else {
				log.Info(len(devices), " Active devices in DB of Model Id:", model.ModelId, " and Version <", model.Version)
			}

			deviceMap := make(map[string][]storage.Device)

			// Separate devices based on region and store in the map
			for _, device := range devices {

				if err := storage.Transaction(func(tx sqlx.Ext) error {
					region, err := storage.GetRegionByDeviceCode(context.Background(), tx, device.DeviceCode)
					if err != nil {
						return fmt.Errorf("GetRegionByDeviceId error: %w", err)
					}
					deviceMap[region] = append(deviceMap[region], device)
					return nil
				}); err != nil {
					log.Fatal(err)
				}
			}
			var payload []byte = GetFirmwarePayload(model.ModelId, model.Version)

			// Loop over the map and print the region and devices
			for region, devices := range deviceMap {

				readyDevices := prepareDevicesForUpdate(devices, model.Version)

				if len(readyDevices) == 0 {
					continue
				}

				currentTime := time.Now()

				targetTime := currentTime.Add(time.Duration(C2config.FuotaInterval) * time.Second)

				targetTimeEpoch := targetTime.Unix()

				sessionTime := C2config.SessionTime

				sendTimestampToDevices(devices, targetTimeEpoch, sessionTime)

				duration := targetTime.Sub(currentTime)

				if duration < 0 {
					log.Println("The target timestamp is in the past. Exiting.")
					return nil
				}

				timer := time.NewTimer(duration)

				log.Println("Firmware update request scheduled at ", targetTime)
				<-timer.C //waiting until the timestamp hits

				createDeploymentRequest(model.Version, devices, applicationId, region, payload)
				break
			}
			return nil
		}); err != nil {
			log.Fatal(err)
		}
		break
	}
}

func prepareDevicesForUpdate(devices []storage.Device, modelVersion string) []storage.Device {
	var devicesReady []storage.Device
	for _, device := range devices {
		if err := storage.Transaction(func(tx sqlx.Ext) error {
			errr := storage.IncrementDeviceAttempt(context.Background(), tx, device.DeviceCode)
			if errr != nil {
				return fmt.Errorf("IncrementDeviceAttempt error: %w", errr)
			}
			failed, errrr := storage.GetDeviceFirmwareUpdateFailed(context.Background(), tx, device.DeviceCode)
			if errrr != nil {
				return fmt.Errorf("GetDeviceFirmwareUpdateFailed error: %w", errrr)
			}
			if failed == true {
				SendFailedDevicesStatusToC2(device.DeviceCode, device.FirmwareVersion, modelVersion)
			} else {
				devicesReady = append(devicesReady, device)
			}
			return nil
		}); err != nil {
			log.Fatal(err)
		}
	}
	return devicesReady
}

func sendTimestampToDevices(devices []storage.Device, timestamp int64, sessiontime int) {
	for _, device := range devices {

		data := fmt.Sprintf("FUOTA_START,%d,%d,", timestamp, sessiontime)
		dataBytes := []byte(data)

		_, err = as.DeviceClient().Enqueue(context.Background(), &api.EnqueueDeviceQueueItemRequest{
			QueueItem: &api.DeviceQueueItem{
				DevEui:    device.DeviceCode,
				FPort:     2,
				Data:      dataBytes,
				Confirmed: true,
			},
		})

		_, err = as.DeviceClient().Enqueue(context.Background(), &api.EnqueueDeviceQueueItemRequest{
			QueueItem: &api.DeviceQueueItem{
				DevEui: device.DeviceCode,
				FPort:  2,
				Data:   dataBytes,
			},
		})

		if err != nil {
			log.WithError(err).WithFields(log.Fields{
				"dev_eui": device.DeviceCode,
			}).Error("fuota: enqueue timestamp payload error")
		} else {
			log.Println("enqueue timestamp payload success for device:", device.DeviceCode)
		}
	}
}

func createDeploymentRequest(firmwareVersion string, devices []storage.Device, applicationId string, region string, payload []byte) {

	if firmUpdateInfo.FirmUpdateInfo["firmUpdateRunning"] == true {
		log.Info("Current Firmware Update request cannot proceed since another Firmware Update is running for Region:", firmUpdateInfo.FirmUpdateInfo["region"], " and version:", firmUpdateInfo.FirmUpdateInfo["firmVersion"])
		return
	}

	firmUpdateInfo.FirmUpdateInfo["firmUpdateRunning"] = true
	firmUpdateInfo.FirmUpdateInfo["firmVersion"] = firmwareVersion
	firmUpdateInfo.FirmUpdateInfo["region"] = region

	// mcRootKey, err := multicastsetup.GetMcRootKeyForGenAppKey(lorawan.AES128Key{0x09, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})
	mcRootKey, err := multicastsetup.GetMcRootKeyForGenAppKey(lorawan.AES128Key{0x51, 0xA9, 0x98, 0x44, 0x7B, 0x33, 0x04, 0xB3, 0x9A, 0xC1, 0x6A, 0x13, 0x80, 0xE2, 0xA8, 0xEA})
	if err != nil {
		log.Fatal(err)
	}
	log.Info("Creating deployement request for Region:", region, " FirmwareVersion:", firmwareVersion)

	/****************changes advised by ravindra*******************/
	const fragsize uint32 = 220
	const redundancy_per uint32 = 10

	var redundancy_padd uint32 = redundancy_per * fragsize

	padding := redundancy_padd - (uint32(len(payload)) % redundancy_padd)
	fdata := append(payload, make([]byte, padding)...)

	var redundancy uint32 = uint32(len(fdata)) / (fragsize * redundancy_per)
	redundancy++
	var pay_ld_len uint32 = uint32(len(fdata))
	fmt.Println("// file size : ", (len(payload)))
	fmt.Println("// data length : ", pay_ld_len)
	fmt.Println("// fragSize = ", fragsize)
	fmt.Println("// fragNb = ", pay_ld_len/fragsize)
	fmt.Println("// Redundancy = ", redundancy)
	//fragment the payload

	/**************************************************************/

	client := fuota.NewFuotaServerServiceClient(GrpcConn)

	resp, err := client.CreateDeployment(context.Background(), &fuota.CreateDeploymentRequest{
		Deployment: &fuota.Deployment{
			ApplicationId:                     applicationId,
			Devices:                           GetDeploymentDevices(mcRootKey, devices),
			MulticastGroupType:                fuota.MulticastGroupType_CLASS_C,
			MulticastDr:                       12, // DR12
			MulticastFrequency:                923300000,
			MulticastGroupId:                  0,
			MulticastTimeout:                  12, // 2^n seconds
			MulticastRegion:                   regions[region],
			UnicastTimeout:                    ptypes.DurationProto(60 * time.Second),
			UnicastAttemptCount:               5,
			FragmentationFragmentSize:         fragsize,
			Payload:                           fdata,
			FragmentationRedundancy:           redundancy,
			FragmentationSessionIndex:         0,
			FragmentationMatrix:               0,
			FragmentationBlockAckDelay:        300,
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

		log.Info("device eui: " + device.DeviceCode)
		deploymentDevices = append(deploymentDevices, &fuota.DeploymentDevice{
			DevEui:    device.DeviceCode,
			McRootKey: mcRootKey.String(),
		})
	}

	return deploymentDevices
}

func GetFirmwarePayload(modelId int, version string) []byte {
	log.Info("Getting firmware file for Model Id:", modelId, "and Version:", version)
	//Request format - {“msg_type”:”FIRMWARE_FILE”, “filter”:{\”models\”:[\”modelId\”: 1234, \”version\”:”1.0.1”, \”latestVersion\”:false]}}
	request := fmt.Sprintf(`{"msg_type":"FIRMWARE_FILE", "filter":"{\"models\":[{\"modelId\": %d, \"version\": \"%s\", \"latestVersion\": false}]}"}`, modelId, version)
	SendWSMessage(request)

	//Respose format - {"msg_type":"FIRMWARE_FILE_RES", "modelId": 1234, "version": "1.0.1", "firmware":base64}
	// responseMessage := ReceiveMessageDummyForFirmware()
	// responseMessage := ReceiveWSMessage()

	// var response FirmwareResponse
	// if err := json.Unmarshal([]byte(responseMessage), &response); err != nil {
	// 	log.Fatalf("failed to unmarshal response: %v", err)
	// }
	// firmwareBytes, err := base64.StdEncoding.DecodeString(string(response.Firmware))
	// if err != nil {
	// 	log.Error("Error decoding Base64 firmware string:", err)
	// }
	responseBytes := ReceiveWSMessageBinary()

	var universal pb.Universal
	err := proto.Unmarshal(responseBytes, &universal)
	if err != nil {
		log.Fatalf("Failed to unmarshal Universal message for FirmwareFileResponse: %v", err)
	}

	// fmt.Println(universal.Type)
	// fmt.Println(universal.Messages)
	var firmwareFileResponse pb.FirmwareFileResponse
	err = proto.Unmarshal(universal.Messages[0], &firmwareFileResponse)
	if err != nil {
		log.Fatalf("Failed to unmarshal FirmwareFileResponse message: %v", err)
	}
	// fmt.Println(firmwareFileResponse.ModelId)
	// fmt.Println(firmwareFileResponse.Version)
	// fmt.Println(firmwareFileResponse.Firmware)
	return firmwareFileResponse.Firmware
}

func ResetStorage() error {
	//reset storage
	if err := storage.DropAll(); err != nil {
		log.Error(err)
		return err
	} else {
		log.Info("Db reset done and connection closed")
	}
	return nil
}

func CloseDBConn() error {
	if err := storage.CloseConn(); err != nil {
		log.Error(err)
		return err
	}
	return nil
}

func InitializeDB() error {
	if err := storage.Setup(&config.C); err != nil {
		log.Error(err)
		return err
	}
	return nil
}

func GetC2ConfigFromToml() C2Config {

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
	if c2config.Frequency == "" {
		log.Fatal("frequency not found in c2intbootconfig.toml file")
	}

	c2config.FuotaInterval = viper.GetInt64("c2App.fuotainterval")
	if c2config.FuotaInterval == 0 {
		log.Fatal("fuotainterval not found in c2intbootconfig.toml file")
	}
	c2config.SessionTime = viper.GetInt("c2App.sessiontime")
	if c2config.SessionTime == 0 {
		log.Fatal("sessiontime not found in c2intbootconfig.toml file")
	}

	c2config.MulticastIP = viper.GetString("c2App.multicastip")
	if c2config.MulticastIP == "" {
		log.Fatal("multicastip not found in c2intbootconfig.toml file")
	}

	c2config.MulticastPort = viper.GetInt("c2App.multicastport")
	if c2config.MulticastPort == 0 {
		log.Fatal("multicastport not found in c2intbootconfig.toml file")
	}

	return c2config
}

func getApplicationId() string {

	viper.SetConfigName("c2intruntimeconfig")
	viper.SetConfigType("toml")
	viper.AddConfigPath("/usr/local/bin")

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			log.Fatalf("c2intruntimeconfig.toml file not found: %v", err)
		} else {
			log.Fatalf("Error reading c2intruntimeconfig.toml file: %v", err)
		}
	}

	applicationId := viper.GetString("chirpstack.application.id")
	if applicationId == "" {
		log.Fatal("Application id not found in c2intruntimeconfig.toml file")
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

	log.Info("deployment status: ", resp.EnqueueCompletedAt)
}

func SendUdpMessage(message string) {
	var C2config C2Config = GetC2ConfigFromToml()
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
