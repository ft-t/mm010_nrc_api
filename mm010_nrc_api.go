package mm010_nrc_api

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"time"

	"github.com/tarm/serial"
)

const (
	RequestStart          = 0x04
	ResponseStart         = 0x01
	CommunicationIdentify = 0x30
	TextStart             = 0x02
	TextEnd               = 0x03
)

type Baud int

const (
	Baud1200 Baud = 1200
	Baud2400 Baud = 2400
	Baud4800 Baud = 4800
	Baud9600 Baud = 9600
)

type ResponseType byte

const (
	ErrorResponse ResponseType = 0x00
	AckResponse   ResponseType = 0x06
	NackResponse  ResponseType = 0x15
	EotResponse   ResponseType = 0x04
)

type StatusCode byte

const (
	GoodOperation        StatusCode = 0x20
	FeedFailure          StatusCode = 0x21
	MistrackedNoteAtExit StatusCode = 0x24
	TooLongAtExit        StatusCode = 0x25
	BlockedExit          StatusCode = 0x26
	TransportError       StatusCode = 0x2A
	DoubleDetectError    StatusCode = 0x2C
	DivertedError        StatusCode = 0x2D
	WrongCount           StatusCode = 0x2E
	NoteMissingAtDD      StatusCode = 0x2F
	RejectRateExceeded   StatusCode = 0x30
	NonVolatileRAMError  StatusCode = 0x34
	OperationTimeout     StatusCode = 0x36
	InternalQueError     StatusCode = 0x37
	InvalidCommand       StatusCode = 0x4F
)

type MMDispenser struct {
	config  *serial.Config
	port    *serial.Port
	logging bool
}

type Status struct {
	FeedSensorBlocked           bool
	ExitSensorBlocked           bool
	ResetSinceLastStatusMessage bool
	TimingWheelSensorBlocked    bool
	CalibratingDoubleDetect     bool
	AverageThickness            byte
	AverageLength               byte
}

func NewConnection(path string, baud Baud, logging bool) (MMDispenser, error) {
	c := &serial.Config{Name: path, Baud: int(baud), ReadTimeout: 5 * time.Second} // TODO
	o, err := serial.OpenPort(c)

	res := MMDispenser{}

	if err != nil {
		return res, err
	}

	res.config = c
	res.port = o
	res.logging = logging

	return res, nil
}

func (s *MMDispenser) Status() (Status, error) {
	sendRequest(s, 0x40, []byte{})

	response, err := readResponse(s)

	status := Status{}

	if err != nil {
		return status, err
	}

	status.FeedSensorBlocked = (response[0] & (1 << 0)) != 0
	status.ExitSensorBlocked = (response[0] & (1 << 1)) != 0
	status.ResetSinceLastStatusMessage = (response[0] & (1 << 3)) != 0
	status.TimingWheelSensorBlocked = (response[0] & (1 << 4)) != 0
	status.CalibratingDoubleDetect = (response[1] & (1 << 4)) != 0
	status.AverageThickness = response[2] - 0x20
	status.AverageLength = response[3] - 0x20

	return status, err
}

func (s *MMDispenser) Reset() {
	sendRequest(s, 0x44, []byte{})
}

func (s *MMDispenser) Purge() (StatusCode, byte, error) {
	sendRequest(s, 0x41, []byte{})

	response, err := readResponse(s)

	if err != nil {
		return 0, 0, err
	}

	return StatusCode(response[0]), response[1] - 0x20, nil
}

func (s *MMDispenser) Dispense(count byte) (StatusCode, byte, byte, error) {
	sendRequest(s, 0x42, []byte{count + 0x20})

	response, err := readResponse(s)

	if err != nil {
		return 0, 0, 0, err
	}

	return StatusCode(response[0]), response[1] - 0x20, response[2] - 0x20, nil
}

func (s *MMDispenser) TestMode() (StatusCode, error) {
	sendRequest(s, 0x54, []byte{})

	response, err := readResponse(s)

	if err != nil {
		return 0, err
	}

	return StatusCode(response[0]), nil
}

