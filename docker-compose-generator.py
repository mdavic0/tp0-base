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
    volumes:
      - ./server/config.ini:/config.ini
    networks:
      - testing_net
"""

    # Datos de ejemplo para apuestas
    example_data = [
        ("Santiago", "Lorca", "30904465", "1999-03-17", "7574"),
        ("Ana", "Gómez", "11111111", "1990-01-01", "1234"),
        ("Luis", "Fernandez", "22222222", "1985-05-10", "5678"),
        ("Carla", "Mendez", "33333333", "1993-08-15", "8888"),
        ("Diego", "Ramirez", "44444444", "1988-12-20", "4321")
    ]

    for i in range(1, num_clients + 1):
        nombre, apellido, documento, nacimiento, numero = example_data[(i - 1) % len(example_data)]
        yaml_content += f"""
  client{i}:
    container_name: client{i}
    image: client:latest
    entrypoint: /client
    environment:
      - CLI_ID={i}
      - CLI_APUESTA_NOMBRE={nombre}
      - CLI_APUESTA_APELLIDO={apellido}
      - CLI_APUESTA_DOCUMENTO={documento}
      - CLI_APUESTA_NACIMIENTO={nacimiento}
      - CLI_APUESTA_NUMERO={numero}
    volumes:
      - ./client/config.yaml:/config.yaml
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

    if num_clients < 0:
        print("Error: <num_clients> must be a non-negative integer.")
        sys.exit(1)

    generate_docker_compose(filename, num_clients)
