#!/bin/sh

# Mensaje de prueba
MESSAGE="Hello, server!"

# Ejecutar netcat dentro de un contenedor de Alpine
RESPONSE=$(docker run --rm --network tp0_testing_net alpine/netcat server 12345 <<EOF
$MESSAGE
EOF
)

# Comparar la respuesta con el mensaje enviado
if [ "$RESPONSE" = "$MESSAGE" ]; then
    echo "action: test_echo_server | result: success"
else
    echo "action: test_echo_server | result: fail"
fi