package main

import (
	"fmt"
	"net"
	"os"
	"time"
	"encoding/json"
	"github.com/eclipse/paho.mqtt.golang"
	"flag"
	"container/list"
	"strings"
	"crypto/cipher"
	"crypto/aes"
	"encoding/hex"
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

var magicKey = []byte{ 0x17, 0x99, 0x6d, 0x09, 0x3d, 0x28, 0xdd, 0xb3, 0xba, 0x69, 0x5a, 0x2e, 0x6f, 0x58, 0x56, 0x2e }

const localeTimeZone = "Europe/Moscow"

type xiaomiDeviceIntf interface {
    GetSID() string
    GetName() string
    SetSID(SID string)
    SetName(Name string)
    GetLastTime() time.Time
    SetLastTime(LastTime time.Time)
    GetModel() string
    SetModel(Model string)
}

type XiaomiDevice struct {

    SID			string
    Name		string
    Model		string
    LastTime		time.Time
    xiaomidevice	xiaomiDeviceIntf

}

func (d *XiaomiDevice) GetSID() string { return d.SID }

func (d *XiaomiDevice) GetName() string { return d.Name }

func (d *XiaomiDevice) SetName(Name string) { d.Name = Name }

func (d *XiaomiDevice) SetSID(SID string) { d.SID = SID }

func (d *XiaomiDevice) GetLastTime() time.Time { return d.LastTime }

func (d *XiaomiDevice) SetLastTime(LastTime time.Time) { d.LastTime = LastTime }

func (d *XiaomiDevice) GetModel() string { return d.Model }

func (d *XiaomiDevice) SetModel(Model string) { d.Model = Model }



type tSensor struct{
    XiaomiDevice
    Voltage		float32
}

type tGateway struct{
    XiaomiDevice
    IPAddress		string
    Password		string
    Token		string
    SecureKey		string
}

func (gw *tGateway) RecalcSecureKey(Token string) string { 

    
    block, err := aes.NewCipher([]byte(gw.Password))

    if err != nil {
	panic(err)
    }

    var ciphertext = make([]byte, 16)

    var mode = cipher.NewCBCEncrypter(block, magicKey)

    mode.CryptBlocks(ciphertext, []byte(Token))

    gw.Token = Token

    gw.SecureKey = hex.EncodeToString(ciphertext)

    return gw.SecureKey
    
}


func (gw *tGateway) GetSecureKey() string { return gw.SecureKey }


type XiaomiList struct {
    list.List
}


func (l *XiaomiList) DeviceBySID(SID string) *list.Element {


    if l.Len() == 0 {
	return nil
    }


    for e:= l.Front(); e != nil; e = e.Next() {
	
	if e.Value.(xiaomiDeviceIntf).GetSID() == SID {
	    return e
	}

    }

    return nil
}


type Config struct {
    mqttURI    		*string
    mqttClientId	*string
    mqttTopic  		*string
    mqttSubscribeTopic	*string
    mqttQos    		*int
    mqttUser   		*string
    mqttPass   		*string
    mqttCleanSession	*bool


    cfgFilename		*string
    lstSensors		map[string]interface{}
    lstGateways		map[string]interface{}
}

var cfg = Config{}

var lstDevices = new(XiaomiList)

var mqttClient mqtt.Client

var udpchannel chan string

func sendUDPMessage(channel chan string, ) {

    for msg := range channel {  // Magic Go - read from endless list
	
	// формируем UDP message

	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(msg), &payload); err != nil {
	    continue
	}


	if (payload["cmd"]!= nil) && (payload["cmd"].(string) == "write") &&
	    (payload["model"]!= nil) && (payload["model"] == "gateway") &&
	    (payload["sid"]!=nil) && (strings.Contains(msg, "%AUTO_SECURE_KEY%")) {
	    

	    
	    var itm *list.Element
	    itm = lstDevices.DeviceBySID(payload["sid"].(string))
	    if itm!=nil {

		if itm.Value.(xiaomiDeviceIntf).GetModel() == payload["model"].(string) {
		    
		    msg = strings.Replace(msg, "%AUTO_SECURE_KEY%", itm.Value.(*tGateway).GetSecureKey(), 1)
		    
		}

	    }


	    fmt.Println("DEBUG To UDO Msg = ", msg)

	}

	// send msg to udp broadcast
	socket, _ := net.DialUDP("udp4", nil, &net.UDPAddr{IP:net.IPv4(224,0,0,50),Port:9898,})
	socket.Write([]byte(msg))
	
    }

}




