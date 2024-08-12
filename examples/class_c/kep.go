package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

type Response struct {
	MsgType string `json:"msg_type"`
	Msg     Msg    `json:"msg"`
}

type Msg struct {
	ID         int         `json:"id"`
	Data       []TagDetail `json:"data"`
	DataSize   int         `json:"dataSize"`
	FinalBatch bool        `json:"finalBatch"`
}

type TagDetail struct {
	TagName string `json:"tagName"`
	TagDesc string `json:"tagDesc"`
}

var channelName string = "Channel100k"

func main() {
	CreatChannel()
	GetTagsFromC2WS()
	// testEpoch()
	// SendUdp()
}

func SendUdp() {
	// multicastAddrStr := "224.1.1.1:7002"
	multicastAddrStr := "127.0.0.1:7002"

	multicastAddr, err := net.ResolveUDPAddr("udp", multicastAddrStr)
	if err != nil {
		fmt.Println("Error resolving UDP address:", err)
	}

	conn, err := net.DialUDP("udp", nil, multicastAddr)
	if err != nil {
		fmt.Println("Error connecting:", err)
		return
	}
	defer conn.Close()

	message := []byte("mgmonitor,all,c2connect")
	_, err = conn.Write(message)
	if err != nil {
		fmt.Println("Error sending message:", err)
		return
	}

	fmt.Println("Message sent:", string(message))

	time.Sleep(3 * time.Second)

	message = []byte("mgmonitor,all,appinit")
	_, err = conn.Write(message)
	if err != nil {
		fmt.Println("Error sending message:", err)
		return
	}

	fmt.Println("Message sent:", string(message))

	time.Sleep(3 * time.Second)

	message = []byte("mgmonitor,all,dbready")
	_, err = conn.Write(message)
	if err != nil {
		fmt.Println("Error sending message:", err)
		return
	}

	fmt.Println("Message sent:", string(message))

	time.Sleep(3 * time.Second)

	message = []byte("mgmonitor,all,sysready")
	_, err = conn.Write(message)
	if err != nil {
		fmt.Println("Error sending message:", err)
		return
	}

	fmt.Println("Message sent:", string(message))

	time.Sleep(3 * time.Second)

}

func testEpoch() {
	currentTime := time.Now()
	futureTime := currentTime.Add(10 * time.Second)

	epochTime := futureTime.Unix()

	targetTime := time.Unix(epochTime, 0)

	fmt.Printf("Current Time: %s\n", currentTime.Format(time.RFC3339))
	fmt.Printf("target Time (10 seconds later): %s\n", futureTime.Format(time.RFC3339))
	fmt.Printf("Unix Epoch Time (10 seconds later): %d\n", epochTime)

	duration := targetTime.Sub(currentTime)

	if duration < 0 {
		fmt.Println("The target timestamp is in the past. Exiting.")
	}

	timer := time.NewTimer(duration)
	fmt.Println("waiting for 10 secs")
	<-timer.C //waiting until the timestamp hits
	fmt.Println("10 secs passed")
}

