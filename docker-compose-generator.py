import sys
import yaml

def generate_docker_compose(filename, num_clients):
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
        docker_compose['services'][f'client{i}'] = {
            'container_name': f'client{i}',
            'image': 'client:latest',
            'entrypoint': '/client',
            'environment': [
                f'CLI_ID={i}',
                'CLI_LOG_LEVEL=DEBUG'
            ],
            'networks': ['testing_net'],
            'depends_on': ['server']
        }

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

    generate_docker_compose(filename, num_clients)
