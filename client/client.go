// Client used for experiments on "Benchmarking QUIC, When Is It Really Quick?"
// Authors: Xiaochen Li (xli237@ncsu.edu), Bruno Candido Volpato da Cunha (bvolpat@ncsu.edu)
// (North Carolina State University)

package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"golang.org/x/net/http2"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/lucas-clemente/quic-go" // my go extension did this everytime I hit save.
	"github.com/lucas-clemente/quic-go/http3"
	"github.com/mackerelio/go-osstat/cpu"
	"github.com/mackerelio/go-osstat/memory"
)

const initialMessageSize = 1      // 1 byte
const finalMessageSize = 67108864 // 64 mb
const bufferMaxSize = 1048576     // 1mb
const filesToSend = 10            // Duplicate files to send over the same session, using multi-stream or other techniques if applicable.
const sampleSizes = 5             // The number of times each experiment is executed

var dataBuffer []byte = nil
var httpByteBuffer [][]byte = nil

func getSizeString(size int) string {
	newSize := float64(size)
	unit := "b"
	if size >= 1048576 {
		unit = "mib"
		newSize /= 1048576.0
	} else if size >= 1024 {
		unit = "kib"
		newSize /= 1024.0
	}
	return fmt.Sprintf("%.0f %s", newSize, unit)
}

func report(protocol string, environment string, kind string, filesToSend int, setupDuration time.Duration, firstByteDuration time.Duration, size int,
	duration time.Duration, memoryBefore *memory.Stats, memoryAfter *memory.Stats, cpuBefore *cpu.Stats, cpuAfter *cpu.Stats) {

	fileSizeStr := getSizeString(size)
	goodput := (float64(size) / duration.Seconds()) * float64(filesToSend)

	fmt.Printf("[%s - %s] [%d files] setup: %s, firstbyte: %s, sent: %s, duration: %s (goodput: %.0f kbps)\n", protocol, environment, filesToSend, setupDuration, firstByteDuration, fileSizeStr, duration, goodput/1024.0)

	if size >= 32 {

		fileName := fmt.Sprintf("/var/log/output/meter_%s.csv", environment)

		f, err := os.OpenFile(fileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0777)
		if err != nil {
			panic(err)
		}

		cpuUser := cpuAfter.User - cpuBefore.User
		cpuSystem := cpuAfter.System - cpuBefore.System
		cpuTotal := cpuAfter.Total - cpuBefore.Total

		memoryDiff := memoryAfter.Used - memoryBefore.Used

		// environment
		f.WriteString(fmt.Sprintf("%s,%s,%s,%d,", protocol, kind, environment, filesToSend))
		f.WriteString(fmt.Sprintf("%d,%d,", setupDuration.Microseconds(), firstByteDuration.Microseconds()))
		f.WriteString(fmt.Sprintf("%s,%d,%f,", fileSizeStr, duration.Microseconds(), goodput))
		f.WriteString(fmt.Sprintf("%d,%d,%d,", cpuUser, cpuSystem, cpuTotal))
		f.WriteString(fmt.Sprintf("%d,%d", int(memoryDiff/1048576.0), int(memoryAfter.Used/1048576.0)))
		f.WriteString("\n")
	}
}

