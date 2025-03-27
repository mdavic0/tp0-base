import socket
import logging
import signal
import multiprocessing
from common.utils import Bet, store_bets, load_bets, has_won

# Message types
MSG_TYPE_BET = 1
MSG_TYPE_ACK = 2
MSG_TYPE_BATCH = 3
MSG_TYPE_FINISHED = 4
MSG_TYPE_QUERY_WINNERS = 5
MSG_TYPE_WINNERS_RESPONSE = 6

# Binary protocol sizes
HEADER_TOTAL_LEN_SIZE = 4
HEADER_TYPE_SIZE = 2
HEADER_ID_SIZE = 16
HEADER_TOTAL_SIZE = HEADER_TYPE_SIZE + HEADER_ID_SIZE

class Server:
    def __init__(self, port, listen_backlog):
        self._server_socket = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        self._server_socket.bind(('', port))
        self._server_socket.listen(listen_backlog)

        manager = multiprocessing.Manager()
        self._should_terminate = multiprocessing.Value('b', False)
        self._total_agencies = 3
        self._notified_agencies_count = multiprocessing.Value('i', 0)
        self._lottery_done = multiprocessing.Value('b', False)
        self._winners_by_agency = manager.dict()
        self._storefile_lock = multiprocessing.Lock()

        signal.signal(signal.SIGTERM, self._handle_sigterm)

    def run(self):
        """
        Dummy Server loop

        Server that accept a new connections and establishes a
        communication with a client. After client with communucation
        finishes, servers starts to accept new connections again
        """
        while not self._should_terminate.value:
            try:
                client_sock = self.__accept_new_connection()
                if client_sock:
                    process = multiprocessing.Process(
                        target=self.__handle_client_connection,
                        args=(client_sock,)
                    )
                    process.start()
            except OSError:
                break

        for p in multiprocessing.active_children():
            p.join()

        logging.info('action: exit | result: success | container: server')

    def __accept_new_connection(self):
        """
        Accept new connections

        Function blocks until a connection to a client is made.
        Then connection created is printed and returned
        """
        logging.info('action: accept_connections | result: in_progress')
        try:
            c, addr = self._server_socket.accept()
            logging.info(f'action: accept_connections | result: success | ip: {addr[0]}')
            return c
        except OSError:
            return None

    def _handle_sigterm(self, signum, frame):
        logging.info('action: receive_signal | result: success | container: server | signal: SIGTERM')
        with self._should_terminate.get_lock():
            self._should_terminate.value = True
        self._server_socket.close()

    def __handle_client_connection(self, client_sock):
        try:
            raw_len = recv_all(client_sock, HEADER_TOTAL_LEN_SIZE)
            total_len = int.from_bytes(raw_len, byteorder='big')

            raw_type = recv_all(client_sock, HEADER_TYPE_SIZE)
            msg_type = int.from_bytes(raw_type, byteorder='big')

            msg_id = recv_all(client_sock, HEADER_ID_SIZE)
            payload_len = total_len - HEADER_TOTAL_SIZE
            payload_raw = recv_all(client_sock, payload_len)
            payload_str = payload_raw.decode("utf-8")

            if msg_type == MSG_TYPE_BET:
                self.__handle_single_bet(client_sock, msg_id, payload_str)
            elif msg_type == MSG_TYPE_BATCH:
                self.__handle_batch(client_sock, msg_id, payload_str)
            elif msg_type == MSG_TYPE_FINISHED:
                self.__handle_finished_notification(client_sock, msg_id, payload_str)
            elif msg_type == MSG_TYPE_QUERY_WINNERS:
                self.__handle_query_winners(client_sock, msg_id, payload_str)
            else:
                logging.warning(f"action: receive_message | result: ignored | reason: unknown_type | type: {msg_type}")
        except Exception as e:
            logging.error(f"action: handle_client | result: fail | error: {e}")
        finally:
            client_sock.close()

    def __handle_single_bet(self, client_sock, msg_id, payload_str):
        try:
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
            with self._storefile_lock:
                store_bets([bet])
            logging.info(f"action: apuesta_almacenada | result: success | dni: {bet.document} | numero: {bet.number}")
            self.__send_ack(client_sock, msg_id, "{result:success}")
        except Exception as e:
            logging.error(f"action: apuesta_almacenada | result: fail | error: {e}")
            self.__send_ack(client_sock, msg_id, "{result:failure}")

    def __handle_batch(self, client_sock, msg_id, payload_str):
        try:
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
            with self._storefile_lock:
                store_bets(bets)
            logging.info(f"action: apuesta_recibida | result: success | cantidad: {len(bets)}")
            self.__send_ack(client_sock, msg_id, "{result:success}")
        except Exception as e:
            logging.error(f"action: apuesta_recibida | result: fail | cantidad: {len(raw_bets)}")
            self.__send_ack(client_sock, msg_id, "{result:failure}")

    def __handle_finished_notification(self, client_sock, msg_id, payload_str):
        try:
            data = parse_payload_string(payload_str)
            with self._notified_agencies_count.get_lock():
                self._notified_agencies_count.value += 1

            with self._lottery_done.get_lock():
                if not self._lottery_done.value and self._notified_agencies_count.value == self._total_agencies:
                    logging.info("action: sorteo | result: in_progress")
                    self._run_lottery()
                    self._lottery_done.value = True
                    logging.info("action: sorteo | result: success")

            self.__send_ack(client_sock, msg_id, "{result:success}")
        except Exception as e:
            logging.error(f"action: notify_finished | result: fail | error: {e}")
            self.__send_ack(client_sock, msg_id, "{result:failure}")

    def _run_lottery(self):
        with self._storefile_lock:
            for bet in load_bets():
                if has_won(bet):
                    agency = bet.agency
                    winners = self._winners_by_agency.get(agency, [])
                    winners.append(bet.document)
                    self._winners_by_agency[agency] = winners


    def __send_ack(self, client_sock, msg_id, result_payload):
        ack_payload = result_payload.encode('utf-8')
        ack_total_len = HEADER_TOTAL_SIZE + len(ack_payload)
        ack_msg = (
            ack_total_len.to_bytes(HEADER_TOTAL_LEN_SIZE, byteorder='big') +
            MSG_TYPE_ACK.to_bytes(HEADER_TYPE_SIZE, byteorder='big') +
            msg_id +
            ack_payload
        )
        client_sock.sendall(ack_msg)
    def __handle_query_winners(self, client_sock, msg_id, payload_str):
        try:
            data = parse_payload_string(payload_str)
            agency = int(data["agency"])
            with self._lottery_done.get_lock():
                if not self._lottery_done.value:
                    result_payload = "{result:in_progress}"
                else:
                    winners = self._winners_by_agency.get(agency, [])
                    result_payload = "{ganadores:" + "|".join(winners) + "}"
        except Exception as e:
            logging.error(f"action: consulta_ganadores | result: fail | error: {e}")
            result_payload = "{result:failure}"

        payload = result_payload.encode("utf-8")
        total_len = HEADER_TOTAL_SIZE + len(payload)
        msg = (
            total_len.to_bytes(HEADER_TOTAL_LEN_SIZE, "big") +
            MSG_TYPE_WINNERS_RESPONSE.to_bytes(HEADER_TYPE_SIZE, "big") +
            msg_id +
            payload
        )
        client_sock.sendall(msg)

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
