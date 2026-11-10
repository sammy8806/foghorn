#!/usr/bin/env bash
# Local Grafana lab for silence permission semantics.
# Usage: grafana-silence-lab.sh up|verify|env|down
# Never points at a shared Grafana: everything runs on 127.0.0.1:3013.
set -euo pipefail

NAME=foghorn-grafana-lab
IMAGE=grafana/grafana:13.2.4
URL=http://127.0.0.1:3013
ENV_FILE="${TMPDIR:-/tmp}/foghorn-grafana-lab.env"
ADMIN=(-u admin:admin)
SF=$(mktemp); trap 'rm -f "$SF"' EXIT

api() { # method path [json] -> body; status in $SF
  local method=$1 path=$2 data=${3:-} out
  out=$(mktemp)
  curl -s -o "$out" -w '%{http_code}' -X "$method" "${AUTH[@]}" \
    -H 'Content-Type: application/json' ${data:+--data "$data"} "$URL$path" >"$SF"
  cat "$out"; rm -f "$out"
}

wait_http() {
  for _ in $(seq 1 60); do
    curl -sf "$URL/api/health" >/dev/null 2>&1 && return 0
    sleep 2
  done
  echo "grafana did not become healthy" >&2; return 1
}

rule_json() { # uid folder title
  cat <<EOF
{"uid":"$1","title":"$3","folderUID":"$2","ruleGroup":"lab","condition":"A",
 "noDataState":"OK","execErrState":"Error","for":"0s","labels":{"severity":"warning"},
 "data":[{"refId":"A","datasourceUid":"__expr__","relativeTimeRange":{"from":600,"to":0},
   "model":{"type":"math","expression":"1 > 0","refId":"A"}}]}
EOF
}

sa_token() { # name role -> token
  local id
  AUTH=("${ADMIN[@]}")
  id=$(api POST /api/serviceaccounts "{\"name\":\"$1\",\"role\":\"$2\"}" | jq -r .id)
  [ "$id" != null ] || { echo "create SA $1 failed: $(<"$SF")" >&2; return 1; }
  api POST "/api/serviceaccounts/$id/tokens" "{\"name\":\"$1\"}" | jq -r .key
}

up() {
  docker rm -f "$NAME" >/dev/null 2>&1 || true
  docker run -d --name "$NAME" -p 127.0.0.1:3013:3000 \
    -e GF_AUTH_ANONYMOUS_ENABLED=false -e GF_SECURITY_ADMIN_PASSWORD=admin \
    "$IMAGE" >/dev/null
  wait_http
  AUTH=("${ADMIN[@]}")
  api POST /api/folders '{"uid":"lab-parent","title":"lab-parent"}' >/dev/null
  api POST /api/folders '{"uid":"lab-child","title":"lab-child","parentUid":"lab-parent"}' >/dev/null
  api POST /api/folders '{"uid":"lab-other","title":"lab-other"}' >/dev/null
  api POST /api/v1/provisioning/alert-rules "$(rule_json lab-rule-child lab-child 'Lab child rule')" >/dev/null
  [ "$(<"$SF")" = 201 ] || { echo "rule child: $(<"$SF")" >&2; return 1; }
  api POST /api/v1/provisioning/alert-rules "$(rule_json lab-rule-other lab-other 'Lab other rule')" >/dev/null
  [ "$(<"$SF")" = 201 ] || { echo "rule other: $(<"$SF")" >&2; return 1; }

  local scoped admin_tok sid
  scoped=$(sa_token lab-scoped Viewer)
  admin_tok=$(sa_token lab-admin Admin)
  AUTH=("${ADMIN[@]}")
  sid=$(api GET '/api/serviceaccounts/search?query=lab-scoped' | jq -r '.serviceAccounts[0].id')
  api POST /api/folders/lab-parent/permissions \
    "{\"items\":[{\"userId\":$sid,\"permission\":2}]}" >/dev/null
  [ "$(<"$SF")" = 200 ] || { echo "folder permission: $(<"$SF")" >&2; return 1; }

  umask 077
  cat >"$ENV_FILE" <<EOF
export GRAFANA_LAB_URL=$URL
export GRAFANA_LAB_ADMIN_TOKEN=$admin_tok
export GRAFANA_LAB_SCOPED_TOKEN=$scoped
EOF

  AUTH=("${ADMIN[@]}")
  for _ in $(seq 1 60); do
    n=$(api GET '/api/alertmanager/grafana/api/v2/alerts?active=true' | jq '[.[]|select(.labels.__alert_rule_uid__|startswith("lab-rule-"))]|length')
    [ "$n" = 2 ] && { echo "lab up: both rules firing"; return 0; }
    sleep 2
  done
  echo "rules not firing after 120s" >&2; return 1
}

