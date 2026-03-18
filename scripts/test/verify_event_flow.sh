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

# イベントフロー統合テスト。
# 実行前に Docker スタックを起動しておくこと: make up
set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:3000}"

COOKIE_JAR=$(mktemp)
trap "rm -f $COOKIE_JAR" EXIT

PASS=0
FAIL=0

# ── ヘルパー ─────────────────────────────────────────────────────────────────

ok() {
  echo "  ✓ $1"
  PASS=$((PASS + 1))
}

fail() {
  echo "  ✗ $1"
  FAIL=$((FAIL + 1))
}

assert_contains() {
  local label="$1" body="$2" expected="$3"
  if [[ "$body" == *"$expected"* ]]; then
    ok "$label"
  else
    fail "$label (期待値: '$expected' が含まれない)"
    echo "    レスポンス: $body"
  fi
}

assert_not_contains() {
  local label="$1" body="$2" unexpected="$3"
  if [[ "$body" != *"$unexpected"* ]]; then
    ok "$label"
  else
    fail "$label ('$unexpected' が含まれてはいけない)"
    echo "    レスポンス: $body"
  fi
}

assert_status() {
  local label="$1" actual="$2" expected="$3"
  if [ "$actual" -eq "$expected" ]; then
    ok "$label (HTTP $actual)"
  else
    fail "$label (期待: HTTP $expected, 実際: HTTP $actual)"
  fi
}

# ── セットアップ ──────────────────────────────────────────────────────────────

echo ""
echo "=== セットアップ ==="

echo "0. CSRF トークン取得..."
curl -s -c "$COOKIE_JAR" "$BASE_URL/api/csrf" > /dev/null
CSRF_TOKEN=$(grep 'csrf_token' "$COOKIE_JAR" | awk '{print $NF}')
if [ -z "$CSRF_TOKEN" ]; then
  echo "FATAL: CSRF トークンの取得に失敗しました"
  exit 1
fi
ok "CSRF トークン取得: $CSRF_TOKEN"

echo "1. イベント作成..."
EVENT_RESP=$(curl -s -X POST "$BASE_URL/api/events" \
  -H "Content-Type: application/json" \
  -H "X-CSRF-Token: $CSRF_TOKEN" \
  -b "$COOKIE_JAR" -c "$COOKIE_JAR" \
  -d '{"title":"Test Event","nickname_policy":"optional"}')
EVENT_ID=$(echo "$EVENT_RESP" | grep -o '"id":"[^"]*"' | head -n 1 | cut -d'"' -f4)
if [ -z "$EVENT_ID" ]; then
  echo "FATAL: イベントの作成に失敗しました"
  exit 1
fi
ok "イベント作成: $EVENT_ID"

echo "2. Poll 作成（Cookie認証）..."
POLL_RESP=$(curl -s -X POST "$BASE_URL/api/events/$EVENT_ID/polls" \
  -H "Content-Type: application/json" \
  -H "X-CSRF-Token: $CSRF_TOKEN" \
  -b "$COOKIE_JAR" -c "$COOKIE_JAR" \
  -d '{"title":"Test Poll","options":[{"label":"A","is_correct":false},{"label":"B","is_correct":true}]}')
POLL_ID=$(echo "$POLL_RESP" | grep -o '"id":"[^"]*"' | head -n 1 | cut -d'"' -f4)
OPTION_A_ID=$(echo "$POLL_RESP" | grep -o '"id":"[^"]*"' | sed -n '2p' | cut -d'"' -f4)
if [ -z "$POLL_ID" ] || [ -z "$OPTION_A_ID" ]; then
  echo "FATAL: Poll の作成に失敗しました"
  exit 1
fi
ok "Poll 作成: $POLL_ID"

echo "3. アクティブ Poll を切り替え（Cookie認証）..."
SWITCH_STATUS=$(curl -s -o /dev/null -w "%{http_code}" -X PUT "$BASE_URL/api/events/$EVENT_ID/active_poll" \
  -H "Content-Type: application/json" \
  -H "X-CSRF-Token: $CSRF_TOKEN" \
  -b "$COOKIE_JAR" -c "$COOKIE_JAR" \
  -d "{\"poll_id\":\"$POLL_ID\"}")
assert_status "アクティブ Poll 切り替え" "$SWITCH_STATUS" 200

# ── 正常系 ────────────────────────────────────────────────────────────────────

echo ""
echo "=== 正常系 ==="

echo "4. owner_token がレスポンスに含まれないこと..."
assert_not_contains "owner_token が除外されている (CREATE)" "$EVENT_RESP" '"owner_token"'
GET_EVENT_RESP=$(curl -s -b "$COOKIE_JAR" "$BASE_URL/api/events/$EVENT_ID")
assert_contains  "current_poll_id が反映されている" "$GET_EVENT_RESP" "$POLL_ID"
assert_not_contains "owner_token が除外されている (GET)" "$GET_EVENT_RESP" '"owner_token"'

echo "5. 投票..."
VOTE_STATUS=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE_URL/api/poll/$POLL_ID/vote" \
  -H "Content-Type: application/json" \
  -H "X-CSRF-Token: $CSRF_TOKEN" \
  -b "$COOKIE_JAR" -c "$COOKIE_JAR" \
  -d "{\"option_id\":\"$OPTION_A_ID\",\"nickname\":\"Alice\"}")
assert_status "投票成功" "$VOTE_STATUS" 200

