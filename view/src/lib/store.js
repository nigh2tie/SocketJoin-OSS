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

/**
 * @typedef {Object} Event
 * @property {string} id
 * @property {string} title
 * @property {string} status
 * @property {string|null} current_poll_id
 */

/** @type {import('svelte/store').Writable<Event|null>} */
export const event = writable(null);

/** @type {import('svelte/store').Writable<string>} */
export const nickname = writable('');

/**
 * @typedef {Object} Poll
 * @property {string} id
 * @property {string} event_id
 * @property {string} title
 * @property {string} status
 * @property {boolean} is_quiz
 * @property {number} points
 * @property {number} max_selections
 * @property {Object[]} options
 * @property {string} options.id
 * @property {string} options.label
 * @property {number} options.order
 * @property {boolean} options.is_correct
 */

/** @type {import('svelte/store').Writable<Poll|null>} */
export const poll = writable(null);

/** @type {import('svelte/store').Writable<Record<string, number>>} */
export const votes = writable({});

/** @type {import('svelte/store').Writable<string>} */
export const connectionStatus = writable('disconnected');

/** @type {import('svelte/store').Writable<string|null>} */
export const error = writable(null);

/**
 * @typedef {Object} RankingEntry
 * @property {number} rank
 * @property {string} nickname
 * @property {number} total_score
 */

/** @type {import('svelte/store').Writable<RankingEntry[]>} */
export const ranking = writable([]);

/**
 * @typedef {Object} Question
 * @property {string} id
 * @property {string} event_id
 * @property {string} visitor_id
 * @property {string} content
 * @property {string} status
 * @property {string} created_at
 * @property {number} upvotes
 * @property {boolean} is_upvoted
 */

/** @type {import('svelte/store').Writable<Question[]>} */
export const questions = writable([]);
