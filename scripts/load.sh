#!/bin/sh
# Gera tráfego contínuo pelo nginx e conta respostas boas e falhas.
# Uso: scripts/load.sh [url-base]
URL=${1:-http://127.0.0.1:8080}

# O link aponta para o /health da própria réplica: com CHAOS_ENABLED a API segue
# a URL ao encurtar, e assim o teste não depende de internet na hora da demonstração.
code=$(curl -s -X POST -d '{"url":"http://127.0.0.1:8080/health"}' "$URL/links" | sed -n 's/.*"code":"\([^"]*\)".*/\1/p')
if [ -z "$code" ]; then
  echo "não foi possível criar o link de teste em $URL" >&2
  exit 1
fi

ok=0
fail=0
while true; do
  out=$(curl -s -o /dev/null -m 10 -w '%{http_code} %{time_total}s %header{x-instance}' "$URL/$code")
  case "$out" in
    302*) ok=$((ok + 1)) ;;
    *) fail=$((fail + 1)) ;;
  esac
  echo "$(date +%T)  $out  ok=$ok falhas=$fail"
  sleep 0.2
done
