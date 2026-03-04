package main

import (
	"log"
	"net/http"

	"github.com/larsp/co2monitor/meter"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	device     = kingpin.Arg("device", "CO2 Meter device, such as /dev/hidraw2").Required().String()
	listenAddr = kingpin.Arg("listen-address", "The address to listen on for HTTP requests.").
			Default(":8080").String()
	encryptedMode = kingpin.Flag("encrypted", "Force encrypted protocol (older devices)").Bool()
	plaintextMode = kingpin.Flag("plaintext", "Force plaintext protocol (newer TFA Dostmann devices)").Bool()
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

func main() {
	kingpin.Parse()
	http.Handle("/metrics", promhttp.Handler())
	go measure()
	log.Printf("Serving metrics at '%v/metrics'", *listenAddr)
	log.Fatal(http.ListenAndServe(*listenAddr, nil))
}

func measure() {
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
	}
}
