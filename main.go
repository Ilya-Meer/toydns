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
	QueryID        uint16
	Flags          uint16
	NumQuestions   uint16
	NumAnswers     uint16
	NumAuthorities uint16
	NumAdditionals uint16
}

type DNSQuestion struct {
	domainName  []byte
	recordType  uint16
	recordClass uint16
}

func buildQuery(domainName string, recordType uint16) []byte {
	name := encodeDomain(domainName)
	id := rand.Intn(1 << 16)
	recursionDesiredFlag := 1 << 8

	// number of bytes of domain name, record type, and class
	questionLength := len(name) + 2 + 2

	header := DNSHeader{
		QueryID:      uint16(id),
		Flags:        uint16(recursionDesiredFlag),
		NumQuestions: 1,
	}

	querySize := binary.Size(header) + questionLength
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

type DNSResponseParser struct {
	buffer *bytes.Buffer
}

func NewParser(response []byte) *DNSResponseParser {
	return &DNSResponseParser{bytes.NewBuffer(response)}
}

func (d *DNSResponseParser) parseHeader() (*DNSHeader, error) {
	header := &DNSHeader{}

	err := binary.Read(d.buffer, binary.BigEndian, header)
	if err != nil {
		return nil, err
	}

	return header, nil
}

func (d *DNSResponseParser) parseQuestion() (*DNSQuestion, error) {
	// Read domain name
	nameParts := make([][]byte, 0)

	for {
		lenByte := make([]byte, 1)
		_, err := d.buffer.Read(lenByte)
		if err != nil {
			return nil, err
		}

		partLength := int(lenByte[0])
		if partLength == 0 {
			break
		}

		namePart := make([]byte, partLength)
		_, err = d.buffer.Read(namePart)
		if err != nil {
			return nil, err
		}

		nameParts = append(nameParts, namePart)
	}

	name := bytes.Join(nameParts, []byte("."))

	question := &DNSQuestion{}
	question.domainName = name

	buf := make([]byte, 2)

	// Read record type
	_, err := d.buffer.Read(buf)
	if err != nil {
		return nil, err
	}
	question.recordType = binary.BigEndian.Uint16(buf)

	// Read record class
	_, err = d.buffer.Read(buf)
	if err != nil {
		return nil, err
	}
	question.recordClass = binary.BigEndian.Uint16(buf)

	return question, nil
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

	parser := NewParser(response)

	_, err = parser.parseHeader()
	if err != nil {
		log.Fatal("Failed to parse response header:", err)
	}

	_, err = parser.parseQuestion()
	if err != nil {
		log.Fatal("Failed to parse question:", err)
	}
}
