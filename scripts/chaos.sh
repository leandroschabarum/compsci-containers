#!/bin/sh
# Provoca as falhas da demonstração.
# Uso: scripts/chaos.sh crash|hang|oom [réplica] | stop | start
set -eu
target=${2:-api2}

# crash e hang mandam o sinal direto ao processo, pelo host. "podman kill"
# marca o contêiner como parado pelo usuário, e aí o restart=always não age.
pid() { podman inspect -f '{{.State.Pid}}' "$1"; }

case "${1:-}" in
  crash)
    kill -KILL "$(pid "$target")"
    echo "$target morto com SIGKILL; o restart=always deve trazê-lo de volta"
    ;;
  hang)
    kill -STOP "$(pid "$target")"
    echo "$target congelado com SIGSTOP; o healthwatch deve reiniciá-lo em uns 20s"
    ;;
  oom)
    echo "$target seguindo um loop de redirecionamentos até estourar 96 MB..."
    podman exec "$target" wget -q -O - --header 'Content-Type: application/json' \
      --post-data "{\"url\":\"http://127.0.0.1:8080/shortme?case=loop\"}" \
      http://127.0.0.1:8080/links || true
    # O exec termina junto com a réplica, antes de o Podman registrar a saída e
    # reiniciá-la: sem a espera, o inspect ainda mostraria o estado anterior.
    sleep 3
    # OOMKilled continua true depois do primeiro OOM, então o RestartCount é
    # o que mostra que esta execução também derrubou a réplica.
    podman inspect -f "$target OOMKilled={{.State.OOMKilled}} RestartCount={{.RestartCount}}" "$target"
    ;;
  stop)
    podman stop api1 api2
    echo "api1 e api2 paradas; o nginx deve mandar tudo para a api3"
    ;;
  start)
    podman start api1 api2 api3
    ;;
  *)
    echo "uso: $0 crash|hang|oom [réplica] | stop | start" >&2
    exit 1
    ;;
esac
