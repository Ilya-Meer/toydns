package main

import (
	"encoding/hex"
	"testing"
)

func TestBuildQuery(t *testing.T) {
	sampleQueryHex := "82980100000100000000000003777777076578616d706c6503636f6d0000010001"

	// bytes structured as follows:
	_ = []byte{
		0x82, 0x98, // queryID
		0x01, 0x00, // flags
		0x00, 0x01, // numQuestions
		0x00, 0x00, // numAnswers
		0x00, 0x00, // numAuthorities
		0x00, 0x00, // numAdditionals
		0x03, 0x77, 0x77, 0x77, // name: www
		0x07, 0x65, 0x78, 0x61, 0x6d, 0x70, 0x6c, 0x65, // name: example
		0x03, 0x63, 0x6f, 0x6d, // name: com
		0x00,       // name: end
		0x00, 0x01, // recordType: A
		0x00, 0x00, // class: IN
	}

	sampleQueryBytes, err := hex.DecodeString(sampleQueryHex)
	if err != nil {
		t.Fatal("Failed to decode hex string:", err)
	}

	host := "www.example.com"
	query := buildQuery(host, typeARecord)

	offsetAfterRandomId := 4

	for i := offsetAfterRandomId; i < len(query); i++ {
		if query[i] != sampleQueryBytes[i] {
			t.Fatalf("DNS query doesn't match expected output: expected %d, but got %d", sampleQueryBytes[i], query[i])
		}
	}
}

func TestParseHeader(t *testing.T) {
	sampleDNSResponse := []byte{134, 253, 129, 128, 0, 1, 0, 1, 0, 0, 0, 0, 3, 119, 119, 119, 7, 101, 120, 97, 109, 112, 108, 101, 3, 99, 111, 109, 0, 0, 1, 0, 1, 192, 12, 0, 1, 0, 1, 0, 0, 80, 205, 0, 4, 93, 184, 216, 34}

	p := NewParser(sampleDNSResponse)
	header, err := p.parseHeader()
	if err != nil {
		t.Fatal("Failed to parse DNS header:", err)
	}

	if header.Flags != 0x8180 {
		t.Fatalf("expected %d but got %d", 0x8180, header.Flags)
	}

	if header.NumQuestions != 0x1 {
		t.Fatalf("expected %d but got %d", 0x1, header.NumQuestions)
	}
}
