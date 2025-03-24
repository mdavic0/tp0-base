package common

import (
	"bufio"
	"bytes"
	"context"
	"crypto/md5"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/op/go-logging"
)

var log = logging.MustGetLogger("log")

// MessageTypes
const (
	MessageTypeApuesta = 1
	MessageTypeACK     = 2
)

// ClientConfig Configuration used by the client
type ClientConfig struct {
	ID            string
	ServerAddress string
	LoopAmount    int
	LoopPeriod    time.Duration
	Bet           Bet
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

// StartClientLoop Send messages to the client until some time threshold is met
func (c *Client) StartClientLoop(ctx context.Context) {
	// There is an autoincremental msgID to identify every message sent
	// Messages if the message amount threshold has not been surpassed
	for msgID := 1; msgID <= c.config.LoopAmount; msgID++ {
		select {
		case <-ctx.Done():
			log.Infof(`action: exit | result: success | container: client_id: %v| reason: signal | signal: SIGTERM`, c.config.ID)
			return
		default:
			if err := c.createClientSocket(); err != nil {
				log.Errorf("action: create_socket | result: fail | error: %v", err)
				return
			}

			// Armar payload estilo JSON plano
			payload := fmt.Sprintf(
				"{agency:%s,nombre:%s,apellido:%s,dni:%s,nacimiento:%s,numero:%s}",
				c.config.ID,
				c.config.Bet.Nombre,
				c.config.Bet.Apellido,
				c.config.Bet.DNI,
				c.config.Bet.Nacimiento,
				c.config.Bet.Numero,
			)

			// Generar ID
			msgIDBytes, msgIDHex := generateMessageID(payload)
			body := []byte(payload)
			msgLen := uint32(len(body) + 2 + 16)

			// Armar mensaje
			buf := new(bytes.Buffer)
			binary.Write(buf, binary.BigEndian, msgLen)
			binary.Write(buf, binary.BigEndian, uint16(MessageTypeApuesta))
			buf.Write(msgIDBytes)
			buf.Write(body)

			// Enviar mensaje
			err := writeExactly(c.conn, buf.Bytes())
			if err != nil {
				log.Errorf("action: send_message | result: fail | error: %v", err)
				c.conn.Close()
				return
			}
			log.Infof("action: apuesta_enviada | result: in_progress | id: %s | dni: %s | numero: %s", msgIDHex, c.config.Bet.DNI, c.config.Bet.Numero)

			// Leer ACK
			reader := bufio.NewReader(c.conn)

			// Leer header del ACK
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
				log.Infof("action: apuesta_enviada | result: success | id: %s | dni: %s | numero: %s", hex.EncodeToString(ackID), c.config.Bet.DNI, c.config.Bet.Numero)
			case "{result:failure}":
				log.Errorf("action: apuesta_enviada | result: fail | reason: server_response | dni: %s | numero: %s", c.config.Bet.DNI, c.config.Bet.Numero)
				return
			default:
				log.Errorf("action: receive_ack | result: fail | reason: unexpected_payload | content: %s", ackStr)
				return
			}

			time.Sleep(c.config.LoopPeriod)
		}
	}
	log.Infof("action: loop_finished | result: success | client_id: %v", c.config.ID)
}