func GetTagsFromC2WS() {
	username := "saikiran.o2testing"
	password := "HydeVil#71"

	//creating authentication string
	authString := fmt.Sprintf("%s:%s", username, password)
	encodedAuth := base64.StdEncoding.EncodeToString([]byte(authString))

	apiURL := "wss://qa65.assetsense.com/ws/"

	// Establish WebSocket connection with Basic Authorization header
	headers := make(http.Header)
	headers.Set("Authorization", "Basic "+encodedAuth)

	// Establish WebSocket connection
	c, _, err := websocket.DefaultDialer.Dial(apiURL, headers)
	if err != nil {
		log.Fatal("C2 Websocket server is offline", err)
	}

	// Send the request message
	message := `{"msg_type":"GET_TAG_DATA","filter":"{\"dateRange\":{\"id\":0,\"duration\":0,\"selection\":0,\"fromDate\":0,\"toDate\":0},\"id\":0,\"type\":{\"id\":4302},\"lastModified\":0,\"assetName\":\"\",\"startingRow\":-1,\"maxRecordCount\":-1,\"orderByProperty\":\"tagName\",\"descending\":false,\"filterDeleted\":false}"}`
	err = c.WriteMessage(websocket.TextMessage, []byte(message))
	if err != nil {
		log.Fatal("Write error:", err)
	}

	log.Println("Waiting for tags from WS... (Restart if no devices received)")

	cnt := 1
	devCnt := 1
	var deviceName string = "Device1"

	CreateDevice(deviceName)
	// Handle incoming messages from the WebSocket server
	var responses []Response
	for {
		_, message, err := c.ReadMessage()
		// fmt.Println(message)
		if err != nil {
			log.Println("Read error:", err)
			// return
		}
		messsageStr := string(message)
		messsageStr = strings.Replace(messsageStr, `"{`, `{`, -1)
		messsageStr = strings.Replace(messsageStr, `}"`, `}`, -1)
		var response Response
		if err := json.Unmarshal([]byte(messsageStr), &response); err != nil {
			fmt.Println("Error parsing response JSON:", err)
			return
		}

		responses = append(responses, response)
		if response.Msg.FinalBatch == true {
			fmt.Println("All tags received from WS")
			break
		}
	}

	for _, response := range responses {
		for _, tag := range response.Msg.Data {
			if cnt > 990 {
				devCnt++
				cnt = 1
				deviceName = "Device" + strconv.Itoa(devCnt)
				CreateDevice(deviceName)
			}
			fmt.Print("Tag Name:", tag.TagName, " ", deviceName, " ")
			// fmt.Println("Tag Description:", tag.TagDesc)
			AddTagToKep(deviceName, tag.TagName, tag.TagDesc)
			time.Sleep(10 * time.Millisecond)
			// if cnt%10 == 0 {
			// 	time.Sleep(100 * time.Millisecond)
			// }
			cnt++
		}
	}

}

func CreatChannel() {
	// Define the URL and JSON request body
	fmt.Print(channelName + " ")
	url := "http://127.0.0.1:57412/config/v1/project/channels/"

	jsonString := `{
		"common.ALLTYPES_NAME": "new-channel",
		"common.ALLTYPES_DESCRIPTION": "Example Simulator Channel",
		"servermain.MULTIPLE_TYPES_DEVICE_DRIVER": "Simulator",
		"servermain.CHANNEL_DIAGNOSTICS_CAPTURE": false,
		"servermain.CHANNEL_WRITE_OPTIMIZATIONS_METHOD": 2,
		"servermain.CHANNEL_WRITE_OPTIMIZATIONS_DUTY_CYCLE": 10,
		"servermain.CHANNEL_NON_NORMALIZED_FLOATING_POINT_HANDLING": 0,
		"simulator.CHANNEL_ITEM_PERSISTENCE": false
	}`

	// Replace tag name
	jsonString = strings.Replace(jsonString, "new-channel", channelName, 1)
	requestBody := []byte(jsonString) // Replace with your JSON data

	// Create a new HTTP client
	client := &http.Client{}

	// Add authorization header
	authHeader := "Basic " + base64.StdEncoding.EncodeToString([]byte("Administrator"))
	request, err := http.NewRequest("POST", url, bytes.NewBuffer(requestBody))
	if err != nil {
		fmt.Println("Error creating request:", err)
		return
	}
	request.Header.Set("Authorization", authHeader)
	request.Header.Set("Content-Type", "application/json")

	// Make the POST request
	response, err := client.Do(request)
	if err != nil {
		fmt.Println("Error making request:", err)
		return
	}
	defer response.Body.Close()
	defer client.CloseIdleConnections()

	// Check the response status code
	if response.StatusCode == http.StatusCreated {
		fmt.Println("Resource created successfully")
	} else if response.StatusCode == http.StatusBadRequest {
		fmt.Println("Bad request")
	} else {
		fmt.Println("Unexpected status code:", response.StatusCode)
	}
}