// We start a server echoing data on the first stream the client opens,
// then connect with a client, send the message, and wait for its receipt.
func main() {
	host := flag.String("host", "localhost", "Host to connect")
	environment := flag.String("env", "Local", "Environment name")

	quicPort := flag.Int("quic", 4242, "QUIC port to connect")
	tcpPort := flag.Int("tcp", 4243, "TCP port to connect")
	tcpTlsPort := flag.Int("tcpTls", 4244, "TCP TLS port to connect")
	httpPort := flag.Int("http", 4245, "HTTP port to connect")
	httpsPort := flag.Int("https", 4246, "HTTPS port to connect")
	http3Port := flag.Int("http3", 4247, "HTTP3 port to connect")
	flag.Parse()

	// Run the loops a bunch of times
	for i := 0; i < sampleSizes; i++ {

		// Set up random data to send.
		dataBuffer = make([]byte, finalMessageSize)
		rand.Read(dataBuffer)

		size := 1
		i := 0
		for size <= finalMessageSize {
			size *= 2
			i++
		}
		finalIteraction := i

		size = 1
		i = 0
		httpByteBuffer = make([][]byte, finalIteraction)
		for size <= finalMessageSize {
			httpByteBuffer[i] = make([]byte, size)
			rand.Read(httpByteBuffer[i])
			size *= 2
			i++
		}

		fmt.Printf("Starting clients to reach %s...\n", *host)

		// Run QUIC first, early feedback on UDP connections
		if *quicPort > 0 {
			errQuic := clientQuicMain(*environment, *host, *quicPort)
			if errQuic != nil {
				panic(errQuic)
			}
		}

		// HTTP-related tests
		if *httpPort > 0 {
			errHttp := clientHttpMain(*environment, *host, *httpPort)
			if errHttp != nil {
				panic(errHttp)
			}
		}

		if *httpsPort > 0 {
			errHttps := clientHttpsMain(*environment, *host, *httpsPort, false, filesToSend)
			if errHttps != nil {
				panic(errHttps)
			}

			errHttpsMult := clientHttpsMain(*environment, *host, *httpsPort, true, 2)
			if errHttpsMult != nil {
				panic(errHttpsMult)
			}
			errHttpsMult = clientHttpsMain(*environment, *host, *httpsPort, true, 4)
			if errHttpsMult != nil {
				panic(errHttpsMult)
			}
			errHttpsMult = clientHttpsMain(*environment, *host, *httpsPort, true, 8)
			if errHttpsMult != nil {
				panic(errHttpsMult)
			}
		}

		if *http3Port > 0 {
			errHttp3 := clientHttp3Main(*environment, *host, *http3Port, false, filesToSend)
			if errHttp3 != nil {
				panic(errHttp3)
			}

			errHttp3Mult := clientHttp3Main(*environment, *host, *http3Port, true, 2)
			if errHttp3Mult != nil {
				panic(errHttp3Mult)
			}
			errHttp3Mult = clientHttp3Main(*environment, *host, *http3Port, true, 4)
			if errHttp3Mult != nil {
				panic(errHttp3Mult)
			}
			errHttp3Mult = clientHttp3Main(*environment, *host, *http3Port, true, 8)
			if errHttp3Mult != nil {
				panic(errHttp3Mult)
			}
		}

		// Raw protocol tests
		if *tcpPort > 0 {
			errTcp := clientTcpMain(*environment, *host, *tcpPort)
			if errTcp != nil {
				panic(errTcp)
			}
		}

		if *tcpTlsPort > 0 {
			errTcpTls := clientTcpTlsMain(*environment, *host, *tcpTlsPort)
			if errTcpTls != nil {
				panic(errTcpTls)
			}
		}
	}

}

func getFirstByte(protocol string, environment string, write func(data []byte) (n int, err error), read func(buf []byte) (n int, err error)) error {
	// send a single byte to get a response.
	// This should be at least 1 round trip time, but should be a bit longer.

	oneByte := make([]byte, 1)
	_, err := write(oneByte)
	if err != nil {
		return err
	}

	buf := make([]byte, 8)
	_, errRead := read(buf)
	if errRead != nil {
		return err
	}

	return nil

}

func flood(protocol string, environment string, size int, write func(data []byte) (n int, err error), read func(buf []byte) (n int, err error)) error {

	// start := time.Now()

	finishedSend := make(chan bool)
	finishedRecv := make(chan bool)

	totalSent := 0
	go func(finished chan bool) {
		left := size
		for left > 0 {
			current := min(left, bufferMaxSize)

			_, err := write(dataBuffer[totalSent : totalSent+current])
			if err != nil {
				fmt.Println(err)
				finished <- false
				break
			} else {
				totalSent += current
				left -= current
			}
		}

		finished <- true
	}(finishedSend)

	// var duration time.Duration
	go func(finished chan bool) {
		received := 0
		for received < size {
			buf := make([]byte, 8)
			_, err := read(buf)
			if err != nil {
				fmt.Println(err)
				finished <- false
				break
			}

			sizeString := string(bytes.Trim(buf, "\x00"))
			sizeRecv, _ := strconv.Atoi(sizeString)
			received += int(sizeRecv)
		}
		finished <- true

	}(finishedRecv)

	sendOk := <-finishedSend
	recvOk := <-finishedRecv

	// Code to measure
	// duration := time.Since(start)

	if sendOk && recvOk {
		// report(protocol, environment, "Raw", setupDuration, size, duration)
		return nil
	} else {
		return fmt.Errorf("%s: %d did not finish ", protocol, size)
	}
}

