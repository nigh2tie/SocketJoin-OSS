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

import { writable } from 'svelte/store';

// Initialize stores with values from localStorage if available (only in browser)
const isBrowser = typeof window !== 'undefined';
/** @type {string[]} */
const initialVotedPolls = isBrowser ? JSON.parse(localStorage.getItem('voted_polls') || '[]') : [];

export const votedPollsStore = writable(initialVotedPolls);

/**
 * @param {string} pollId
 */
export function markPollAsVoted(pollId) {
    votedPollsStore.update(polls => {
        if (!polls.includes(pollId)) {
            const newPolls = [...polls, pollId];
            if (isBrowser) localStorage.setItem('voted_polls', JSON.stringify(newPolls));
            return newPolls;
        }
        return polls;
    });
}

/**
 * @param {string} pollId
 */
export function clearPollVotedState(pollId) {
    votedPollsStore.update(polls => {
        if (!polls.includes(pollId)) {
            return polls;
        }
        const newPolls = polls.filter(id => id !== pollId);
        if (isBrowser) localStorage.setItem('voted_polls', JSON.stringify(newPolls));
        return newPolls;
    });
}

/**
 * @param {string} pollId
 * @param {string[]} polls
 * @returns {boolean}
 */
export function hasVotedForPoll(pollId, polls) {
    return polls.includes(pollId);
}

/**
 * @param {string} name
 */
export function saveNickname(name) {
    if (isBrowser) localStorage.setItem('nickname', name);
}

/**
 * @returns {string|null}
 */
export function getSavedNickname() {
    return isBrowser ? localStorage.getItem('nickname') : null;
}
