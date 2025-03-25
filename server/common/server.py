import socket
import logging
import signal
from common.utils import Bet, store_bets, load_bets, has_won

class Server:
    def __init__(self, port, listen_backlog):
        # Initialize server socket
        self._server_socket = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        self._server_socket.bind(('', port))
        self._server_socket.listen(listen_backlog)
        self._should_terminate = False
        self._total_agencies = 3
        self._notified_agencies = set()
        self._lottery_done = False
        self._winners_by_agency = {}
        signal.signal(signal.SIGTERM, self._handle_sigterm)

    def run(self):
        """
        Dummy Server loop

        Server that accept a new connections and establishes a
        communication with a client. After client with communucation
        finishes, servers starts to accept new connections again
        """
        while not self._should_terminate:
            try:
                client_sock = self.__accept_new_connection()
                self.__handle_client_connection(client_sock)
            except OSError as e:
                if self._should_terminate and e.errno == 9:  # Bad file descriptor
                    logging.info("action: accept_connections | result: fail | reason: graceful_shutdown")
                else:
                    logging.error(f"action: accept_connections | result: fail | error: {e}")
                break

        logging.info('action: exit | result: success | container: server')

    def __accept_new_connection(self):
        """
        Accept new connections

        Function blocks until a connection to a client is made.
        Then connection created is printed and returned
        """

        # Connection arrived
        logging.info('action: accept_connections | result: in_progress')
        c, addr = self._server_socket.accept()
        logging.info(f'action: accept_connections | result: success | ip: {addr[0]}')
        return c

    def _handle_sigterm(self, signum, frame):
        logging.info('action: receive_signal | result: success | container: server | signal: SIGTERM')
        self._should_terminate = True
        self._server_socket.close()

    def __handle_client_connection(self, client_sock):
        try:
            raw_len = recv_all(client_sock, 4)
            total_len = int.from_bytes(raw_len, byteorder='big')

            raw_type = recv_all(client_sock, 2)
            msg_type = int.from_bytes(raw_type, byteorder='big')

            msg_id = recv_all(client_sock, 16)
            payload_len = total_len - 2 - 16
            payload_raw = recv_all(client_sock, payload_len)

            if msg_type == 1:
                self.__handle_single_bet(client_sock, msg_id, payload_raw)
            elif msg_type == 3:
                self.__handle_batch(client_sock, msg_id, payload_raw)
            elif msg_type == 4:
                self.__handle_finished_notification(client_sock, msg_id, payload_raw)
            elif msg_type == 5:
                self.__handle_query_winners(client_sock, msg_id, payload_raw)
            else:
                logging.warning(f"action: receive_message | result: ignored | reason: unknown_type | type: {msg_type}")
                client_sock.close()

        except Exception as e:
            logging.error(f"action: handle_client | result: fail | error: {e}")
            client_sock.close()

    def __handle_single_bet(self, client_sock, msg_id, payload_raw):
        try:
            payload_str = payload_raw.decode('utf-8')
            addr = client_sock.getpeername()
            logging.info(f"action: receive_message | result: success | ip: {addr[0]} | msg: {payload_str}")

            data = parse_payload_string(payload_str)

            bet = Bet(
                agency=data["agency"],
                first_name=data["nombre"],
                last_name=data["apellido"],
                document=data["dni"],
                birthdate=data["nacimiento"],
                number=data["numero"]
            )

            store_bets([bet])
            logging.info(f"action: apuesta_almacenada | result: success | dni: {bet.document} | numero: {bet.number}")
            result_payload = "{result:success}"

        except Exception as e:
            logging.error(f"action: apuesta_almacenada | result: fail | error: {e}")
            result_payload = "{result:failure}"

        self.__send_ack(client_sock, msg_id, result_payload)

    def __handle_batch(self, client_sock, msg_id, payload_raw):
        try:
            payload_str = payload_raw.decode('utf-8')
            addr = client_sock.getpeername()

            raw_bets = payload_str.split('|')
            bets = []
            for payload in raw_bets:
                data = parse_payload_string(payload)
                bet = Bet(
                    agency=data["agency"],
                    first_name=data["nombre"],
                    last_name=data["apellido"],
                    document=data["dni"],
                    birthdate=data["nacimiento"],
                    number=data["numero"]
                )
                bets.append(bet)

            store_bets(bets)
            result_payload = "{result:success}"
            logging.info(f"action: apuesta_recibida | result: success | cantidad: {len(bets)}")

        except Exception as e:
            logging.error(f"action: apuesta_recibida | result: fail | cantidad: {len(raw_bets)}")
            result_payload = "{result:failure}"

        self.__send_ack(client_sock, msg_id, result_payload)

    def __handle_finished_notification(self, client_sock, msg_id, payload_raw):
        try:
            payload_str = payload_raw.decode('utf-8')
            data = parse_payload_string(payload_str)
            agency = int(data["agency"])

            self._notified_agencies.add(agency)

            if not self._lottery_done and len(self._notified_agencies) == self._total_agencies:
                self._run_lottery()

            result_payload = "{result:success}"

        except Exception as e:
            logging.error(f"action: notify_finished | result: fail | error: {e}")
            result_payload = "{result:failure}"

        self.__send_ack(client_sock, msg_id, result_payload)

    def _run_lottery(self):
        try:
            logging.info("action: sorteo | result: in_progress")
            self._winners_by_agency = {}

            for bet in load_bets():
                if has_won(bet):
                    self._winners_by_agency.setdefault(bet.agency, []).append(bet.document)

            self._lottery_done = True
            logging.info("action: sorteo | result: success")
        except Exception as e:
            logging.error(f"action: sorteo | result: fail | error: {e}")

    def __send_ack(self, client_sock, msg_id, result_payload):
        addr = client_sock.getpeername()
        ack_payload = result_payload.encode('utf-8')
        ack_total_len = 2 + 16 + len(ack_payload)
        ack_msg = (
            ack_total_len.to_bytes(4, byteorder='big') +
            (2).to_bytes(2, byteorder='big') +
            msg_id +
            ack_payload
        )
        client_sock.sendall(ack_msg)
        logging.info(f"action: send_ack | result: success | ip: {addr[0]} | id: {msg_id.hex()} | msg: {result_payload}")
        client_sock.close()

    def __handle_query_winners(self, client_sock, msg_id, payload_raw):
        try:
            payload_str = payload_raw.decode('utf-8')
            data = parse_payload_string(payload_str)
            agency = int(data["agency"])

            if not self._lottery_done:
                result_payload = "{result:in_progress}"
            else:
                winners = self._winners_by_agency.get(agency, [])
                result_payload = "{ganadores:" + "|".join(winners) + "}"

        except Exception as e:
            logging.error(f"action: consulta_ganadores | result: fail | error: {e}")
            result_payload = "{result:failure}"

        payload = result_payload.encode("utf-8")
        total_len = 2 + 16 + len(payload)
        msg = (
            total_len.to_bytes(4, "big")
            + (6).to_bytes(2, "big")
            + msg_id
            + payload
        )
        client_sock.sendall(msg)
        client_sock.close()


def recv_all(sock, n):
    data = b''
    while len(data) < n:
        chunk = sock.recv(n - len(data))
        if not chunk:
            raise ConnectionError("Socket closed before receiving full data")
        data += chunk
    return data


def parse_payload_string(payload_str):
    payload_str = payload_str.strip('{}\n ')
    pairs = payload_str.split(',')
    parsed = {}
    for pair in pairs:
        if ':' not in pair:
            raise ValueError(f"Invalid field in payload: {pair}")
        k, v = pair.split(':', 1)
        parsed[k.strip()] = v.strip()
    return parsed