func clientQuicMain(environment string, host string, quicPort int) error {
	fmt.Println("Testing QUIC...")
	protocolName := "QUIC" // for report and logging strings
	url := fmt.Sprintf("%s:%d", host, quicPort)
	tlsConf := &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"h3"},
	}

	size := initialMessageSize
	for size <= finalMessageSize {

		memoryBefore, err2 := memory.Get()
		if err2 != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err2)
			return err2
		}

		cpuBefore, err1 := cpu.Get()
		if err1 != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err1)
			return err1
		}

		session, err := quic.DialAddr(url, tlsConf, nil)
		if err != nil {
			return err
		}

		start := time.Now()
		stream, err := session.OpenStreamSync(context.Background())
		if err != nil {
			return err
		}
		defer stream.Close()
		setupDuration := time.Since(start)
		getFirstByte(protocolName, environment, stream.Write, stream.Read)
		firstByteDuration := time.Since(start)

		floodStart := time.Now()
		for fileNum := 0; fileNum < filesToSend; fileNum++ {
			err = flood(protocolName, environment, size, stream.Write, stream.Read)
		}

		duration := time.Since(floodStart)
		if err != nil {
			fmt.Println(err)
		} else {
			cpuAfter, err1 := cpu.Get()
			if err1 != nil {
				fmt.Fprintf(os.Stderr, "%s\n", err1)
				return err1
			}

			memoryAfter, err2 := memory.Get()
			if err2 != nil {
				fmt.Fprintf(os.Stderr, "%s\n", err2)
				return err2
			}
			report(protocolName, environment, "Raw", filesToSend, setupDuration, firstByteDuration, size, duration, memoryBefore, memoryAfter, cpuBefore, cpuAfter)
		}

		size *= 2
	}
	return nil
}

func clientTcpMain(environment string, host string, tcpPort int) error {
	fmt.Println("Testing TCP...")
	protocolName := "TCP" // for report and logging strings
	url := fmt.Sprintf("%s:%d", host, tcpPort)
	size := initialMessageSize
	for size <= finalMessageSize {
		memoryBefore, err2 := memory.Get()
		if err2 != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err2)
			return err2
		}

		cpuBefore, err1 := cpu.Get()
		if err1 != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err1)
			return err1
		}

		start := time.Now()

		session, err := net.Dial("tcp", url)
		if err != nil {
			return err
		}
		defer session.Close()
		setupDuration := time.Since(start)
		getFirstByte(protocolName, environment, session.Write, session.Read)
		firstByteDuration := time.Since(start)

		floodStart := time.Now()
		for fileNum := 0; fileNum < filesToSend; fileNum++ {
			err = flood(protocolName, environment, size, session.Write, session.Read)
		}
		duration := time.Since(floodStart)
		if err != nil {
			fmt.Println(err)
		} else {
			cpuAfter, err1 := cpu.Get()
			if err1 != nil {
				fmt.Fprintf(os.Stderr, "%s\n", err1)
				return err1
			}
			memoryAfter, err2 := memory.Get()
			if err2 != nil {
				fmt.Fprintf(os.Stderr, "%s\n", err2)
				return err2
			}
			report(protocolName, environment, "Raw", filesToSend, setupDuration, firstByteDuration, size, duration, memoryBefore, memoryAfter, cpuBefore, cpuAfter)
		}

		size *= 2
	}
	return nil
}

func clientTcpTlsMain(environment string, host string, tcpPort int) error {
	fmt.Println("Testing TCP-TLS...")
	protocolName := "TCP_TLS" // for report and logging strings
	tlsConf := &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"h3"},
	}
	url := fmt.Sprintf("%s:%d", host, tcpPort)

	size := initialMessageSize
	for size <= finalMessageSize {
		memoryBefore, err2 := memory.Get()
		if err2 != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err2)
			return err2
		}

		cpuBefore, err1 := cpu.Get()
		if err1 != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err1)
			return err1
		}

		start := time.Now()

		session, err := tls.Dial("tcp", url, tlsConf)
		if err != nil {
			return err
		}
		defer session.Close()
		setupDuration := time.Since(start)
		getFirstByte(protocolName, environment, session.Write, session.Read)
		firstByteDuration := time.Since(start)

		floodStart := time.Now()
		for fileNum := 0; fileNum < filesToSend; fileNum++ {
			err = flood("TCP_TLS", environment, size, session.Write, session.Read)
		}
		duration := time.Since(floodStart)
		if err != nil {
			fmt.Println(err)
		} else {
			cpuAfter, err1 := cpu.Get()
			if err1 != nil {
				fmt.Fprintf(os.Stderr, "%s\n", err1)
				return err1
			}
			memoryAfter, err2 := memory.Get()
			if err2 != nil {
				fmt.Fprintf(os.Stderr, "%s\n", err2)
				return err2
			}
			report(protocolName, environment, "Raw", filesToSend, setupDuration, firstByteDuration, size, duration, memoryBefore, memoryAfter, cpuBefore, cpuAfter)
		}

		size *= 2
	}

	return nil
}