func CreateDevice(deviceName string) {
	// Define the URL and JSON request body
	fmt.Print(deviceName + " ")
	url := "http://127.0.0.1:57412/config/v1/project/channels/" + channelName + "/devices/"

	jsonString := `{
		"common.ALLTYPES_NAME": "new-dev",
		"common.ALLTYPES_DESCRIPTION": "",
		"servermain.MULTIPLE_TYPES_DEVICE_DRIVER": "Simulator",
		"servermain.DEVICE_MODEL": 0,
		"servermain.DEVICE_UNIQUE_ID": 3112581556,
		"servermain.DEVICE_CHANNEL_ASSIGNMENT": "Channel1",
		"servermain.DEVICE_ID_FORMAT": 1,
		"servermain.DEVICE_ID_STRING": "1",
		"servermain.DEVICE_ID_HEXADECIMAL": 1,
		"servermain.DEVICE_ID_DECIMAL": 1,
		"servermain.DEVICE_ID_OCTAL": 1,
		"servermain.DEVICE_DATA_COLLECTION": true,
		"servermain.DEVICE_SCAN_MODE": 0,
		"servermain.DEVICE_SCAN_MODE_RATE_MS": 1000,
		"servermain.DEVICE_SCAN_MODE_PROVIDE_INITIAL_UPDATES_FROM_CACHE": false
	}`

	// Replace tag name
	jsonString = strings.Replace(jsonString, "new-dev", deviceName, 1)
	requestBody := []byte(jsonString) // Replace with your JSON data

	// Create a new HTTP client
	client := &http.Client{}

	// Add authorization header
	authHeader := "Basic " + base64.StdEncoding.EncodeToString([]byte("Administrator"))
	request, err := http.NewRequest("POST", url, bytes.NewBuffer(requestBody))
	if err != nil {
		fmt.Println("Error creating request:", err)
		return
	}
	request.Header.Set("Authorization", authHeader)
	request.Header.Set("Content-Type", "application/json")

	// Make the POST request
	response, err := client.Do(request)
	if err != nil {
		fmt.Println("Error making request:", err)
		return
	}
	defer response.Body.Close()
	defer client.CloseIdleConnections()

	// Check the response status code
	if response.StatusCode == http.StatusCreated {
		fmt.Println("Resource created successfully")
	} else if response.StatusCode == http.StatusBadRequest {
		fmt.Println("Bad request")
	} else {
		fmt.Println("Unexpected status code:", response.StatusCode)
	}
}

func AddTagToKep(devicename, name, desc string) {
	// Define the URL and JSON request body
	url := "http://127.0.0.1:57412/config/v1/project/channels/" + channelName + "/devices/" + devicename + "/tags"

	jsonString := `{
		"common.ALLTYPES_NAME": "new-tag",
		"common.ALLTYPES_DESCRIPTION": "new-tag-desc",
		"servermain.TAG_ADDRESS": "SINE (10, 0.000000, 10.000000, 0.050000, 180)",
		"servermain.TAG_DATA_TYPE": 8,
		"servermain.TAG_READ_WRITE_ACCESS": 0,
		"servermain.TAG_SCAN_RATE_MILLISECONDS": 100,
		"servermain.TAG_AUTOGENERATED": false,
		"servermain.TAG_SCALING_TYPE": 0,
		"servermain.TAG_SCALING_RAW_LOW": 0,
		"servermain.TAG_SCALING_RAW_HIGH": 1000,
		"servermain.TAG_SCALING_SCALED_DATA_TYPE": 9,
		"servermain.TAG_SCALING_SCALED_LOW": 0,
		"servermain.TAG_SCALING_SCALED_HIGH": 1000,
		"servermain.TAG_SCALING_CLAMP_LOW": false,
		"servermain.TAG_SCALING_CLAMP_HIGH": false,
		"servermain.TAG_SCALING_NEGATE_VALUE": false,
		"servermain.TAG_SCALING_UNITS": ""
	}`

	// Replace tag name
	jsonString = strings.Replace(jsonString, "new-tag", name, 1)

	// Replace tag description
	jsonString = strings.Replace(jsonString, "new-tag-desc", desc, 1)
	requestBody := []byte(jsonString) // Replace with your JSON data

	// Create a new HTTP client
	client := &http.Client{}

	// Add authorization header
	authHeader := "Basic " + base64.StdEncoding.EncodeToString([]byte("Administrator"))
	request, err := http.NewRequest("POST", url, bytes.NewBuffer(requestBody))
	if err != nil {
		fmt.Println("Error creating request:", err)
		return
	}
	request.Header.Set("Authorization", authHeader)
	request.Header.Set("Content-Type", "application/json")

	// Make the POST request
	response, err := client.Do(request)
	if err != nil {
		fmt.Println("Error making request:", err)
		return
	}
	defer response.Body.Close()
	client.CloseIdleConnections()

	// Check the response status code
	if response.StatusCode == http.StatusCreated {
		fmt.Println("Resource created successfully")
	} else if response.StatusCode == http.StatusBadRequest {
		fmt.Println("Bad request")
	} else {
		fmt.Println("Unexpected status code:", response.StatusCode)
	}
}
