package main

import (
	"bufio"
	"encoding/xml"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/tarm/serial"
	"github.com/yosssi/gmq/mqtt"
	"github.com/yosssi/gmq/mqtt/client"
)

func main() {
	// Set up channel on which to send signal notifications.
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, os.Interrupt, os.Kill)

	// Create an MQTT Client.
	cli := client.New(&client.Options{
		// Define the processing of the error handler.
		ErrorHandler: func(err error) {
			fmt.Println(err)
		},
	})

	// Terminate the Client.
	defer cli.Terminate()

	// Connect to the MQTT Server.
	err := cli.Connect(&client.ConnectOptions{
		Network:  "tcp",
		Address:  "localhost:1883",
		ClientID: []byte("example-client"),
	})
	if err != nil {
		panic(err)
	}

	go func() {
		for {
			readInput(cli)
			// readInput only returns in case of failure.
			// On failure, back off before retrying.
			time.Sleep(1 * time.Second)
		}
	}()
	// Wait for receiving a signal.
	<-sigc

	// Disconnect the Network Connection.
	if err := cli.Disconnect(); err != nil {
		panic(err)
	}
}

func readInput(cli *client.Client) {
	f, err := serial.OpenPort(
		&serial.Config{
			Name: "/dev/ttyUSB0",
			Baud: 57600,
		})
	if err != nil {
		panic(err)
	}
	defer f.Close()
	start := time.Now()
	s := bufio.NewScanner(f)
	for s.Scan() {
		watt := parseWattLine(s.Bytes())
		if watt.Watts1 != "" && time.Since(start).Seconds() > 60 {
			// Publish a message.
			fmt.Printf("Publishing %sW %s°C\n", watt.Watts1, watt.TempC)
			start = time.Now()
			err = cli.Publish(&client.PublishOptions{
				QoS:       mqtt.QoS0,
				TopicName: []byte("home/power/global"),
				Message:   []byte(watt.Watts1),
			})

			if err != nil {
				panic(err)
			}
			err = cli.Publish(&client.PublishOptions{
				QoS:       mqtt.QoS0,
				TopicName: []byte("home/temperature/placard"),
				Message:   []byte(watt.TempC),
			})

			if err != nil {
				panic(err)
			}
		}
	}

	err = s.Err()
	if err != nil {
		panic(err)
	}
}

type wattUsage struct {
	// use strings so setGauge() can check for presence vs zero
	XMLName xml.Name `xml:"msg"`
	TempC   string   `xml:"tmpr"`
	TempF   string   `xml:"tmprF"`
	Watts1  string   `xml:"ch1>watts"`
	Watts2  string   `xml:"ch2>watts"`
}

func parseWattLine(d []byte) *wattUsage {
	w := new(wattUsage)
	err := xml.Unmarshal(d, &w)
	if err != nil {
		fmt.Printf("xml: %v", err)
		return nil
	}

	return w
}
