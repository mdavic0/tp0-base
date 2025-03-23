import sys


def generate_docker_compose(filename, num_clients):
    yaml_content = """name: tp0
services:
  server:
    container_name: server
    image: server:latest
    entrypoint: python3 /main.py
    environment:
      - PYTHONUNBUFFERED=1
      - LOGGING_LEVEL=DEBUG
    volumes:
      - ./server/config.ini:/config/config.ini
    networks:
      - testing_net
"""

    for i in range(1, num_clients + 1):
        yaml_content += f"""
  client{i}:
    container_name: client{i}
    image: client:latest
    entrypoint: /client
    environment:
      - CLI_ID={i}
      - CLI_LOG_LEVEL=DEBUG
    volumes:
      - ./client/config.yaml:/config/config.yaml
    networks:
      - testing_net
    depends_on:
      - server
"""

    yaml_content += """
networks:
  testing_net:
    ipam:
      driver: default
      config:
        - subnet: 172.25.125.0/24
"""

    with open(filename, "w") as file:
        file.write(yaml_content)


if __name__ == "__main__":
    if len(sys.argv) != 3:
        print("Usage: python3 docker-compose-generator.py <filename> <num_clients>")
        sys.exit(1)

    filename = sys.argv[1]
    try:
        num_clients = int(sys.argv[2])
    except ValueError:
        print("Error: <num_clients> must be an integer.")
        sys.exit(1)

    if num_clients < 1:
        print("Error: <num_clients> must be at least 1.")
        sys.exit(1)

    generate_docker_compose(filename, num_clients)