silence_json() { # name value [id]
  local now end
  now=$(date -u +%Y-%m-%dT%H:%M:%SZ)
  end=$(date -u -v+1H +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || date -u -d '+1 hour' +%Y-%m-%dT%H:%M:%SZ)
  printf '{%s"matchers":[{"name":"%s","value":"%s","isEqual":true,"isRegex":false}],"startsAt":"%s","endsAt":"%s","createdBy":"lab","comment":"lab"}' \
    "${3:+\"id\":\"$3\",}" "$1" "$2" "$now" "$end"
}

CREATED=()
check() { # id expected-pattern actual description
  if [[ $3 =~ ^($2)$ ]]; then echo "PASS $1 ($3) $4"; else echo "FAIL $1 ($3, want $2) $4"; FAILED=1; fi
}

verify() {
  # shellcheck disable=SC1090
  source "$ENV_FILE"
  local SC=(-H "Authorization: Bearer $GRAFANA_LAB_SCOPED_TOKEN")
  local AD=(-H "Authorization: Bearer $GRAFANA_LAB_ADMIN_TOKEN")
  local P=/api/alertmanager/grafana/api/v2 body id general
  FAILED=0

  AUTH=("${SC[@]}")
  body=$(api GET "$P/alerts?active=true")
  check V1 200 "$(<"$SF")" "$(jq -r '[.[]|select(.labels.__alert_rule_uid__)]|length' <<<"$body") alerts carry __alert_rule_uid__"

  body=$(api POST "$P/silences" "$(silence_json __alert_rule_uid__ lab-rule-child)")
  id=$(jq -r '.silenceID // empty' <<<"$body" 2>/dev/null || true)
  check V2a '2[0-9][0-9]' "$(<"$SF")" "rule silence in permitted subfolder (id=${id:-none})"
  [ -n "$id" ] && CREATED+=("$id")

  body=$(api POST "$P/silences" "$(silence_json alertname 'Lab child rule')")
  check V2b 403 "$(<"$SF")" "general silence refused"
  x=$(jq -r '.silenceID // empty' <<<"$body" 2>/dev/null || true); [ -n "$x" ] && CREATED+=("$x")

  body=$(api POST "$P/silences" "$(silence_json __alert_rule_uid__ lab-rule-other)")
  check V2c 403 "$(<"$SF")" "rule silence in other folder refused"

  if [ -n "$id" ]; then
    api POST "$P/silences" "$(silence_json __alert_rule_uid__ lab-rule-child "$id")" >/dev/null
    check V3a '2[0-9][0-9]' "$(<"$SF")" "update of own rule silence"
  else
    check V3a 2xx skipped "no V2a silence"
  fi

  AUTH=("${AD[@]}")
  general=$(api POST "$P/silences" "$(silence_json alertname 'Lab other rule')" | jq -r .silenceID)
  CREATED+=("$general")
  AUTH=("${SC[@]}")
  api DELETE "$P/silence/$general" >/dev/null
  check V3b 403 "$(<"$SF")" "expire of general silence refused"

  body=$(api GET /api/access-control/user/permissions)
  local scopes; scopes=$(jq -r '."alert.silences:create" // [] | join(",")' <<<"$body")
  if [[ ,$scopes, == *,folders:uid:lab-parent,* && ,$scopes, != *,folders:uid:lab-child,* ]]; then
    check V4 "$(<"$SF")" "$(<"$SF")" "create scopes list lab-parent, not lab-child [$scopes]"
  else
    check V4 listed-parent-only "$(<"$SF")" "create scopes [$scopes]"
  fi

  AUTH=("${AD[@]}")
  for s in "${CREATED[@]}"; do api DELETE "$P/silence/$s" >/dev/null || true; done
  return $FAILED
}

case "${1:-}" in
  up) up ;;
  verify) verify ;;
  env) cat "$ENV_FILE" ;;
  down) docker rm -f "$NAME" >/dev/null && rm -f "$ENV_FILE" ;;
  *) echo "usage: $0 up|verify|env|down" >&2; exit 2 ;;
esac
