
MESSAGE="Hello, server!"

RESPONSE=$(docker run --rm --network tp0_testing_net alpine/netcat server 12345 <<< "$MESSAGE")

if [ "$RESPONSE" == "$MESSAGE" ]; then
    echo "action: test_echo_server | result: success"
else
    echo "action: test_echo_server | result: fail"
fi