var mqttRecvHandler mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {

// var mqttRecvHandler mqtt.MessageHandler = func(client *mqtt.Client, msg mqtt.Message) {

   //////fmt.Printf("MSG: %s\n", msg.Payload())
    //udpchannel := make(chan string, 1024)
    //defer close(udpchannel)
    //go sendUDPMessage(udpchannel)


    //channel <- string(buffer[0:n])
    udpchannel <- string(msg.Payload())






   //text:= fmt.Sprintf("this is result msg #%d!", knt)
   //knt++
   //token := client.Publish("nn/result", 0, false, text)
   //token.Wait()
}


func init() {
    cfg.mqttURI = flag.String("broker", "tcp://192.168.1.10:1883", "The broker URI. ex: tcp://192.168.1.10:1883")
    cfg.mqttClientId = flag.String("id", "mqtt-proxy", "The ClientID (optional)")
    cfg.mqttTopic = flag.String("topic", "stat/xiaomi", "The topic name to/from which to publish/subscribe")
    cfg.mqttSubscribeTopic = flag.String("subscribe", "cmd/xiaomi", "The topic name to subscribe")

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
	


	var mapXiaomiDevices map[string]interface{}
	err = json.Unmarshal(bs, &mapXiaomiDevices)
	checkError(err)

	// ищем в настройках gateways
	
	if _, ok:= mapXiaomiDevices["gateways"]; ok {
	    //fmt.Println(keyname, ok)


	    
	for gatewaySID, gatewayPassword:= range mapXiaomiDevices["gateways"].(map[string]interface{}) {

	    lstDevices.PushBack(&tGateway{Password: gatewayPassword.(string), XiaomiDevice: XiaomiDevice{SID: gatewaySID, Model: "gateway"}})
	}

	for deviceSID, deviceName:= range mapXiaomiDevices {
	    
	    if deviceSID!="gateways" {

		var itm *list.Element
		itm = lstDevices.DeviceBySID(deviceSID)

		if itm!= nil {
		
		    itm.Value.(xiaomiDeviceIntf).SetName(deviceName.(string))
		
		} else {
		
		    lstDevices.PushBack(&tSensor{XiaomiDevice: XiaomiDevice{Name: deviceName.(string), SID: deviceSID}})
		}
	    }

	}






	}




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


    if token := mqttClient.Subscribe(*cfg.mqttSubscribeTopic, 0, mqttRecvHandler); token.Wait() && token.Error() != nil {
    // if token := mqttClient.Subscribe(*cfg.mqttSubscribeTopic, 0, nil); token.Wait() && token.Error() != nil {
        fmt.Println(token.Error())
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

	
	// сразу и без промедления отправляем MQTT сообщение
	if publish := mqttClient.Publish(*cfg.mqttTopic, byte(*cfg.mqttQos), false, msg); publish.Wait() && publish.Error() != nil {
		fmt.Println(publish.Error())
	}

	// а теперь парсим и т.п.

	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(msg), &payload); err != nil {
	    continue
	}


	currentTime:= time.Now()
	var dev_cmd string = ""
	_ = dev_cmd

	if payload["cmd"]!= nil {
	    dev_cmd = payload["cmd"].(string)
	}

	if (dev_cmd == "read")||(dev_cmd == "read_ask") {
	    continue
	}

	if (payload["model"] == nil)||(payload["sid"] == nil) {
	    continue
	}
		

	var devName string
	devSID:= payload["sid"].(string)
	devModel:= payload["model"].(string)

	//fmt.Println("2MODEL = ", devModel)

	var devToken string = ""
	var devIPAddress string= ""
	var devVoltage float32 = 0

	_ = devToken
	_ = devIPAddress
	_ = devVoltage

	//fmt.Println("3MODEL = ", devModel)

	if devModel == "gateway" {
	    
	    if payload["token"]!=nil {
		devToken = payload["token"].(string)
	    }

	    if payload["data"]!= nil {

		
		var pdata_json map[string]interface{}

		if err := json.Unmarshal([]byte(payload["data"].(string)), &pdata_json); err != nil {
		    continue
		}		    
		

		if pdata_json["ip"]!=nil {

		    devIPAddress = pdata_json["ip"].(string)
		}

		
	    }
	    
	    // fmt.Println("gateway data = ", devIPAddress)

	} else
	{
	    if payload["Voltage"]!= nil {
		devVoltage = payload["Voltage"].(float32)
	    }

	}


	// ищем в списке item по заданному в udp пакете SID
	var itm *list.Element
	itm = lstDevices.DeviceBySID(devSID)
	if itm!= nil {
	    
	    if itm.Value.(xiaomiDeviceIntf).GetName() == "" {
		itm.Value.(xiaomiDeviceIntf).SetName("Unknown" + strings.Title(devModel) + devSID)
	    }

	    if itm.Value.(xiaomiDeviceIntf).GetModel() == "" {
		itm.Value.(xiaomiDeviceIntf).SetModel(devModel)
	    }
	    
	    itm.Value.(xiaomiDeviceIntf).SetLastTime(currentTime)

	    if devModel == "gateway" {
				
		// itm.Value.(*tGateway).Token = devToken

		itm.Value.(*tGateway).IPAddress = devIPAddress

		if devToken!="" && itm.Value.(*tGateway).Password!="" {

		    var sKey = itm.Value.(*tGateway).RecalcSecureKey(devToken)

		    fmt.Printf("SID = %s, IPAddress = %s, Token = %s, SecureKey = %s\n\r", devSID, devIPAddress, devToken, sKey)


		}


	    } else {
		
		itm.Value.(*tSensor).Voltage = devVoltage

	    }


	    
	} else // itm не найден - это новый элемент!
	{
	    devName = "Unknown" + strings.Title(devModel) + devSID
	    if devModel == "gateway" {

		lstDevices.PushBack(&tGateway{Token: devToken, IPAddress: devIPAddress, XiaomiDevice: XiaomiDevice{Name: devName, SID: devSID, Model: devModel, LastTime: currentTime}})

		// увы, тут нет смысла расчитывать SecureKey, т.к. устройство добавлено автоматически и не имеет пароля из настройки

	    } else {
		lstDevices.PushBack(&tSensor{Voltage: devVoltage, XiaomiDevice: XiaomiDevice{Name: devName, SID: devSID, Model: devModel, LastTime: currentTime}})
	    }
	}


	/*
	if (cfg.lstSensors == nil)||((payload["sid"]!=nil) && (cfg.lstSensors[payload["sid"].(string)]!=nil)) {
	    
	    sensorName := "noname"
	    if (cfg.lstSensors!= nil) {
		sensorName = cfg.lstSensors[payload["sid"].(string)].(string)
	    }

	

	
	fmt.Println("Translate data for sensor", sensorName)


	for e := lstDevices.Front(); e != nil; e = e.Next() {
	//fmt.Println(e.Value.(*XiaomiDevice).Name)
	//fmt.Println(e.Value.(*tSensor).Voltage)
	//fmt.Println(e.Value.Name)
	//fmt.Println(e.Value.XiaomiDevice.(*XiaomiDevice).Name)
	fmt.Printf("SID = %s, Name = %s\n\r", e.Value.(xiaomiDeviceIntf).GetSID(), e.Value.(xiaomiDeviceIntf).GetName())
	}


	    

	    if publish := mqttClient.Publish(*cfg.mqttTopic, byte(*cfg.mqttQos), false, msg); publish.Wait() && publish.Error() != nil {
		fmt.Println(publish.Error())
	    }
	    
	    // fmt.Println("Msg = ", msg)

	}

	*/

  }
}


