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
	MessageTypeBet   = 1
	MessageTypeACK   = 2
	MessageTypeBatch = 3
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

func (c *Client) StartClientLoop(ctx context.Context) {
	file, err := os.Open("/data/agency.csv")
	if err != nil {
		log.Errorf("action: open_csv | result: fail | error: %v", err)
		return
	}
	defer file.Close()

	csvReader := csv.NewReader(file)

	for {
		select {
		case <-ctx.Done():
			log.Infof(`action: exit | result: success | container: client_id: %v | reason: signal | signal: SIGTERM`, c.config.ID)
			return
		default:
			var bets []Bet
			for len(bets) < c.config.BatchBets {
				record, err := csvReader.Read()
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

			if len(bets) == 0 {
				return // no hay más apuestas que enviar
			}

			if err := c.createClientSocket(); err != nil {
				log.Errorf("action: create_socket | result: fail | error: %v", err)
				return
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

			err = writeExactly(c.conn, buf.Bytes())
			if err != nil {
				log.Errorf("action: send_batch | result: fail | error: %v", err)
				c.conn.Close()
				return
			}
			log.Infof("action: batch_enviado | result: in_progress | id: %s | cantidad: %d", msgIDHex, len(bets))

			reader := bufio.NewReader(c.conn)

			lenBytes, err := readExactly(reader, 4)
			if err != nil {
				log.Errorf("action: receive_ack | result: fail | error: %v", err)
				c.conn.Close()
				return
			}
			totalLen := binary.BigEndian.Uint32(lenBytes)

			typeBytes, err := readExactly(reader, 2)
			if err != nil {
				log.Errorf("action: receive_ack | result: fail | error: %v", err)
				c.conn.Close()
				return
			}
			msgType := binary.BigEndian.Uint16(typeBytes)
			if msgType != MessageTypeACK {
				log.Errorf("action: receive_ack | result: fail | error: unexpected_type | expected: %d got: %d", MessageTypeACK, msgType)
				c.conn.Close()
				return
			}

			ackID, err := readExactly(reader, 16)
			if err != nil {
				log.Errorf("action: receive_ack | result: fail | error: %v", err)
				c.conn.Close()
				return
			}

			if !bytes.Equal(ackID, msgIDBytes) {
				log.Errorf("action: receive_ack | result: fail | error: mismatched_id | expected: %s", msgIDHex)
				c.conn.Close()
				return
			}

			payloadLen := int(totalLen) - 2 - 16
			ackPayload, err := readExactly(reader, payloadLen)
			c.conn.Close()
			if err != nil {
				log.Errorf("action: receive_ack | result: fail | error: %v", err)
				return
			}

			ackStr := string(ackPayload)
			log.Infof("action: receive_ack | result: success | id: %s | payload: %s", hex.EncodeToString(ackID), ackStr)

			switch ackStr {
			case "{result:success}":
				log.Infof("action: batch_enviado | result: success | id: %s | cantidad: %d", hex.EncodeToString(ackID), len(bets))
			case "{result:failure}":
				log.Errorf("action: batch_enviado | result: fail | id: %s | cantidad: %d", hex.EncodeToString(ackID), len(bets))
				return
			default:
				log.Errorf("action: receive_ack | result: fail | reason: unexpected_payload | content: %s", ackStr)
				return
			}

			time.Sleep(c.config.LoopPeriod)
		}
	}
}
