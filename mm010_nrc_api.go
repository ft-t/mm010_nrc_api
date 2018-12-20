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
	RequestStart          byte = 0x04
	ResponseStart         byte = 0x01
	CommunicationIdentify byte = 0x30
	TextStart             byte = 0x02
	TextEnd               byte = 0x03
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

type DataItem uint16

const (
	ProgramID                        DataItem = 100
	MachineID                        DataItem = 101
	MaxNumberOfNotesInOneTransaction DataItem = 104
	Baudrate                         DataItem = 115
	Parity                           DataItem = 116
	DispenseCounterLifelong          DataItem = 303
	RejectCounterLifelong            DataItem = 304
	TotalProcessedCounterLifelong    DataItem = 305
	DispenseCounterTrip              DataItem = 306
	RejectCounterTrip                DataItem = 307
	TotalProcessedCcounterTrip       DataItem = 308
	TransactionCounterLifelong       DataItem = 313
	TransactionCounterTrip           DataItem = 314
	ThroatSensorCalibrationValue     DataItem = 350
	LearningNotes                    DataItem = 392
	RejectReasonCounter              DataItem = 501
	ErrorStatusCounter               DataItem = 502
	MachineStatus                    DataItem = 503
)

type MMDispenser struct {
	config  *serial.Config
	port    *serial.Port
	logging bool
	open    bool
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
	c := &serial.Config{Name: path, Baud: int(baud), ReadTimeout: 5 * time.Second, Parity: serial.ParityEven, StopBits: serial.Stop1,
		Size: 7}

	o, err := serial.OpenPort(c)

	res := MMDispenser{}

	if err != nil {
		return res, err
	}

	res.config = c
	res.port = o
	res.logging = logging
	res.open = true

	return res, nil
}

func (s *MMDispenser) Open() error {
	if s.open {
		return errors.New("port already opened")
	}

	p, err := serial.OpenPort(s.config)

	if err != nil {
		return err
	}

	s.port = p
	s.open = true

	return nil
}

func (s *MMDispenser) Close() error {
	if s.port == nil || !s.open {
		return errors.New("port not opened")
	}

	err := s.port.Close()
	s.open = false

	return err
}