func main() {


    //var sens tSensor
    //sens.Voltage = 3
    //sens.Name = "SensorName"
    // fmt.Println("sens.Name = ", sens.Name)

    //var l = list.New()
    //var l = new(XiaomiList)
    
    //var l = new(XiaomiList)
    //l.Init()


    //l.PushBack(&tSensor{Name: "Sensor A"})
    //l.PushBack(&tSensor{Name: "Sensor B"})
    //l.PushBack(&tSensor{Name: "Sensor C"})

    //l.PushBack(&tSensor{Voltage: 1, XiaomiDevice: XiaomiDevice{Name: "Name A", SID: "111"}})
    //l.PushBack(&tSensor{Voltage: 2, XiaomiDevice: XiaomiDevice{Name: "Name B", SID: "222"}})
    //l.PushBack(&tSensor{Voltage: 3, XiaomiDevice: XiaomiDevice{Name: "Name C", SID: "333"}})
    //l.PushBack(&tSensor{Voltage: 4, XiaomiDevice: XiaomiDevice{Name: "Name D", SID: "444"}})
    //l.PushBack(&tGateway{Token: "TokenValue1", XiaomiDevice: XiaomiDevice{Name: "Gateway A", SID: "555"}})
    //l.PushBack(&tGateway{Token: "TokenValue2", XiaomiDevice: XiaomiDevice{Name: "Gateway B", SID: "666"}})


    //l.PushBack(sens)


    
/*
    for e := lstDevices.Front(); e != nil; e = e.Next() {
	//fmt.Println(e.Value.(*XiaomiDevice).Name)
	//fmt.Println(e.Value.(*tSensor).Voltage)
	//fmt.Println(e.Value.Name)
	//fmt.Println(e.Value.XiaomiDevice.(*XiaomiDevice).Name)
	fmt.Printf("SID = %s, Name = %s\n\r", e.Value.(xiaomiDeviceIntf).GetSID(), e.Value.(xiaomiDeviceIntf).GetName())

    }
    

    
    // var d = DeviceBySID(l, "555")
    /*
    var d = lstDevices.DeviceBySID("158d0001de8d40")
    if d!= nil {
	fmt.Println(d.Value.(xiaomiDeviceIntf).GetName())
    }
    */


/*
    opts := mqtt.NewClientOptions().AddBroker("tcp://192.168.1.10:1883").SetClientID("mqtt-proxy")
    opts.SetKeepAlive(2 * time.Second)
    opts.SetUsername("esp8266")
    opts.SetPassword("rP09-x27mB3g")
    opts.SetCleanSession(true)

    mqttClient := mqtt.NewClient(opts)
    if token := mqttClient.Connect(); token.Wait() && token.Error() != nil {
	panic(token.Error())
    }
*/


	conn, err := net.ListenMulticastUDP("udp", nil, &net.UDPAddr{IP: net.IPv4(224, 0, 0, 50), Port: 9898})
	checkError(err)
	defer conn.Close()

	buffer := make([]byte, 1024)

	channel := make(chan string, 1024)
	defer close(channel)

	go sendMQTTMessage(channel)

	udpchannel = make(chan string, 1024)
	defer close(udpchannel)
	go sendUDPMessage(udpchannel)

	for {
		n,_, err := conn.ReadFromUDP(buffer)
		//fmt.Println("Received ", string(buffer[0:n]), " from ", addr)

		if err != nil {
			fmt.Println("Error: ", err)
		} else {

		    channel <- string(buffer[0:n])
		}


	}

    
    // mqttClient.Unsubscribe(*cfg.mqttSubscribeTopic)

    mqttClient.Disconnect(250)
    time.Sleep(1 * time.Second)

}
