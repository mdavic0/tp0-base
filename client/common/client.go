package common

import (
	"bufio"
	"bytes"
	"context"
	"crypto/md5"
	"encoding/binary"
	"encoding/csv"
	"encoding/hex"
	"io"
	"net"
	"os"
	"strings"
	"time"

	"github.com/op/go-logging"
)

var log = logging.MustGetLogger("log")

// MessageTypes
const (
	MessageTypeBet             = 1
	MessageTypeACK             = 2
	MessageTypeBatch           = 3
	MessageTypeFinished        = 4
	MessageTypeAskForWinners   = 5
	MessageTypeWinnersResponse = 6
)

// ClientConfig Configuration used by the client
type ClientConfig struct {
	ID            string
	ServerAddress string
	LoopAmount    int
	LoopPeriod    time.Duration
	BatchBets     int
}

// Client Entity that encapsulates how
type Client struct {
	config ClientConfig
	conn   net.Conn
}

// NewClient Initializes a new client receiving the configuration
// as a parameter
func NewClient(config ClientConfig) *Client {
	client := &Client{
		config: config,
	}
	return client
}

// CreateClientSocket Initializes client socket. In case of
// failure, error is printed in stdout/stderr and exit 1
// is returned
func (c *Client) createClientSocket() error {
	conn, err := net.Dial("tcp", c.config.ServerAddress)
	if err != nil {
		log.Criticalf("action: connect | result: fail | client_id: %v | error: %v", c.config.ID, err)
		return err
	}
	c.conn = conn
	return nil
}

// generateMessageID creates a 16-byte ID using MD5 of the payload -> uuid ;)
func generateMessageID(payload string) ([]byte, string) {
	hash := md5.Sum([]byte(payload))
	return hash[:], hex.EncodeToString(hash[:])
}

// readExactly reads n bytes, handling short-reads
func readExactly(r io.Reader, n int) ([]byte, error) {
	buf := make([]byte, n)
	total := 0
	for total < n {
		nRead, err := r.Read(buf[total:])
		if err != nil {
			return nil, err
		}
		total += nRead
	}
	return buf, nil
}

func writeExactly(conn net.Conn, data []byte) error {
	total := 0
	for total < len(data) {
		n, err := conn.Write(data[total:])
		if err != nil {
			return err
		}
		total += n
	}
	return nil
}

// readBatch reads up to `max` bets from CSV
func readBatch(reader *csv.Reader, max int) ([]Bet, error) {
	var bets []Bet
	for len(bets) < max {
		record, err := reader.Read()
		if err == io.EOF {
			break // salgo del for, puede haber batch incompleto
		}
		if err != nil || len(record) < 5 {
			log.Warningf("action: read_csv | result: skip | reason: parse_error | line: %v | error: %v", record, err)
			continue
		}
		bets = append(bets, Bet{
			Nombre:     record[0],
			Apellido:   record[1],
			DNI:        record[2],
			Nacimiento: record[3],
			Numero:     record[4],
		})
	}
	return bets, nil
}

// sendBatch serializes and sends the batch to the server
func (c *Client) sendBatch(bets []Bet) ([]byte, string, error) {
	if err := c.createClientSocket(); err != nil {
		return nil, "", err
	}

	var payloadParts []string
	for _, bet := range bets {
		payloadParts = append(payloadParts, bet.Serialize(c.config.ID))
	}
	payload := strings.Join(payloadParts, "|")

	msgIDBytes, msgIDHex := generateMessageID(payload)
	body := []byte(payload)
	msgLen := uint32(len(body) + 2 + 16)

	buf := new(bytes.Buffer)
	binary.Write(buf, binary.BigEndian, msgLen)
	binary.Write(buf, binary.BigEndian, uint16(MessageTypeBatch))
	buf.Write(msgIDBytes)
	buf.Write(body)

	if err := writeExactly(c.conn, buf.Bytes()); err != nil {
		c.conn.Close()
		return nil, "", err
	}

	log.Infof("action: batch_enviado | result: in_progress | id: %s | cantidad: %d", msgIDHex, len(bets))
	return msgIDBytes, msgIDHex, nil
}

// receiveACK waits for the ACK and validates it
func (c *Client) receiveACK(expectedID []byte, expectedHex string) (string, error) {
	reader := bufio.NewReader(c.conn)

	lenBytes, err := readExactly(reader, 4)
	if err != nil {
		return "", err
	}
	totalLen := binary.BigEndian.Uint32(lenBytes)

	typeBytes, err := readExactly(reader, 2)
	if err != nil {
		return "", err
	}
	msgType := binary.BigEndian.Uint16(typeBytes)
	if msgType != MessageTypeACK {
		log.Errorf("action: receive_ack | result: fail | error: unexpected_type | expected: %d got: %d", MessageTypeACK, msgType)
		return "", err
	}

	ackID, err := readExactly(reader, 16)
	if err != nil {
		return "", err
	}
	if !bytes.Equal(ackID, expectedID) {
		log.Errorf("action: receive_ack | result: fail | error: mismatched_id | expected: %s, got: %s",
			expectedHex, hex.EncodeToString(ackID))
		return "", err
	}

	payloadLen := int(totalLen) - 2 - 16
	payload, err := readExactly(reader, payloadLen)
	if err != nil {
		return "", err
	}

	ackStr := string(payload)
	log.Infof("action: receive_ack | result: success | id: %s | payload: %s", hex.EncodeToString(ackID), ackStr)
	return ackStr, nil
}