func (s *MMDispenser) Status() (Status, error) {
	status := Status{}
	err := sendRequest(s, 0x40, []byte{})

	if err != nil {
		return status, err
	}

	response, err := readResponse(s)

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

func (s *MMDispenser) Purge() (StatusCode, byte, error) {
	err := sendRequest(s, 0x41, []byte{})

	if err != nil {
		return 0, 0, err
	}

	response, err := readResponse(s)

	if err != nil {
		return 0, 0, err
	}

	return StatusCode(response[0]), response[1] - 0x20, nil
}

func (s *MMDispenser) Dispense(count byte) (StatusCode, byte, byte, error) {
	err := sendRequest(s, 0x42, []byte{count + 0x20})

	if err != nil {
		return 0, 0, 0, err
	}

	response, err := readResponse(s)

	if err != nil {
		return 0, 0, 0, err
	}

	return StatusCode(response[0]), response[1] - 0x20, response[2] - 0x20, nil
}

func (s *MMDispenser) TestDispense(count byte) (StatusCode, byte, byte, error) {
	err := sendRequest(s, 0x43, []byte{count + 0x20})

	if err != nil {
		return 0, 0, 0, err
	}

	response, err := readResponse(s)

	if err != nil {
		return 0, 0, 0, err
	}

	return StatusCode(response[0]), response[1] - 0x20, response[2] - 0x20, nil
}

func (s *MMDispenser) Reset() error {
	err := sendRequest(s, 0x44, []byte{})

	if err != nil {
		return err
	}

	_, err = readRespCode(s)
	return err
}

func (s *MMDispenser) LastStatus() (StatusCode, byte, byte, error) {
	err := sendRequest(s, 0x45, []byte{})

	if err != nil {
		return 0, 0, 0, err
	}

	response, err := readResponse(s)

	if err != nil {
		return 0, 0, 0, err
	}

	return StatusCode(response[0]), response[1] - 0x20, response[2] - 0x20, nil
}

func (s *MMDispenser) ConfigurationStatus() (byte, byte, error) {
	err := sendRequest(s, 0x46, []byte{})

	if err != nil {
		return 0, 0, err
	}

	response, err := readResponse(s)

	if err != nil {
		return 0, 0, err
	}

	return response[0] - 0x20, response[1] - 0x20, nil
}

func (s *MMDispenser) DoubleDetectDiagnostics() (StatusCode, byte, byte, error) {
	err := sendRequest(s, 0x47, []byte{})

	if err != nil {
		return 0, 0, 0, err
	}

	response, err := readResponse(s)

	if err != nil {
		return 0, 0, 0, err
	}

	return StatusCode(response[0]), response[1] - 0x20, response[2] - 0x20, nil
}

func (s *MMDispenser) SensorDiagnostics() (StatusCode, byte, byte, error) {
	err := sendRequest(s, 0x48, []byte{})

	if err != nil {
		return 0, 0, 0, err
	}

	response, err := readResponse(s)

	if err != nil {
		return 0, 0, 0, err
	}

	return StatusCode(response[0]), response[1] - 0x20, response[2] - 0x20, nil
}

func (s *MMDispenser) SingleNoteDispense() (StatusCode, byte, byte, error) {
	err := sendRequest(s, 0x4A, []byte{})

	if err != nil {
		return 0, 0, 0, err
	}

	response, err := readResponse(s)

	if err != nil {
		return 0, 0, 0, err
	}

	return StatusCode(response[0]), response[1] - 0x20, response[2] - 0x20, nil
}

func (s *MMDispenser) SingleNoteEject() (StatusCode, byte, byte, error) {
	err := sendRequest(s, 0x4B, []byte{})

	if err != nil {
		return 0, 0, 0, err
	}

	response, err := readResponse(s)

	if err != nil {
		return 0, 0, 0, err
	}

	return StatusCode(response[0]), response[1] - 0x20, response[2] - 0x20, nil
}

func (s *MMDispenser) TestMode() (StatusCode, error) {
	err := sendRequest(s, 0x54, []byte{})

	if err != nil {
		return 0, err
	}

	response, err := readResponse(s)

	if err != nil {
		return 0, err
	}

	return StatusCode(response[0]), nil
}

func (s *MMDispenser) ReadData(item DataItem, param string) (string, error) {
	str := fmt.Sprintf("D/%3d", item)

	if len(param) > 0 {
		str += fmt.Sprintf("/%s", param)
	}

	sendRequest(s, 0x52, []byte(str))

	response, err := readResponse(s)

	if err != nil {
		return "", err
	}

	if response[0] != 0x30 {
		return "", errors.New("illegal command")
	}

	return string(response[1:]), nil
}

func (s *MMDispenser) WriteData(item DataItem, data string) error {
	err := sendRequest(s, 0x57, []byte(fmt.Sprintf("D/%3d/%s", item, data)))

	if err != nil {
		return err
	}

	response, err := readResponse(s)

	if err != nil {
		return err
	}

	if response[0] != 0x30 {
		return errors.New("illegal command")
	}

	return nil
}

func (s *MMDispenser) Ack() {
	_, _ = s.port.Write([]byte{0x06})
}

func (s *MMDispenser) Nack() {
	_, _ = s.port.Write([]byte{0x15})
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

	time.Sleep(time.Millisecond * 200)

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
			fmt.Printf("mm010_nrc[%v]: <- ACK\n", v.config.Name)
		}
		return AckResponse, nil // TODO Ack
	}

	if buf[0] == 0x15 {
		if v.logging {
			fmt.Printf("mm010_nrc[%v]: <- NAK\n", v.config.Name)
		}
		return NackResponse, nil
	}

	if buf[0] == 0x04 {
		if v.logging {
			fmt.Printf("mm010_nrc[%v]: <- EOT\n", v.config.Name)
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

	lastRead := false

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

		if len(buf) > 2 && buf[len(buf)-2] == TextEnd {
			lastRead = true
		}

		if lastRead == false {
			continue
		}

		break
	}

	if buf[0] != ResponseStart || buf[1] != CommunicationIdentify {
		fmt.Printf("mm010_nrc[%v]: <- %X\n", v.config.Name, buf)
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
		fmt.Printf("mm010_nrc[%v]: <- %X\n", v.config.Name, buf)
	}

	return buf, nil
}

func sendRequest(v *MMDispenser, commandCode byte, bytesData ...[]byte) error {
	if !v.open {
		return errors.New("serial port is closed")
	}

	buf := new(bytes.Buffer)

	length := 6

	for _, b := range bytesData {
		length += len(b)
	}

	_ = binary.Write(buf, binary.LittleEndian, RequestStart)
	_ = binary.Write(buf, binary.LittleEndian, CommunicationIdentify)
	_ = binary.Write(buf, binary.LittleEndian, TextStart)
	_ = binary.Write(buf, binary.LittleEndian, commandCode)

	for _, data := range bytesData {
		_ = binary.Write(buf, binary.LittleEndian, data)
	}

	_ = binary.Write(buf, binary.LittleEndian, TextEnd)

	crc := getChecksum(buf.Bytes())

	_ = binary.Write(buf, binary.LittleEndian, crc)

	if v.logging {
		fmt.Printf("mm010_nrc[%v]: -> %X\n", v.config.Name, buf.Bytes())
	}

	_, err := v.port.Write(buf.Bytes())

	return err
}

func getChecksum(data []byte) byte {
	chksum := byte(0)

	for _, b := range data {
		chksum = chksum ^ b
	}

	return chksum
}
