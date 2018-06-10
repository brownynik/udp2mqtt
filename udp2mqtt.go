package main

import (
	"fmt"
	"net"
	"os"
	"time"
	"encoding/json"
	"github.com/eclipse/paho.mqtt.golang"
	"flag"

)

/*
Options:
 [-help]                      Display help
 [-q 0|1|2]                   Quality of Service
 [-clean]                     CleanSession (true if -clean is present)
 [-id <clientid>]             CliendID
 [-user <user>]               User
 [-password <password>]       Password
 [-broker <uri>]              Broker URI
 [-topic <topic>]             Topic
 [-cfgfile <filename>]        Filename with filter of sensors and other options
*/


type Config struct {
    mqttURI    		*string
    mqttClientId	*string
    mqttTopic  		*string
    mqttQos    		*int
    mqttUser   		*string
    mqttPass   		*string
    mqttCleanSession	*bool

    cfgFilename		*string
    lstSensors		map[string]interface{}
    lstGateways		map[string]interface{}
}

var cfg = Config{}

var mqttClient mqtt.Client

func init() {
    cfg.mqttURI = flag.String("broker", "tcp://192.168.1.10:1883", "The broker URI. ex: tcp://192.168.1.10:1883")
    cfg.mqttClientId = flag.String("id", "mqtt-proxy", "The ClientID (optional)")
    cfg.mqttTopic = flag.String("topic", "stat/xiaomi", "The topic name to/from which to publish/subscribe")
    cfg.mqttQos = flag.Int("qos", 0, "The Quality of Service 0,1,2 (default 0)")
    cfg.mqttUser = flag.String("user", "", "The User (optional)")
    cfg.mqttPass = flag.String("password", "", "The password (optional)")
    cfg.mqttCleanSession = flag.Bool("clean", false, "Set Clean Session (default false)")
    cfg.cfgFilename = flag.String("cfgfile", "./devicelist.json", "The configuration file name (filter by SID and other...)")
    flag.Parse()


/*
    fmt.Printf("Sample Info:\n")
    fmt.Printf("\tbroker:    %s\n", *cfg.mqttURI)
    fmt.Printf("\tclientid:  %s\n", *cfg.mqttClientId)
    fmt.Printf("\tuser:      %s\n", *cfg.mqttUser)
    fmt.Printf("\tpassword:  %s\n", *cfg.mqttPass)
    fmt.Printf("\ttopic:     %s\n", *cfg.mqttTopic)
    fmt.Printf("\tqos:       %d\n", *cfg.mqttQos)
    fmt.Printf("\tcleansess: %v\n", *cfg.mqttCleanSession)
*/

    cfg.lstSensors = nil
    cfg.lstGateways = nil

    if stat, err := os.Stat(*cfg.cfgFilename); err == nil && (!stat.IsDir()) {

	file, err := os.Open(*cfg.cfgFilename)
	checkError(err)
	defer file.Close()

	bs := make([]byte, stat.Size())
	_, err = file.Read(bs)
	checkError(err)
	
	err = json.Unmarshal(bs, &cfg.lstSensors)
	checkError(err)

    }

    

    // os.Exit(0)

    opts := mqtt.NewClientOptions().AddBroker(*cfg.mqttURI)

    if len(*cfg.mqttUser) > 0 && len(*cfg.mqttPass) > 0 {
	opts.SetUsername(*cfg.mqttUser)
	opts.SetPassword(*cfg.mqttPass)
    }

    opts.SetClientID(*cfg.mqttClientId)
    opts.SetKeepAlive(2 * time.Second)
    opts.SetCleanSession(*cfg.mqttCleanSession)
    //opts.SetDefaultPublishHandler(f)
    //opts.SetPingTimeout(1 * time.Second)

    mqttClient = mqtt.NewClient(opts)




    if token := mqttClient.Connect(); token.Wait() && token.Error() != nil {
	// fmt.Println(connect.Error())
	// checkError(err)
	// checkError(errors.New("MQTT Broker not connected"))

	fmt.Println("Error: ", token.Error())
	checkError(token.Error())

    }


}


func checkError(err error) {
	if err != nil {
		fmt.Println("Error: ", err)
		os.Exit(0)
	}
}


func sendMQTTMessage(channel chan string, ) {
    for msg := range channel {  // Magic Go - read from endless list
	
	// fmt.Println("Msg = ", msg)

	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(msg), &payload); err != nil {
	    continue
	}


	if (cfg.lstSensors == nil)||((payload["sid"]!=nil) && (cfg.lstSensors[payload["sid"].(string)]!=nil)) {
	    
	    sensorName := "noname"
	    if (cfg.lstSensors!= nil) {
		sensorName = cfg.lstSensors[payload["sid"].(string)].(string)
	    }

	    fmt.Println("Translate data for sensor", sensorName)


	    // check for gateway
	    if (payload["model"]!=nil)&&(payload["model"].(string) == "gateway") {

		// create gateways interface
		if cfgGateways == nil {
		    
		
		}

	    }


	    if publish := mqttClient.Publish(*cfg.mqttTopic, byte(*cfg.mqttQos), false, msg); publish.Wait() && publish.Error() != nil {
		fmt.Println(publish.Error())
	    }
	    
	    fmt.Println("Msg = ", msg)

	}

  }
}


func main() {
	conn, err := net.ListenMulticastUDP("udp", nil, &net.UDPAddr{IP: net.IPv4(224, 0, 0, 50), Port: 9898})
	checkError(err)
	defer conn.Close()

	buffer := make([]byte, 1024)

	channel := make(chan string, 1024)
	defer close(channel)

	go sendMQTTMessage(channel)


	for {
		n,_, err := conn.ReadFromUDP(buffer)
		//fmt.Println("Received ", string(buffer[0:n]), " from ", addr)

		if err != nil {
			fmt.Println("Error: ", err)
		} else {

		    channel <- string(buffer[0:n])
		}


	}

    mqttClient.Disconnect(250)
    time.Sleep(1 * time.Second)

}