func (c *Client) sendFinishedNotification() error {
	payload := "{agency:" + c.config.ID + "}"
	msgIDBytes, msgIDHex := generateMessageID(payload)
	body := []byte(payload)
	msgLen := uint32(len(body) + 2 + 16)

	buf := new(bytes.Buffer)
	binary.Write(buf, binary.BigEndian, msgLen)
	binary.Write(buf, binary.BigEndian, uint16(MessageTypeFinished))
	buf.Write(msgIDBytes)
	buf.Write(body)

	if err := c.createClientSocket(); err != nil {
		return err
	}

	err := writeExactly(c.conn, buf.Bytes())
	if err != nil {
		log.Errorf("action: notify_finished | result: fail | error: %v", err)
		c.conn.Close()
		return err
	}
	log.Infof("action: notify_finished | result: success | id: %s", msgIDHex)

	_, err = c.receiveACK(msgIDBytes, msgIDHex)
	c.conn.Close()
	return err
}

func (c *Client) tryAskForWinners() error {
	payload := "{agency:" + c.config.ID + "}"
	msgIDBytes, msgIDHex := generateMessageID(payload)
	body := []byte(payload)
	msgLen := uint32(len(body) + 2 + 16)

	buf := new(bytes.Buffer)
	binary.Write(buf, binary.BigEndian, msgLen)
	binary.Write(buf, binary.BigEndian, uint16(MessageTypeAskForWinners))
	buf.Write(msgIDBytes)
	buf.Write(body)

	if err := c.createClientSocket(); err != nil {
		return err
	}

	err := writeExactly(c.conn, buf.Bytes())
	if err != nil {
		log.Errorf("action: consulta_ganadores | result: fail | error: %v", err)
		c.conn.Close()
		return err
	}

	reader := bufio.NewReader(c.conn)

	lenBytes, err := readExactly(reader, 4)
	if err != nil {
		return err
	}
	totalLen := binary.BigEndian.Uint32(lenBytes)

	typeBytes, err := readExactly(reader, 2)
	if err != nil {
		return err
	}
	msgType := binary.BigEndian.Uint16(typeBytes)
	if msgType != MessageTypeWinnersResponse {
		log.Errorf("action: consulta_ganadores | result: fail | error: unexpected_type | got: %d", msgType)
		return err
	}

	ackID, err := readExactly(reader, 16)
	if err != nil {
		return err
	}
	if !bytes.Equal(ackID, msgIDBytes) {
		log.Errorf("action: consulta_ganadores | result: fail | error: mismatched_id | expected: %s, got: %s", msgIDHex, hex.EncodeToString(ackID))
		return err
	}

	payloadLen := int(totalLen) - 2 - 16
	payloadResp, err := readExactly(reader, payloadLen)
	c.conn.Close()
	if err != nil {
		return err
	}

	payloadStr := string(payloadResp)
	payloadStr = strings.Trim(payloadStr, "{}")
	parts := strings.Split(payloadStr, ":")

	// Caso sorteo pendiente
	if parts[0] == "result" && parts[1] == "in_progress" {
		return io.EOF // mando error para reintentar
	}

	// Caso correcto
	if parts[0] != "ganadores" {
		log.Errorf("action: consulta_ganadores | result: fail | reason: unexpected_format | content: %s", payloadStr)
		return nil
	}

	count := 0
	if parts[1] != "" {
		count = len(strings.Split(parts[1], "|"))
	}

	log.Infof("action: consulta_ganadores | result: success | cant_ganadores: %d", count)
	return nil
}

func (c *Client) askForWinners() error {
	maxRetries := 10
	for attempt := 1; attempt <= maxRetries; attempt++ {
		err := c.tryAskForWinners()
		if err == nil {
			return nil
		}
		log.Infof("action: consulta_ganadores | result: in_progress | intento: %d/%d", attempt, maxRetries)
		time.Sleep(c.config.LoopPeriod)
	}
	log.Errorf("action: consulta_ganadores | result: fail | reason: max_retries_exceeded")
	return nil
}

func (c *Client) StartClientLoop(ctx context.Context) {
	file, err := os.Open("/data/agency.csv")
	if err != nil {
		log.Errorf("action: open_csv | result: fail | error: %v", err)
		return
	}
	defer file.Close()

	reader := csv.NewReader(file)

	for {
		select {
		case <-ctx.Done():
			log.Infof("action: exit | result: success | client_id: %v | reason: signal", c.config.ID)
			return
		default:
			bets, err := readBatch(reader, c.config.BatchBets)
			if err != nil {
				log.Errorf("action: read_batch | result: fail | error: %v", err)
				return
			}

			if len(bets) == 0 {
				// no hay más apuestas que enviar
				if err := c.sendFinishedNotification(); err != nil {
					log.Errorf("action: notify_finished | result: fail | error: %v", err)
					return
				}

				if err := c.askForWinners(); err != nil {
					log.Errorf("action: consulta_ganadores | result: fail | error: %v", err)
				}
				return
			}

			msgID, msgIDHex, err := c.sendBatch(bets)
			if err != nil {
				log.Errorf("action: send_batch | result: fail | error: %v", err)
				return
			}

			ack, err := c.receiveACK(msgID, msgIDHex)
			c.conn.Close()
			if err != nil {
				log.Errorf("action: receive_ack | result: fail | error: %v", err)
				return
			}

			switch ack {
			case "{result:success}":
				log.Infof("action: batch_enviado | result: success | id: %s | cantidad: %d", msgIDHex, len(bets))
			case "{result:failure}":
				log.Errorf("action: batch_enviado | result: fail | id: %s | cantidad: %d", msgIDHex, len(bets))
				return
			default:
				log.Errorf("action: receive_ack | result: fail | reason: unexpected_payload | content: %s", ack)
				return
			}

			time.Sleep(c.config.LoopPeriod)
		}
	}
}
