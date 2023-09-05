package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	tsdb "bosun.org/opentsdb"
	mqtt "github.com/eclipse/paho.mqtt.golang"
)

const (
	TOPIC_TEMPERATURE = "temperature"
	TOPIC_HUMIDITY    = "humidity"
	TOPIC_SENSORS     = "sensors"
	QOS               = 1
	CLIENTID          = "data-logger"

	DATABASE = "/data/sensors.db"
	TSDB_URL = "http://database:6182/api/put"
)

type handler struct {
	db *gorm.DB
}

type Temperature struct {
	TS     uint    // timestamp
	Value  float32 // value in celcius
	Source string  // device where the value comes from
}

type Humidity struct {
	TS     uint    // timestamp
	Value  float32 // value in percent
	Source string  // device where the value comes from
}

func NewHandler() *handler {
	db, err := gorm.Open(sqlite.Open(DATABASE), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}
	db.AutoMigrate(&Temperature{}, &Humidity{})
	return &handler{db: db}
}

func (o *handler) Close() {
	db, _ := o.db.DB()
	db.Close()
}

func (o *handler) handleTemperature(_ mqtt.Client, msg mqtt.Message) {
	// We extract the count and write that out first to simplify checking for missing values
	var t Temperature
	if err := json.Unmarshal(msg.Payload(), &t); err != nil {
		fmt.Printf("Message could not be parsed (%s): %s", msg.Payload(), err)
	}
	fmt.Printf("received message: %s on topic %s\n", msg.Payload(), msg.Topic())
	o.db.Create(&t)
}

func (o *handler) handleHumidity(_ mqtt.Client, msg mqtt.Message) {
	// We extract the count and write that out first to simplify checking for missing values
	var h Humidity
	if err := json.Unmarshal(msg.Payload(), &h); err != nil {
		fmt.Printf("Message could not be parsed (%s): %s", msg.Payload(), err)
	}
	fmt.Printf("received message: %s on topic %s\n", msg.Payload(), msg.Topic())
	o.db.Create(&h)
}

func (o *handler) handleSensorMetric(_ mqtt.Client, msg mqtt.Message) {
	// We extract the count and write that out first to simplify checking for missing values
	var d tsdb.DataPoint
	if err := json.Unmarshal(msg.Payload(), &d); err != nil {
		fmt.Printf("Message could not be parsed (%s): %s", msg.Payload(), err)
		return
	}
	fmt.Printf("received message: %s on topic %s\n", msg.Payload(), msg.Topic())

	req, err := http.NewRequest(http.MethodPost, TSDB_URL, bytes.NewBuffer(msg.Payload()))
	if err != nil {
		fmt.Printf("Cannot prepare request to TSDB at %s with content %s -  error: %s\n", TSDB_URL, msg.Payload(), err.Error())
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Cannot write %s to TSDB - error: %s\n", msg.Payload(), err.Error())
	} else if resp.StatusCode != 200 {
		fmt.Printf("Cannot write %s to TSDB (%d)\n", msg.Payload(), resp.StatusCode)
	}
	defer resp.Body.Close()

}

func waitForSubscription(topic string, t mqtt.Token) {
	<-t.Done()
	if t.Error() != nil {
		fmt.Printf("ERROR SUBSCRIBING: %s\n", t.Error())
	} else {
		fmt.Println("subscribed to: ", topic)
	}
}

func main() {

	fmt.Printf("Starting\n")

	h := NewHandler()
	defer h.Close()

	broker, broker_defined := os.LookupEnv("BROKER")
	if !broker_defined {
		panic("Please specify a mqtt broker")
	}

	opts := mqtt.NewClientOptions()
	opts.AddBroker(broker)
	opts.SetClientID(CLIENTID)

	opts.SetOrderMatters(false)
	opts.ConnectTimeout = time.Second
	opts.WriteTimeout = time.Second
	opts.KeepAlive = 10
	opts.PingTimeout = time.Second

	opts.ConnectRetry = true
	opts.AutoReconnect = true

	opts.DefaultPublishHandler = func(_ mqtt.Client, msg mqtt.Message) {
		fmt.Printf("UNEXPECTED MESSAGE: %s\n", msg)
	}

	opts.OnConnectionLost = func(cl mqtt.Client, err error) {
		fmt.Println("connection lost")
	}

	opts.OnConnect = func(c mqtt.Client) {
		fmt.Println("connection established")

		t := c.Subscribe(TOPIC_TEMPERATURE, QOS, h.handleTemperature)
		go waitForSubscription(TOPIC_TEMPERATURE, t)
		t = c.Subscribe(TOPIC_HUMIDITY, QOS, h.handleHumidity)
		go waitForSubscription(TOPIC_HUMIDITY, t)
		t = c.Subscribe(TOPIC_SENSORS, QOS, h.handleSensorMetric)
		go waitForSubscription(TOPIC_SENSORS, t)

	}
	opts.OnReconnecting = func(mqtt.Client, *mqtt.ClientOptions) {
		fmt.Println("attempting to reconnect")
	}

	client := mqtt.NewClient(opts)

	client.AddRoute(TOPIC_TEMPERATURE, h.handleTemperature)
	client.AddRoute(TOPIC_HUMIDITY, h.handleHumidity)

	fmt.Printf("Connecting to %s\n", broker)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}
	fmt.Println("Connection is up")

	// Messages will be delivered asynchronously so we just need to wait for a signal to shutdown
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	signal.Notify(sig, syscall.SIGTERM)

	<-sig
	fmt.Println("signal caught - exiting")
	client.Disconnect(1000)
	fmt.Println("shutdown complete")

}