func getFirstByteHttp(protocol string, environment string, client *http.Client, url string) error {
	reader := bytes.NewReader(httpByteBuffer[0])

	response, err := client.Post(url, "application/octet-stream", reader)
	if err != nil {
		return err
	}

	_, errDump := ioutil.ReadAll(response.Body)
	if errDump != nil {
		return err
	}

	return nil
}

func floodHttp(protocol string, environment string, size int, sizeIndex int, client *http.Client, url string) error {
	// shuf := make([]byte, size)
	// rand.Read(shuf)

	// reader := bytes.NewBuffer(shuf)
	reader := bytes.NewReader(httpByteBuffer[sizeIndex])
	// fmt.Printf("[%s - %s] Sending %d bytes (size index: %d)\n", protocol, environment, len(httpByteBuffer[sizeIndex]), sizeIndex)

	// start := time.Now()
	response, err := client.Post(url, "application/octet-stream", reader)
	if err != nil {
		return err
	}

	_, errDump := ioutil.ReadAll(response.Body)
	if errDump != nil {
		return err
	}

	// duration := time.Since(start) // moved to before processing response.

	// fmt.Println("HTTP Response", bytesResponse)
	//fmt.Printf("Flood %s %s, sent %d, recv %d\n", protocol, environment, size, len(received))

	// report(protocol, environment, "HTTP", setupDuration, size, duration)

	return nil
}

func clientHttpMain(environment string, host string, httpsPort int) error {
	fmt.Println("Testing HTTP...")
	protocolName := "HTTP/1" // for report and logging strings
	url := fmt.Sprintf("http://%s:%d/", host, httpsPort)

	customTransport := &(*http.DefaultTransport.(*http.Transport)) // make shallow copy

	size := initialMessageSize
	sizeIndex := 0
	for size <= finalMessageSize {
		memoryBefore, err2 := memory.Get()
		if err2 != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err2)
			return err2
		}

		cpuBefore, err1 := cpu.Get()
		if err1 != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err1)
			return err1
		}

		start := time.Now()

		client := &http.Client{Transport: customTransport}
		setupDuration := time.Since(start)
		getFirstByteHttp(protocolName, environment, client, url)
		firstByteDuration := time.Since(start)

		floodStart := time.Now()
		var err error
		for fileNum := 0; fileNum < filesToSend; fileNum++ {
			err = floodHttp(protocolName, environment, size, sizeIndex, client, url)
		}
		duration := time.Since(floodStart)
		if err != nil {
			fmt.Println(err)
		} else {
			cpuAfter, err1 := cpu.Get()
			if err1 != nil {
				fmt.Fprintf(os.Stderr, "%s\n", err1)
				return err1
			}
			memoryAfter, err2 := memory.Get()
			if err2 != nil {
				fmt.Fprintf(os.Stderr, "%s\n", err2)
				return err2
			}
			report(protocolName, environment, "HTTP", filesToSend, setupDuration, firstByteDuration, size, duration, memoryBefore, memoryAfter, cpuBefore, cpuAfter)
		}

		client.CloseIdleConnections()

		size *= 2
		sizeIndex++
	}
	return nil
}

