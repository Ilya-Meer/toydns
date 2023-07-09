package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net"
	"strings"
)

const (
	typeARecord = uint16(1)
	classIN     = uint16(1)

	lenRecordType = int(2)
	lenClass      = int(2)
)

var (
	errParsingDomainName = errors.New("domain name parsing failed")
	errParsingQuestion   = errors.New("question parsing failed")
	errParsingRecord     = errors.New("record parsing failed")
	errInsufficientData  = errors.New("insufficient data to fill buffer")
)

type DNSHeader struct {
	QueryID        uint16
	Flags          uint16
	NumQuestions   uint16
	NumAnswers     uint16
	NumAuthorities uint16
	NumAdditionals uint16
}

type DNSRecord struct {
	Name       []byte
	RecordType uint16
	Class      uint16
	Ttl        uint32
	DataLength uint16
	Data       []byte
}

type DNSQuestion struct {
	domainName  []byte
	recordType  uint16
	recordClass uint16
}

func buildQuery(domainName string, recordType uint16) []byte {
	queryId := rand.Intn(1 << 16)
	recursionDesiredFlag := 1 << 8

	header := DNSHeader{
		QueryID:      uint16(queryId),
		Flags:        uint16(recursionDesiredFlag),
		NumQuestions: 1,
	}

	name := encodeDomain(domainName)

	questionLength := len(name) + lenRecordType + lenClass
	querySize := binary.Size(header) + questionLength
	queryBuf := bytes.NewBuffer(make([]byte, 0, querySize))

	// Encode header
	writeBinary(queryBuf, &header, "header")

	// Encode question
	writeBinary(queryBuf, name, "name")
	writeBinary(queryBuf, recordType, "recordType")
	writeBinary(queryBuf, classIN, "class")

	return queryBuf.Bytes()
}

func writeBinary(buf *bytes.Buffer, data interface{}, datum string) {
	err := binary.Write(buf, binary.BigEndian, data)
	if err != nil {
		log.Fatalf("Failed to write %s: %s", datum, err.Error())
	}
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
	bytes  []byte
	offset int
}

func NewParser(response []byte) *DNSResponseParser {
	return &DNSResponseParser{response, 0}
}

func (d *DNSResponseParser) parseHeader() (*DNSHeader, error) {
	header := &DNSHeader{}
	headerSize := binary.Size(header)

	buf := make([]byte, headerSize)
	_, err := d.read(buf)
	if err != nil {
		return nil, err
	}

	err = binary.Read(bytes.NewReader(buf), binary.BigEndian, header)
	if err != nil {
		return nil, err
	}

	return header, nil
}

func (d *DNSResponseParser) parseQuestion() (*DNSQuestion, error) {
	name, err := d.parseDomainName()
	if err != nil {
		return nil, err
	}

	question := &DNSQuestion{}

	question.domainName = name

	// Read record type
	if err = d.readUint16(&question.recordType); err != nil {
		return nil, err
	}

	// Read record class
	if err = d.readUint16(&question.recordClass); err != nil {
		return nil, err
	}

	return question, nil
}

func (d *DNSResponseParser) parseDomainName() ([]byte, error) {
	nameParts := make([][]byte, 0)

	for {
		lenByte := make([]byte, 1)
		n := copy(lenByte, d.bytes[d.offset:d.offset+1])
		if n < len(lenByte) {
			return nil, errParsingDomainName
		}
		d.offset += n

		if lenByte[0]&0b11000000 == 0b11000000 {
			pointer := d.bytes[d.offset]

			name, err := d.parseCompressedName(pointer)
			if err != nil {
				return nil, err
			}

			nameParts = append(nameParts, name)
			d.offset += 1
			break
		}

		partLength := int(lenByte[0])
		if partLength == 0 {
			break
		}

		namePart := make([]byte, partLength)
		n = copy(namePart, d.bytes[d.offset:d.offset+len(namePart)])
		if n < len(namePart) {
			return nil, errParsingDomainName
		}
		d.offset += n

		nameParts = append(nameParts, namePart)
	}

	name := bytes.Join(nameParts, []byte("."))

	return name, nil
}

func (d *DNSResponseParser) parseCompressedName(pointer byte) ([]byte, error) {
	// save current offset
	currOffset := d.offset

	// set offset to pointer byte
	d.offset = int(pointer)

	// parse domain name
	name, err := d.parseDomainName()
	if err != nil {
		return nil, err
	}

	// restore offset
	d.offset = currOffset

	return name, nil
}

func (d *DNSResponseParser) parseRecord() (*DNSRecord, error) {
	record := &DNSRecord{}

	name, err := d.parseDomainName()
	if err != nil {
		return nil, err
	}

	record.Name = name

	// Read record type
	if err = d.readUint16(&record.RecordType); err != nil {
		return nil, err
	}

	// Read record type
	if err = d.readUint16(&record.Class); err != nil {
		return nil, err
	}

	// Read TTL
	if err = d.readUint32(&record.Ttl); err != nil {
		return nil, err
	}

	// Read data length
	if err = d.readUint16(&record.DataLength); err != nil {
		return nil, err
	}

	// Read data
	record.Data = make([]byte, record.DataLength)
	_, err = d.read(record.Data)
	if err != nil {
		return nil, errParsingRecord
	}

	return record, nil
}

func (d *DNSResponseParser) readUint16(val *uint16) error {
	buf := make([]byte, 2)

	_, err := d.read(buf)
	if err != nil {
		return errParsingRecord
	}

	*val = binary.BigEndian.Uint16(buf)

	return nil
}

func (d *DNSResponseParser) readUint32(val *uint32) error {
	buf := make([]byte, 4)

	_, err := d.read(buf)
	if err != nil {
		return errParsingRecord
	}

	*val = binary.BigEndian.Uint32(buf)

	return nil
}

func (d *DNSResponseParser) read(buf []byte) (int, error) {
	n := copy(buf, d.bytes[d.offset:])
	if n < len(buf) {
		return n, errInsufficientData
	}
	d.offset += n

	return n, nil
}

func main() {
	query := buildQuery("www.google.com", typeARecord)

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

	record, err := parser.parseRecord()
	if err != nil {
		log.Fatal("Failed to parse DNS record:", err)
	}

	fmt.Printf("IP address %d", record.Data)
}
