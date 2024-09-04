import sys
import yaml

def load_env_file(env_file):
    env_vars = {}
    try:
        with open(env_file, 'r') as file:
            for line in file:
                # Ignorar líneas vacías o comentarios
                line = line.strip()
                if line and not line.startswith('#'):
                    key, value = line.split('=', 1)
                    env_vars[key.strip()] = value.strip()
    except FileNotFoundError:
        print(f"Error: {env_file} not found.")
        sys.exit(1)
    return env_vars

def generate_docker_compose(filename, num_clients, env_file):
    # Load environment variables manually from the .env file
    env_vars = load_env_file(env_file)
    
    docker_compose = {
        'name': 'tp0',
        'services': {
            'server': {
                'container_name': 'server',
                'image': 'server:latest',
                'entrypoint': 'python3 /main.py',
                'environment': [
                    'PYTHONUNBUFFERED=1',
                    'LOGGING_LEVEL=DEBUG'
                ],
                'volumes': [ './server/config.ini:/config/config.ini' ],
                'networks': ['testing_net']
            }
        },
        'networks': {
            'testing_net': {
                'ipam': {
                    'driver': 'default',
                    'config': [
                        {'subnet': '172.25.125.0/24'}
                    ]
                }
            }
        }
    }

    for i in range(1, num_clients + 1):
        # Prepare the client-specific environment variables
        client_env = [
            f'CLI_ID={i}',
            'CLI_LOG_LEVEL=DEBUG',
            f'CLI_NOMBRE={env_vars.get(f"CLI{i}_NOMBRE", "")}',
            f'CLI_APELLIDO={env_vars.get(f"CLI{i}_APELLIDO", "")}',
            f'CLI_DOCUMENTO={env_vars.get(f"CLI{i}_DOCUMENTO", "")}',
            f'CLI_NACIMIENTO={env_vars.get(f"CLI{i}_NACIMIENTO", "")}',
            f'CLI_NUMERO={env_vars.get(f"CLI{i}_NUMERO", "")}',
        ]

        # Add the client service to docker-compose configuration
        docker_compose['services'][f'client{i}'] = {
            'container_name': f'client{i}',
            'image': 'client:latest',
            'entrypoint': '/client',
            'environment': client_env,
            'volumes': [ './client/config.yaml:/config/config.yaml' ],
            'networks': ['testing_net'],
            'depends_on': ['server']
        }

    # Write the generated configuration to the specified file
    with open(filename, 'w') as file:
        yaml.dump(docker_compose, file, sort_keys=False)

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

    generate_docker_compose(filename, num_clients, "./client/.env")
