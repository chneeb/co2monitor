package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/larsp/co2monitor/meter"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	device     = kingpin.Arg("device", "CO2 Meter device, such as /dev/hidraw2").Required().String()
	listenAddr = kingpin.Arg("listen-address", "The address to listen on for HTTP requests.").
			Default(":8080").String()
	encryptedMode  = kingpin.Flag("encrypted", "Force encrypted protocol (older devices)").Bool()
	plaintextMode  = kingpin.Flag("plaintext", "Force plaintext protocol (newer TFA Dostmann devices)").Bool()
	mqttHost       = kingpin.Flag("mqtt-host", "MQTT broker host (e.g. localhost:1883); omit to disable MQTT").String()
	mqttTopic      = kingpin.Flag("mqtt-topic", "MQTT topic to publish measurements to").String()
	mqttThreshold  = kingpin.Flag("mqtt-co2-threshold", "CO2 ppm threshold above which co2_detected is true").Default("1800").Int()
)

var (
	temperature = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "meter_temperature_celsius",
		Help: "Current temperature in Celsius",
	})
	co2 = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "meter_co2_ppm",
		Help: "Current CO2 level (ppm)",
	})
)

func init() {
	prometheus.MustRegister(temperature)
	prometheus.MustRegister(co2)
}

type mqttPayload struct {
	Temperature float64 `json:"temperature"`
	Co2         int     `json:"co2"`
	Co2Detected bool    `json:"co2_detected"`
	LinkQuality int     `json:"linkquality"`
}

func newMQTTClient(host string) mqtt.Client {
	opts := mqtt.NewClientOptions().
		AddBroker(fmt.Sprintf("tcp://%s", host)).
		SetClientID("co2monitor").
		SetAutoReconnect(true)
	client := mqtt.NewClient(opts)
	if tok := client.Connect(); tok.Wait() && tok.Error() != nil {
		log.Fatalf("MQTT connect failed: %v", tok.Error())
	}
	log.Printf("Connected to MQTT broker at %v", host)
	return client
}

func main() {
	kingpin.Parse()
	http.Handle("/metrics", promhttp.Handler())

	var mqttClient mqtt.Client
	if *mqttHost != "" && *mqttTopic != "" {
		mqttClient = newMQTTClient(*mqttHost)
	} else if *mqttHost != "" || *mqttTopic != "" {
		log.Fatal("Both --mqtt-host and --mqtt-topic must be set to enable MQTT")
	}

	go measure(mqttClient)
	log.Printf("Serving metrics at '%v/metrics'", *listenAddr)
	log.Fatal(http.ListenAndServe(*listenAddr, nil))
}

func measure(mqttClient mqtt.Client) {
	m := new(meter.Meter)
	switch {
	case *encryptedMode:
		m.SetMode(meter.ModeEncrypted)
	case *plaintextMode:
		m.SetMode(meter.ModePlaintext)
	}

	err := m.Open(*device)
	if err != nil {
		log.Fatalf("Could not open '%v'", *device)
		return
	}

	for {
		result, err := m.Read()
		if err != nil {
			log.Fatalf("Something went wrong: '%v'", err)
		}
		temperature.Set(result.Temperature)
		co2.Set(float64(result.Co2))

		if mqttClient != nil {
			payload := mqttPayload{
				Temperature: result.Temperature,
				Co2:         result.Co2,
				Co2Detected: result.Co2 >= *mqttThreshold,
				LinkQuality: 255,
			}
			data, err := json.Marshal(payload)
			if err != nil {
				log.Printf("MQTT marshal error: %v", err)
				continue
			}
			mqttClient.Publish(*mqttTopic, 0, false, data)
		}
	}
}
