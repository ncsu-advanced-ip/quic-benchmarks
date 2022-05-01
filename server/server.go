// Server used for experiments on "Benchmarking QUIC, When Is It Really Quick?"
// Authors: Xiaochen Li (xli237@ncsu.edu), Bruno Candido Volpato da Cunha (bvolpat@ncsu.edu)
// (North Carolina State University)
package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"flag"
	"fmt"
	"io/ioutil"
	"math/big"
	"net"
	"net/http"

	"github.com/lucas-clemente/quic-go"
	"github.com/lucas-clemente/quic-go/http3"
)

const bufferMaxSize = 1048576 // 1mb

// We start a server echoing data on the first stream the client opens,
// then connect with a client, send the message, and wait for its receipt.
func main() {
	fmt.Println("Starting server...")

	host := flag.String("host", "0.0.0.0", "Host to bind")
	quicPort := flag.Int("quic", 4242, "QUIC port to listen")
	tcpPort := flag.Int("tcp", 4243, "TCP port to listen")
	tcpTlsPort := flag.Int("tcpTls", 4244, "TCP TLS port to listen")
	httpPort := flag.Int("http", 4245, "HTTP port to listen")
	httpsPort := flag.Int("https", 4246, "HTTPS port to listen")
	http3Port := flag.Int("http3", 4247, "HTTP3 port to use")
	//httpQuicPort := flag.Int("httpQuic", 4246, "QUIC HTTP port to listen")

	flag.Parse()

	go echoQuicServer(*host, *quicPort)
	go echoHttp3Server(*host, *http3Port)
	go echoTcpServer(*host, *tcpPort)
	go echoTcpTlsServer(*host, *tcpTlsPort)
	go echoHttpServer(*host, *httpPort)
	go echoHttpsServer(*host, *httpsPort)

	select {}
}

func handleQuicStream(stream quic.Stream) {

	totalBytes := 0

	for {
		buf := make([]byte, bufferMaxSize)
		size, err := stream.Read(buf)
		if err != nil {
			//fmt.Printf("QUIC: Got '%d' bytes\n", totalBytes)
			return
		}

		responseString := pad([]byte(fmt.Sprintf("%d", size)), 8)
		_, err = stream.Write(responseString)
		if err != nil {
			panic(err)
		}

		totalBytes += size

	}

}

func handleQuicSession(sess quic.Session) {
	for {
		stream, err := sess.AcceptStream(context.Background())
		if err != nil {
			return // Using panic here will terminate the program if a new connection has not come in in a while, such as transmitting large file.
		}
		go handleQuicStream(stream)
	}
}

func handleTcp(conn net.Conn) {
	defer conn.Close()

	totalBytes := 0

	for {
		buf := make([]byte, bufferMaxSize)
		size, err := conn.Read(buf)
		if err != nil {
			// fmt.Printf("TCP: Got '%d' bytes\n", totalBytes)
			return
		}

		responseString := pad([]byte(fmt.Sprintf("%d", size)), 8)
		_, err = conn.Write(responseString)
		if err != nil {
			panic(err)
		}

		totalBytes += size

	}
}

// Start a server that echos all data on top of QUIC
func echoQuicServer(host string, quicPort int) error {
	listener, err := quic.ListenAddr(fmt.Sprintf("%s:%d", host, quicPort), generateTLSConfig(), nil)
	if err != nil {
		return err
	}

	fmt.Printf("Started QUIC server! %s:%d\n", host, quicPort)

	for {
		sess, err := listener.Accept(context.Background())
		fmt.Printf("Accepted Connection! %s\n", sess.RemoteAddr())

		if err != nil {
			return err
		}

		go handleQuicSession(sess)
	}
}

// Start a server that echos all data on top of TCP
func echoTcpServer(host string, tcpPort int) error {

	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", host, tcpPort))
	if err != nil {
		return err
	}
	fmt.Printf("Started TCP server! %s:%d\n", host, tcpPort)
	defer listener.Close()

	if err != nil {
		return err
	}

	for {
		conn, err := listener.Accept()
		if err != nil {
			return err
		}

		go handleTcp(conn)
	}
}

// Start a server that echos all data on top of TCP (using TLS)
func echoTcpTlsServer(host string, tcpTlsPort int) error {

	sslCert := generateTLSConfig()

	listener, err := tls.Listen("tcp", fmt.Sprintf("%s:%d", host, tcpTlsPort), sslCert)
	if err != nil {
		return err
	}
	fmt.Printf("Started TCP TLS server! %s:%d\n", host, tcpTlsPort)
	defer listener.Close()

	if err != nil {
		return err
	}

	for {
		conn, err := listener.Accept()
		if err != nil {
			return err
		}

		go handleTcp(conn)
	}
}

// EchoHandler echos back the request as a response
func EchoHandler(writer http.ResponseWriter, request *http.Request) {

	responseData, err := ioutil.ReadAll(request.Body)
	if err != nil {
		panic(err)
	}

	// fmt.Printf("EchoHandler returning %d bytes as response. bytes %d\n", int(request.ContentLength), len(body))
	responseString := pad([]byte(fmt.Sprintf("%d", len(responseData))), 8)
	writer.Write(responseString)
}

func echoHttpServer(host string, httpPort int) {

	fmt.Printf("Started HTTP server! %s:%d\n", host, httpPort)

	mux := http.NewServeMux()
	mux.HandleFunc("/", EchoHandler)

	http.ListenAndServe(fmt.Sprintf("%s:%d", host, httpPort), mux)

}

func echoHttpsServer(host string, httpPort int) {

	sslCert := generateTLSConfig()

	mux := http.NewServeMux()
	mux.HandleFunc("/", EchoHandler)

	server := &http.Server{
		Addr:      fmt.Sprintf("%s:%d", host, httpPort),
		Handler:   mux,
		TLSConfig: sslCert,
	}
	fmt.Printf("Started HTTPS server! %s:%d\n", host, httpPort)

	server.ListenAndServeTLS("", "")
}

func echoHttp3Server(host string, httpPort int) {

	sslCert := generateTLSConfig()

	mux := http.NewServeMux()
	mux.HandleFunc("/", EchoHandler)

	server := &http.Server{
		Addr:      fmt.Sprintf("%s:%d", host, httpPort),
		Handler:   mux,
		TLSConfig: sslCert,
	}

	quicConf := &quic.Config{MaxIncomingStreams: 128, MaxIncomingUniStreams: 128}
	http3Server := &http3.Server{Server: server, QuicConfig: quicConf}
	fmt.Printf("Started HTTPS server! %s:%d\n", host, httpPort)

	http3Server.ListenAndServe()
}

// Setup a bare-bones TLS config for the server
func generateTLSConfig() *tls.Config {
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		panic(err)
	}
	template := x509.Certificate{SerialNumber: big.NewInt(1)}
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		panic(err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		panic(err)
	}
	return &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
		NextProtos:   []string{"h3"},
	}
}

func pad(bb []byte, size int) []byte {
	l := len(bb)
	if l == size {
		return bb
	}
	tmp := make([]byte, size)
	copy(tmp[size-l:], bb)
	return tmp
}
