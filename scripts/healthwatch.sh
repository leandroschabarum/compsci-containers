#!/bin/sh
# O Podman agenda os healthchecks com timers do systemd, e o Alpine usa OpenRC:
# sem este laço, nenhum healthcheck roda. Ele também reinicia quem ficar
# unhealthy, o papel de --health-on-failure=restart, que o podman-compose não repassa.
INTERVAL=${INTERVAL:-5}

while true; do
  # Em paralelo, para uma réplica travada (que demora o timeout) não atrasar as outras.
  # Contêineres sem healthcheck dão erro aqui, e o erro é ignorado.
  for c in $(podman ps --format '{{.Names}}'); do
    podman healthcheck run "$c" >/dev/null 2>&1 &
  done
  wait

  for c in $(podman ps --filter health=unhealthy --format '{{.Names}}'); do
    echo "$(date '+%F %T') $c unhealthy, reiniciando"
    podman restart "$c"
  done
  sleep "$INTERVAL"
done