func (s *MMDispenser) Ack() {
	s.port.Write([]byte{0x06})
}

func (s *MMDispenser) Nack() {
	s.port.Write([]byte{0x15})
}

func readResponse(v *MMDispenser) ([]byte, error) {
	resp, err := readRespCode(v)

	if err != nil {
		return nil, err
	}

	if resp != AckResponse {
		return nil, errors.New("Response not ACK")
	}

	data, err := readRespData(v)

	if err != nil {
		return nil, err
	}

	v.Ack()

	resp, err = readRespCode(v)

	if err != nil {
		return nil, err
	}

	if resp != EotResponse {
		return nil, errors.New("Response not EOT")
	}

	return data, nil
}

func readRespCode(v *MMDispenser) (ResponseType, error) {
	var buf []byte
	innerBuf := make([]byte, 256)

	totalRead := 0
	readTriesCount := 0
	maxReadCount := 1050

	for ; ; {
		readTriesCount += 1

		if readTriesCount >= maxReadCount {
			return ErrorResponse, fmt.Errorf("Reads tries exceeded")
		}

		n, err := v.port.Read(innerBuf)

		if err != nil {
			return ErrorResponse, err
		}

		totalRead += n
		buf = append(buf, innerBuf[:n]...)

		if totalRead < 1 {
			continue
		}
		break
	}

	if buf[0] == 0x06 {
		if v.logging {
			fmt.Printf("<- ACK\n")
		}
		return AckResponse, nil // TODO Ack
	}

	if buf[0] == 0x15 {
		if v.logging {
			fmt.Printf("<- NAK\n")
		}
		return NackResponse, nil
	}

	if buf[0] == 0x04 {
		if v.logging {
			fmt.Printf("<- EOT\n")
		}
		return EotResponse, nil
	}

	return ErrorResponse, nil
}

func readRespData(v *MMDispenser) ([]byte, error) {
	var buf []byte
	innerBuf := make([]byte, 256)

	totalRead := 0
	readTriesCount := 0
	maxReadCount := 1050

	for ; ; {
		readTriesCount += 1

		if readTriesCount >= maxReadCount {
			return nil, fmt.Errorf("Reads tries exceeded")
		}

		n, err := v.port.Read(innerBuf)

		if err != nil {
			return nil, err
		}

		totalRead += n
		buf = append(buf, innerBuf[:n]...)

		if totalRead < 6 {
			continue
		}

		break
	}

	if buf[0] != ResponseStart || buf[1] != CommunicationIdentify {
		return nil, fmt.Errorf("Response format invalid")
	}

	crc := buf[len(buf)-1]

	buf = buf[:len(buf)-1]

	crc2 := getChecksum(buf)

	if crc != crc2 {
		return nil, fmt.Errorf("Response verification failed")
	}

	if buf[2] != TextStart || buf[len(buf)-1] != TextEnd {
		return nil, fmt.Errorf("Response format invalid")
	}

	buf = buf[4 : len(buf)-1]

	if v.logging {
		fmt.Printf("<- %X\n", buf)
	}

	return buf, nil
}

func sendRequest(v *MMDispenser, commandCode byte, bytesData ...[]byte) {
	buf := new(bytes.Buffer)

	length := 6

	for _, b := range bytesData {
		length += len(b)
	}

	binary.Write(buf, binary.LittleEndian, RequestStart)
	binary.Write(buf, binary.LittleEndian, CommunicationIdentify)
	binary.Write(buf, binary.LittleEndian, TextStart)
	binary.Write(buf, binary.LittleEndian, commandCode)

	for _, data := range bytesData {
		binary.Write(buf, binary.LittleEndian, data)
	}

	binary.Write(buf, binary.LittleEndian, TextEnd)

	crc := getChecksum(buf.Bytes())

	binary.Write(buf, binary.LittleEndian, crc)

	if v.logging {
		fmt.Printf("-> %X\n", buf.Bytes())
	}

	v.port.Write(buf.Bytes())
}

func getChecksum(data []byte) byte {
	chksum := byte(0)

	for _, b := range data {
		chksum = chksum ^ b
	}

	return chksum
}