func clientHttpsMain(environment string, host string, httpsPort int, multiplex bool, multiFilesToSend int) error {

	fmt.Println("Testing HTTPS...")
	protocolName := "HTTP/2" // for report and logging strings
	if multiplex {
		protocolName += " (Multiplex)"
	}

	url := fmt.Sprintf("https://%s:%d/", host, httpsPort)
	tlsConf := &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"h2"},
	}
	customTransport := &http2.Transport{TLSClientConfig: tlsConf, StrictMaxConcurrentStreams: true, AllowHTTP: false}

	size := initialMessageSize
	sizeIndex := 0

	for size <= finalMessageSize {
		memoryBefore, err2 := memory.Get()
		if err2 != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err2)
			return err2
		}
		cpuBefore, err1 := cpu.Get()
		if err1 != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err1)
			return err1
		}

		start := time.Now()

		client := &http.Client{Transport: customTransport}
		defer client.CloseIdleConnections()
		setupDuration := time.Since(start)
		getFirstByteHttp(protocolName, environment, client, url)
		firstByteDuration := time.Since(start)
		floodStart := time.Now()

		var wg sync.WaitGroup

		var err error
		for fileNum := 0; fileNum < multiFilesToSend; fileNum++ {
			if multiplex {
				wg.Add(1)

				go func(wg *sync.WaitGroup) {
					err = floodHttp(protocolName, environment, size, sizeIndex, client, url)
					wg.Done()

				}(&wg)
			} else {
				err = floodHttp(protocolName, environment, size, sizeIndex, client, url)
			}
		}

		wg.Wait()

		duration := time.Since(floodStart)
		if err != nil {
			fmt.Println(err)
		} else {
			cpuAfter, err1 := cpu.Get()
			if err1 != nil {
				fmt.Fprintf(os.Stderr, "%s\n", err1)
				return err1
			}
			memoryAfter, err2 := memory.Get()
			if err2 != nil {
				fmt.Fprintf(os.Stderr, "%s\n", err2)
				return err2
			}
			report(protocolName, environment, "HTTP", multiFilesToSend, setupDuration, firstByteDuration, size, duration, memoryBefore, memoryAfter, cpuBefore, cpuAfter)
		}

		size *= 2
		sizeIndex++

	}

	return nil
}

func clientHttp3Main(environment string, host string, http3Port int, multiplex bool, multiFilesToSend int) error {

	fmt.Println("Testing HTTP3...")
	protocolName := "HTTP/3 (QUIC)" // for report and logging strings
	if multiplex {
		protocolName += " (Multiplex)"
	}

	url := fmt.Sprintf("https://%s:%d/", host, http3Port)
	tlsConf := &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"h3"},
	}
	quicConfig := &quic.Config{KeepAlive: true}
	quicTransport := &http3.RoundTripper{TLSClientConfig: tlsConf, QuicConfig: quicConfig}

	size := initialMessageSize
	sizeIndex := 0
	for size <= finalMessageSize {
		memoryBefore, err2 := memory.Get()
		if err2 != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err2)
			return err2
		}

		cpuBefore, err1 := cpu.Get()
		if err1 != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err1)
			return err1
		}

		start := time.Now()

		client := &http.Client{Transport: quicTransport}
		defer client.CloseIdleConnections()
		setupDuration := time.Since(start)
		getFirstByteHttp(protocolName, environment, client, url)
		firstByteDuration := time.Since(start)
		floodStart := time.Now()

		var wg sync.WaitGroup

		var err error
		for fileNum := 0; fileNum < multiFilesToSend; fileNum++ {
			if multiplex {
				wg.Add(1)

				go func(wg *sync.WaitGroup) {
					err = floodHttp(protocolName, environment, size, sizeIndex, client, url)
					wg.Done()

				}(&wg)
			} else {
				err = floodHttp(protocolName, environment, size, sizeIndex, client, url)
			}
		}

		wg.Wait()

		duration := time.Since(floodStart)
		if err != nil {
			fmt.Println(err)
		} else {
			cpuAfter, err1 := cpu.Get()
			if err1 != nil {
				fmt.Fprintf(os.Stderr, "%s\n", err1)
				return err1
			}

			memoryAfter, err2 := memory.Get()
			if err2 != nil {
				fmt.Fprintf(os.Stderr, "%s\n", err2)
				return err2
			}
			report(protocolName, environment, "HTTP", multiFilesToSend, setupDuration, firstByteDuration, size, duration, memoryBefore, memoryAfter, cpuBefore, cpuAfter)
		}

		size *= 2
		sizeIndex++
	}

	return nil
}

/**
 * Return the minimum value between a and b
 */
func min(a int, b int) int {
	if a < b {
		return a
	}

	return b
}
