package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"math/rand"
	"net"
	"strings"
)

const (
	typeARecord = uint16(1)
	classIN     = uint16(1)
)

type DNSHeader struct {
	queryID        uint16
	flags          uint16
	numQuestions   uint16
	numAnswers     uint16
	numAuthorities uint16
	numAdditionals uint16
}

func buildQuery(domainName string, recordType uint16) []byte {
	name := encodeDomain(domainName)
	id := rand.Intn(1 << 16)
	recursionDesiredFlag := 1 << 8

	header := DNSHeader{
		queryID:      uint16(id),
		flags:        uint16(recursionDesiredFlag),
		numQuestions: 1,
	}

	querySize := binary.Size(header) + len(name) + 4
	queryBuf := bytes.NewBuffer(make([]byte, 0, querySize))

	err := binary.Write(queryBuf, binary.BigEndian, &header)
	if err != nil {
		log.Fatal("Failed to write DNS header:", err)
	}

	err = binary.Write(queryBuf, binary.BigEndian, name)
	if err != nil {
		log.Fatal("Failed to write domain name:", err)
	}

	err = binary.Write(queryBuf, binary.BigEndian, recordType)
	if err != nil {
		log.Fatal("Failed to write record type:", err)
	}

	err = binary.Write(queryBuf, binary.BigEndian, classIN)
	if err != nil {
		log.Fatal("Failed to write class:", err)
	}

	return queryBuf.Bytes()
}

func encodeDomain(domainName string) []byte {
	parts := strings.Split(domainName, ".")

	output := make([]byte, 0)
	for _, part := range parts {
		output = append(output, byte(len(part)))
		output = append(output, []byte(part)...)
	}

	output = append(output, byte(0))

	return output
}

func main() {
	query := buildQuery("www.example.com", typeARecord)

	// resolve the address for 8.8.8.8 and port 53
	udpAddr, err := net.ResolveUDPAddr("udp", "8.8.8.8:53")
	if err != nil {
		log.Fatal("Failed to resolve UDP address:", err)
		return
	}

	// create a UDP socket
	conn, err := net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		log.Fatal("Failed to create UDP socket:", err)
		return
	}
	defer conn.Close()

	// send our query
	_, err = conn.Write(query)
	if err != nil {
		log.Fatal("Failed to send query:", err)
		return
	}

	buffer := make([]byte, 1024)
	n, _, err := conn.ReadFromUDP(buffer)
	if err != nil {
		log.Fatal("Failed to read response:", err)
		return
	}

	response := buffer[:n]
	fmt.Println("Response:", response)
}