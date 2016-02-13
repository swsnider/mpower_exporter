package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"
	"net/url"
	"strconv"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	addr       = flag.String("addr", ":9101", "metrics address")
	mpowerAddr = flag.String("mpower-addr", "10.1.10.10:80", "address of the mpower you're scanning.")
	username   = flag.String("username", "ubnt", "username for device")
	password   = flag.String("password", "ubnt", "password for device")

	powerDesc = prometheus.NewDesc(
		prometheus.BuildFQName("mpower", "exporter", "power"),
		"Unknown custom mPower 'power' metric",
		[]string{"port"}, nil)
	outputDesc = prometheus.NewDesc(
		prometheus.BuildFQName("mpower", "exporter", "output"),
		"Whether the port is on or not.",
		[]string{"port"}, nil)
	energyDesc = prometheus.NewDesc(
		prometheus.BuildFQName("mpower", "exporter", "energy"),
		"Unknown custom mPower 'energy' metric",
		[]string{"port"}, nil)
	currentDesc = prometheus.NewDesc(
		prometheus.BuildFQName("mpower", "exporter", "current"),
		"The current flowing through the outlet (unknown units)",
		[]string{"port"}, nil)
	voltageDesc = prometheus.NewDesc(
		prometheus.BuildFQName("mpower", "exporter", "voltage"),
		"The voltage flowing through the outlet (unknown units)",
		[]string{"port"}, nil)
	powerFactorDesc = prometheus.NewDesc(
		prometheus.BuildFQName("mpower", "exporter", "power_factor"),
		"Unknown custom mPower 'powerFactor' metric",
		[]string{"port"}, nil)

	// {
	//     "port": 1,
	//     "output": 1,
	//     "power": 7.19548583,
	//     "energy": 2.8125,
	//     "enabled": 0,
	//     "current": 0.105353891,
	//     "voltage": 117.739974975,
	//     "powerfactor": 0.580076938,
	//     "relay": 1,
	//     "lock": 0
	//   },
)

type mPowerCollector struct{}

func (m *mPowerCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- powerDesc
	ch <- outputDesc
	ch <- energyDesc
	ch <- currentDesc
	ch <- voltageDesc
	ch <- powerFactorDesc
}

type OutletData struct {
	Port        int
	Output      float64
	Power       float64
	Energy      float64
	Current     float64
	Voltage     float64
	Powerfactor float64
}

func (m *mPowerCollector) Collect(ch chan<- prometheus.Metric) {
	pdata := url.Values{
		"username": {*username},
		"password": {*password},
	}.Encode()
	req, err := http.NewRequest("POST", fmt.Sprintf("http://%v/login.cgi", *mpowerAddr), strings.NewReader(pdata))
	if err != nil {
		log.Printf("Unable to create login request: %v", err)
		return
	}
	req.Header.Set("Cookie", "AIROS_SESSIONID=01234567890123456789012345678901")
	_, err = http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("Unable to log into device: %v", err)
		return
	}

	req, err = http.NewRequest("GET", fmt.Sprintf("http://%v/sensors", *mpowerAddr), nil)
	if err != nil {
		log.Printf("Unable to create sensor request: %v", err)
		return
	}
	req.Header.Set("Cookie", "AIROS_SESSIONID=01234567890123456789012345678901")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("Unable to get data from device: %v", err)
		return
	}
	defer resp.Body.Close()

	data := make(map[string][]OutletData)
	err = json.NewDecoder(resp.Body).Decode(data)
	if err != nil {
		log.Printf("Unable to parse data from device: %v", err)
		return
	}
	outlets := data["sensors"]
	if len(outlets) == 0 {
		log.Println("Unable to see data from device")
		return
	}
	for _, outlet := range outlets {
		// Output      float64
		// Power       float64
		// Energy      float64
		// Current     float64
		// Voltage     float64
		// Powerfactor float64
		ch <- prometheus.MustNewConstMetric(
			outputDesc,
			prometheus.GaugeValue,
			outlet.Output,
			strconv.Itoa(outlet.Port),
		)
		ch <- prometheus.MustNewConstMetric(
			powerDesc,
			prometheus.GaugeValue,
			outlet.Power,
			strconv.Itoa(outlet.Port),
		)
		ch <- prometheus.MustNewConstMetric(
			energyDesc,
			prometheus.GaugeValue,
			outlet.Energy,
			strconv.Itoa(outlet.Port),
		)
		ch <- prometheus.MustNewConstMetric(
			currentDesc,
			prometheus.GaugeValue,
			outlet.Current,
			strconv.Itoa(outlet.Port),
		)
		ch <- prometheus.MustNewConstMetric(
			voltageDesc,
			prometheus.GaugeValue,
			outlet.Voltage,
			strconv.Itoa(outlet.Port),
		)
		ch <- prometheus.MustNewConstMetric(
			powerFactorDesc,
			prometheus.GaugeValue,
			outlet.Powerfactor,
			strconv.Itoa(outlet.Port),
		)
	}
}

func main() {
	flag.Parse()
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	prometheus.MustRegister(&mPowerCollector{})

	http.Handle("/metrics", prometheus.Handler())
	http.ListenAndServe(*addr, prometheus.Handler())
}
