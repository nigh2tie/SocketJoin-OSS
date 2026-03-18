// SocketJoin: Real-time event interaction platform.
// Copyright (C) 2026 Q-Q
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

/**
 * @returns {string}
 */
export function getCsrfToken() {
    const match = document.cookie.match(new RegExp('(^| )csrf_token=([^;]+)'));
    if (match) return match[2];
    return '';
}

/**
 * @param {string} eventId
 * @returns {Promise<any[]>}
 */
export async function fetchEventHistory(eventId) {
    try {
        const res = await fetch(`/api/events/${eventId}/history`);
        if (res.ok) {
            return await res.json();
        }
    } catch (err) {
        console.error('Failed to load history', err);
    }
    return [];
}

/**
 * @param {string} pollId
 * @param {string[]} optionIds
 * @param {string} nickname
 * @returns {Promise<boolean>}
 */
export async function submitPollVote(pollId, optionIds, nickname) {
    const res = await fetch(`/api/poll/${pollId}/vote`, {
        method: 'POST',
        body: JSON.stringify({
            option_ids: optionIds,
            nickname: nickname
        }),
        headers: {
            'Content-Type': 'application/json',
            'X-CSRF-Token': getCsrfToken()
        }
    });

    if (!res.ok) {
        const txt = await res.text();
        if (txt.includes('banned')) {
            throw new Error('accessed_denied');
        } else if (txt.includes('already voted')) {
            throw new Error('already_voted');
        } else {
            throw new Error('unknown_error');
        }
    }
    return true;
}

/**
 * @param {string} eventId
 * @param {string} content
 * @returns {Promise<boolean>}
 */
export async function submitQuestionData(eventId, content) {
    const res = await fetch(`/api/events/${eventId}/questions`, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
            'X-CSRF-Token': getCsrfToken()
        },
        body: JSON.stringify({ content: content })
    });

    if (!res.ok) {
        const txt = await res.text();
        throw new Error(txt || '投稿に失敗しました');
    }
    return true;
}

/**
 * @param {string} eventId
 * @param {string} qid
 */
export async function toggleUpvoteData(eventId, qid) {
    await fetch(`/api/events/${eventId}/questions/${qid}/upvote`, {
        method: 'POST',
        headers: {
            'X-CSRF-Token': getCsrfToken()
        }
    });
}

