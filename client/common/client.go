package common

import (
	"bufio"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/op/go-logging"
)

var log = logging.MustGetLogger("log")

// ClientConfig Configuration used by the client
type ClientConfig struct {
	ID            string
	ServerAddress string
	LoopAmount    int
	LoopPeriod    time.Duration
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
		log.Criticalf(
			"action: connect | result: fail | client_id: %v | error: %v",
			c.config.ID,
			err,
		)
		return err
	}
	c.conn = conn
	return nil
}

// Send a message to the server avoiding the short-write problem
func (c *Client) sendMessage(msg string) error {
	writer := bufio.NewWriter(c.conn)
	_, err := writer.WriteString(msg)
	if err != nil {
		log.Fatalf("action: send_message | result: fail | client_id: %v | error: %v",
			c.config.ID,
			err,
		)
		return err
	}
	err = writer.Flush()
	if err != nil {
		log.Fatalf("action: flush_message | result: fail | client_id: %v | error: %v",
			c.config.ID,
			err,
		)
		return err
	}
	return nil
}

// Receives a message from the server avoiding the short-read problem
func (c *Client) receiveMessage() (string, error) {
	msg, err := bufio.NewReader(c.conn).ReadString(DELIMITER[0])
	if err != nil {
		log.Errorf("action: receive_message | result: fail | client_id: %v | error: %v",
			c.config.ID,
			err,
		)
	}
	return msg, err
}

func (c *Client) verifyBetResult(receivedMsg string, bet Bet) {
	if receivedMsg == ACK_MESSAGE {
		log.Infof("action: apuesta_enviada | result: success | dni: %d | numero: %d",
			bet.Document,
			bet.Number,
		)
	} else {
		log.Errorf("action: apuesta_enviada | result: fail | dni: %d | numero: %d",
			bet.Document,
			bet.Number,
		)
	}
}

// StartClientLoop Send messages to the client until some time threshold is met
func (c *Client) StartClientLoop(bet Bet) {
	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, syscall.SIGINT, syscall.SIGTERM)

	// There is an autoincremental msgID to identify every message sent
	// Messages if the message amount threshold has not been surpassed
	for msgID := 1; msgID <= c.config.LoopAmount; msgID++ {
		select {
		case sig := <-stopChan:
			log.Infof("action: signal_received | signal: %v | client_id: %v", sig, c.config.ID)
			c.Stop()
			close(stopChan)
			return
		default:
			// Create the connection the server in every loop iteration.
			c.createClientSocket()

			msgSent := bet.ParseToString() + DELIMITER
			err := c.sendMessage(msgSent)
			if err != nil {
				return
			}

			msgReceived, err := c.receiveMessage()
			if err != nil {
				return
			}

			c.conn.Close()

			c.verifyBetResult(msgReceived, bet)
		}
		// Wait a time between sending one message and the next one
		time.Sleep(c.config.LoopPeriod)
	}
	log.Infof("action: loop_finished | result: success | client_id: %v", c.config.ID)
}

func (c *Client) Stop() {
	if c.conn != nil {
		log.Infof("action: closing_connection | client_id: %v", c.config.ID)
		c.conn.Close()
	}
	log.Infof("action: client_stopped | client_id: %v", c.config.ID)
}
