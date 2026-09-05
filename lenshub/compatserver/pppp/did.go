package pppp

import (
	"encoding/binary"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

const DIDWireSize = 20

var ErrInvalidDID = errors.New("invalid pppp DID")

type DID struct {
	Prefix string
	Serial uint64
	Suffix string
}

func ParseDID(s string) (DID, error) {
	parts := strings.Split(strings.TrimSpace(s), "-")
	if len(parts) != 3 {
		return DID{}, fmt.Errorf("%w: expected PREFIX-SERIAL-SUFFIX", ErrInvalidDID)
	}
	prefix := strings.ToUpper(parts[0])
	suffix := strings.ToUpper(parts[2])
	if len(prefix) != 4 || !asciiAlphaNum(prefix) {
		return DID{}, fmt.Errorf("%w: prefix must be 4 ASCII alphanumeric bytes", ErrInvalidDID)
	}
	if len(suffix) == 0 || len(suffix) > 8 || !asciiAlphaNum(suffix) {
		return DID{}, fmt.Errorf("%w: suffix must be 1..8 ASCII alphanumeric bytes", ErrInvalidDID)
	}
	if len(parts[1]) == 0 || len(parts[1]) > 20 {
		return DID{}, fmt.Errorf("%w: serial", ErrInvalidDID)
	}
	serial, err := strconv.ParseUint(parts[1], 10, 64)
	if err != nil {
		return DID{}, fmt.Errorf("%w: serial: %v", ErrInvalidDID, err)
	}
	return DID{Prefix: prefix, Serial: serial, Suffix: suffix}, nil
}

func ParseDIDWire(data []byte) (DID, error) {
	if len(data) < DIDWireSize {
		return DID{}, fmt.Errorf("%w: wire length %d", ErrInvalidDID, len(data))
	}
	prefix := strings.TrimRight(string(data[:4]), "\x00")
	suffix := strings.TrimRight(string(data[12:20]), "\x00")
	if len(prefix) != 4 || !asciiAlphaNum(prefix) || len(suffix) == 0 || !asciiAlphaNum(suffix) {
		return DID{}, fmt.Errorf("%w: malformed wire identifier", ErrInvalidDID)
	}
	return DID{Prefix: prefix, Serial: binary.BigEndian.Uint64(data[4:12]), Suffix: suffix}, nil
}

func (d DID) Wire20() ([DIDWireSize]byte, error) {
	var out [DIDWireSize]byte
	if len(d.Prefix) != 4 || !asciiAlphaNum(d.Prefix) || len(d.Suffix) == 0 || len(d.Suffix) > 8 || !asciiAlphaNum(d.Suffix) {
		return out, ErrInvalidDID
	}
	copy(out[:4], strings.ToUpper(d.Prefix))
	binary.BigEndian.PutUint64(out[4:12], d.Serial)
	copy(out[12:20], strings.ToUpper(d.Suffix))
	return out, nil
}

func (d DID) String() string {
	return fmt.Sprintf("%s-%06d-%s", strings.ToUpper(d.Prefix), d.Serial, strings.ToUpper(d.Suffix))
}

func asciiAlphaNum(s string) bool {
	for _, r := range s {
		if r > unicode.MaxASCII || !((r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')) {
			return false
		}
	}
	return s != ""
}
