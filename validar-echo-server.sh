#!/bin/bash

# Configuración
SERVER_HOSTNAME="server"
NETWORK_NAME="tp0_testing_net" 
TEST_MESSAGE="Hello, EchoServer!"
EXPECTED_RESPONSE="$TEST_MESSAGE"
SERVER_PORT=12345


ACTUAL_RESPONSE=$(docker run --rm --network "$NETWORK_NAME" busybox:latest sh -c "echo '$TEST_MESSAGE' | nc -w 2 $SERVER_HOSTNAME $SERVER_PORT")

if [ "$ACTUAL_RESPONSE" == "$EXPECTED_RESPONSE" ]; then
  echo "action: test_echo_server | result: success"
else
  echo "action: test_echo_server | result: fail"
fi
