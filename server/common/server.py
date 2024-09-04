import socket
import logging
import signal
import threading
from common import utils

class Server:
    def __init__(self, port, listen_backlog):
        # Initialize server socket
        self._server_socket = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        self._server_socket.bind(('', port))
        self._server_socket.listen(listen_backlog)

        self._is_running = threading.Event()
        self._is_running.set()
        signal.signal(signal.SIGTERM, self.stop)

    def run(self):
        """
        Server loop with graceful shutdown handling.

        Server that accepts new connections and establishes communication
        with a client. When a shutdown signal is received, stops accepting new
        connections and gracefully closes the server.
        """

        while self._is_running.is_set():
            client_sock = self.__accept_new_connection()
            if client_sock is not None:
                self.__handle_client_connection(client_sock)

    def __handle_client_connection(self, client_sock):
        """
        Read message from a specific client socket and closes the socket

        If a problem arises in the communication with the client, the
        client socket will also be closed
        """
        try:
            msg = self.receive_message(client_sock)
            if msg is None:
                return
            
            bet = utils.parse_bets(msg)[0]

            self.send_message(client_sock, utils.ACK_MESSAGE)
            
            utils.store_bets([bet])

            logging.info(f'action: apuesta_almacenada | result: success | dni: {bet.document} | numero: {bet.number}')
        except OSError as e:
            logging.error("action: receive_message | result: fail | error: {e}")
        finally:
            client_sock.close()

    def __accept_new_connection(self):
        """
        Accept new connections, returns client socket.
        If the server is shutting down, returns None.
        """
        try:
            # Connection arrived
            logging.info('action: accept_connections | result: in_progress')
            c, addr = self._server_socket.accept()
            logging.info(f'action: accept_connections | result: success | ip: {addr[0]}')
            return c
        except OSError as e:
            # logging.error(f"action: accept_connections | result: fail | error: {e}")
            return None

    def stop(self, *args):
        """
        Gracefully stops the server, stops accepting new connections and closes the server socket.
        """
        logging.info("action: shutdown_initiated | result: success")
        self._is_running.clear()
        self._server_socket.shutdown(socket.SHUT_RDWR)
        logging.info("action: server_stopped | result: success")


    def receive_message(self, client_sock):
        """
        Try reading a complete message from client avoiding short-reads
        """
        msg = b''
        recv_all = False
        while not recv_all:

            packet = client_sock.recv(1024)
            if not packet:
                logging.error('action: receive_message | result: fail | error: client closed connection')
                client_sock.close()
                return None
            
            msg += packet

            if msg.endswith(utils.DELIMITER):
                recv_all = True

        addr = client_sock.getpeername()
        logging.info(f'action: receive_message | result: success | ip: {addr[0]} | msg: {msg.decode("utf-8")}')
        return msg.decode('utf-8')


    def send_message(self, client_sock, msg):
        """
        Send a message to a client
        It avoids short-writes
        """
        try:
            client_sock.sendall(msg.encode('utf-8'))
        except OSError as e:
            logging.error(f'action: send_message | result: fail | error: {e}')
            client_sock.close()