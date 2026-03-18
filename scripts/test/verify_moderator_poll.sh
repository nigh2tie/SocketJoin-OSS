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

BASE_URL="${BASE_URL:-http://localhost:3000}"
COOKIE_JAR=$(mktemp)
trap 'rm -f "$COOKIE_JAR" "$COOKIE_JAR.bak"' EXIT

echo "1. Get CSRF Token"
CSRF_RES=$(curl -s -c "$COOKIE_JAR" "$BASE_URL/api/csrf")
CSRF_TOKEN=$(echo "$CSRF_RES" | grep -o '"token":"[^"]*' | cut -d'"' -f4)

echo "2. Create Event"
EVT_RES=$(curl -s -b "$COOKIE_JAR" -c "$COOKIE_JAR" -X POST -H 'Content-Type: application/json' -H "X-CSRF-Token: $CSRF_TOKEN" -d '{"title":"Test Event"}' "$BASE_URL/api/events")
EVT_ID=$(echo "$EVT_RES" | grep -o '"id":"[^"]*' | cut -d'"' -f4)

echo "Event ID: $EVT_ID"

echo "3. Add Moderator"
MOD_RES=$(curl -s -b "$COOKIE_JAR" -c "$COOKIE_JAR" -X POST -H 'Content-Type: application/json' -H "X-CSRF-Token: $CSRF_TOKEN" -d '{"name":"TestMod"}' "$BASE_URL/api/events/${EVT_ID}/moderators")
MOD_TOKEN=$(echo "$MOD_RES" | grep -o '"token":"[^"]*' | cut -d'"' -f4)

echo "Mod Token: $MOD_TOKEN"

echo "4. Create Poll"
POLL_RES=$(curl -s -b "$COOKIE_JAR" -c "$COOKIE_JAR" -X POST -H 'Content-Type: application/json' -H "X-CSRF-Token: $CSRF_TOKEN" -d '{"title":"Test Poll","is_quiz":false,"points":0,"max_selections":1,"options":[{"label":"A","is_correct":false}]}' "$BASE_URL/api/events/${EVT_ID}/polls")
POLL_ID=$(echo "$POLL_RES" | grep -o '"id":"[^"]*' | cut -d'"' -f4)

echo "Poll ID: $POLL_ID"

echo "5. Clear owner cookie, Mod Login"
sed -i.bak "/owner_${EVT_ID}/d" "$COOKIE_JAR"
LOGIN_RES=$(curl -s -v -b "$COOKIE_JAR" -c "$COOKIE_JAR" -X POST -H 'Content-Type: application/json' -H "X-CSRF-Token: $CSRF_TOKEN" -d "{\"token\":\"${MOD_TOKEN}\"}" "$BASE_URL/api/moderator/login" 2>&1)
echo "$LOGIN_RES" | grep 'set-cookie' || true

echo "6. Mod tries to switch poll"
curl -s -v -b "$COOKIE_JAR" -X PUT -H 'Content-Type: application/json' -H "X-CSRF-Token: $CSRF_TOKEN" -d "{\"poll_id\":\"${POLL_ID}\"}" "$BASE_URL/api/events/${EVT_ID}/active_poll"