echo "6. 投票結果確認..."
RESULT_RESP=$(curl -s -b "$COOKIE_JAR" "$BASE_URL/api/poll/$POLL_ID/result")
assert_contains "投票結果に option_id が含まれる" "$RESULT_RESP" "$OPTION_A_ID"

echo "7. Poll 締め切り（Cookie認証）..."
CLOSE_STATUS=$(curl -s -o /dev/null -w "%{http_code}" -X PUT "$BASE_URL/api/events/$EVENT_ID/polls/$POLL_ID/close" \
  -H "Content-Type: application/json" \
  -H "X-CSRF-Token: $CSRF_TOKEN" \
  -b "$COOKIE_JAR" -c "$COOKIE_JAR")
assert_status "Poll 締め切り成功" "$CLOSE_STATUS" 200

echo "8. Poll 詳細で status が closed になっていること..."
GET_POLL_RESP=$(curl -s -b "$COOKIE_JAR" "$BASE_URL/api/poll/$POLL_ID")
assert_contains "Poll status が closed" "$GET_POLL_RESP" '"status":"closed"'

echo "9. Embed トークン発行（Cookie認証）..."
EMBED_RESP=$(curl -s -X POST "$BASE_URL/api/events/$EVENT_ID/embed_token" \
  -H "Content-Type: application/json" \
  -H "X-CSRF-Token: $CSRF_TOKEN" \
  -b "$COOKIE_JAR" -c "$COOKIE_JAR" \
  -d '{"allowed_origins":["https://example.com"]}')
assert_contains "Embed トークン発行成功" "$EMBED_RESP" '"token"'

# ── 異常系 ────────────────────────────────────────────────────────────────────

echo ""
echo "=== 異常系 ==="

echo "10. CSRF トークンなしで POST → 拒否されること..."
NO_CSRF_STATUS=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE_URL/api/events" \
  -H "Content-Type: application/json" \
  -b "$COOKIE_JAR" \
  -d '{"title":"Should Fail"}')
assert_status "CSRF なしで拒否" "$NO_CSRF_STATUS" 403

echo "11. オーナー Cookie なしで Poll 作成 → 拒否されること..."
NO_OWNER_JAR=$(mktemp)
curl -s -c "$NO_OWNER_JAR" "$BASE_URL/api/csrf" > /dev/null
NO_OWNER_CSRF=$(grep 'csrf_token' "$NO_OWNER_JAR" | awk '{print $NF}')
BAD_COOKIE_STATUS=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE_URL/api/events/$EVENT_ID/polls" \
  -H "Content-Type: application/json" \
  -H "X-CSRF-Token: $NO_OWNER_CSRF" \
  -b "$NO_OWNER_JAR" -c "$NO_OWNER_JAR" \
  -d '{"title":"Bad Poll","options":[{"label":"X","is_correct":false}]}')
rm -f "$NO_OWNER_JAR"
assert_status "オーナーCookieなしで拒否" "$BAD_COOKIE_STATUS" 401

echo "12. 同一 visitor_id での 2 重投票 → 拒否されること..."
DOUBLE_VOTE_STATUS=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE_URL/api/poll/$POLL_ID/vote" \
  -H "Content-Type: application/json" \
  -H "X-CSRF-Token: $CSRF_TOKEN" \
  -b "$COOKIE_JAR" -c "$COOKIE_JAR" \
  -d "{\"option_id\":\"$OPTION_A_ID\",\"nickname\":\"Alice\"}")
assert_status "2 重投票で拒否" "$DOUBLE_VOTE_STATUS" 409

echo "13. 締め切り済み Poll への投票 → 拒否されること..."
ANOTHER_JAR=$(mktemp)
curl -s -c "$ANOTHER_JAR" "$BASE_URL/api/csrf" > /dev/null
ANOTHER_CSRF=$(grep 'csrf_token' "$ANOTHER_JAR" | awk '{print $NF}')
CLOSED_VOTE_STATUS=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE_URL/api/poll/$POLL_ID/vote" \
  -H "Content-Type: application/json" \
  -H "X-CSRF-Token: $ANOTHER_CSRF" \
  -b "$ANOTHER_JAR" -c "$ANOTHER_JAR" \
  -d "{\"option_id\":\"$OPTION_A_ID\",\"nickname\":\"Bob\"}")
rm -f "$ANOTHER_JAR"
assert_status "締め切り済みへの投票で拒否" "$CLOSED_VOTE_STATUS" 409

echo "14. オーナー Cookie なしで Embed トークン発行 → 拒否されること..."
NO_EMBED_JAR=$(mktemp)
curl -s -c "$NO_EMBED_JAR" "$BASE_URL/api/csrf" > /dev/null
NO_EMBED_CSRF=$(grep 'csrf_token' "$NO_EMBED_JAR" | awk '{print $NF}')
BAD_EMBED_STATUS=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE_URL/api/events/$EVENT_ID/embed_token" \
  -H "Content-Type: application/json" \
  -H "X-CSRF-Token: $NO_EMBED_CSRF" \
  -b "$NO_EMBED_JAR" -c "$NO_EMBED_JAR" \
  -d '{"allowed_origins":["https://example.com"]}')
rm -f "$NO_EMBED_JAR"
assert_status "オーナーCookieなしでEmbed拒否" "$BAD_EMBED_STATUS" 401

# ── 結果サマリー ──────────────────────────────────────────────────────────────

echo ""
echo "=== 結果 ==="
echo "PASS: $PASS  FAIL: $FAIL"
echo ""

if [ "$FAIL" -gt 0 ]; then
  exit 1
fi
