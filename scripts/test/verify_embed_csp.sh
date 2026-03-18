#!/bin/bash
# SocketJoin: Real-time event interaction platform.
# Copyright (C) 2026 Q-Q
#
# This program is free software: you can redistribute it and/or modify
# it under the terms of the GNU Affero General Public License as published by
# the Free Software Foundation, either version 3 of the License, or
# (at your option) any later version.
#
# This program is distributed in the hope that it will be useful,
# but WITHOUT ANY WARRANTY; without even the implied warranty of
# MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
# GNU Affero General Public License for more details.
#
# You should have received a copy of the GNU Affero General Public License
# along with this program.  If not, see <https://www.gnu.org/licenses/>.

set -euo pipefail

APP_URL="${APP_URL:-http://localhost:3000}"
DIRECT_URL="${DIRECT_URL:-http://localhost:8080}"
COOKIE_JAR=$(mktemp)
HEADER_8080=$(mktemp)
HEADER_3000=$(mktemp)
trap 'rm -f "$COOKIE_JAR" "$HEADER_8080" "$HEADER_3000"' EXIT

CSRF_RES=$(curl -s -c "$COOKIE_JAR" "$APP_URL/api/csrf")
CSRF_TOKEN=$(echo "$CSRF_RES" | grep -o '"token":"[^"]*' | cut -d'"' -f4)

EVT_RES=$(curl -s -b "$COOKIE_JAR" -c "$COOKIE_JAR" -X POST -H 'Content-Type: application/json' -H "X-CSRF-Token: $CSRF_TOKEN" -d '{"title":"CSP Test Event"}' "$APP_URL/api/events")
EVT_ID=$(echo "$EVT_RES" | grep -o '"id":"[^"]*' | cut -d'"' -f4)

curl -s -b "$COOKIE_JAR" -c "$COOKIE_JAR" -X POST -H 'Content-Type: application/json' -H "X-CSRF-Token: $CSRF_TOKEN" -d '{"title":"Test Poll","is_quiz":false,"options":[{"label":"A","is_correct":false}]}' "$APP_URL/api/events/${EVT_ID}/polls" > /dev/null

TOKEN_RES=$(curl -s -b "$COOKIE_JAR" -c "$COOKIE_JAR" -X POST -H 'Content-Type: application/json' -H "X-CSRF-Token: $CSRF_TOKEN" -d "{\"allowed_origins\":[\"${DIRECT_URL}\"]}" "$APP_URL/api/events/${EVT_ID}/embed_token")
EMBED_TOKEN=$(echo "$TOKEN_RES" | grep -o '"token":"[^"]*' | cut -d'"' -f4)

echo "Testing curl to API DIRECTLY (8080):"
curl -s -D "$HEADER_8080" -b "$COOKIE_JAR" "${DIRECT_URL}/embed/join/${EVT_ID}?token=${EMBED_TOKEN}" > /dev/null
cat "$HEADER_8080"

echo "Testing curl to NGINX (3000):"
curl -s -D "$HEADER_3000" -b "$COOKIE_JAR" "${APP_URL}/embed/join/${EVT_ID}?token=${EMBED_TOKEN}" > /dev/null
cat "$HEADER_3000"
