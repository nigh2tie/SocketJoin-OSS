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

/** @type {import('./$types').PageLoad} */
export const load = async ({ fetch, params }) => {
    try {
        const res = await fetch(`/api/events/${params.id}`);
        if (res.ok) {
            const event = await res.json();
            
            // Initial data: if poll active, connection established in svelte
            return { event };
        }
        return { error: 'Event not found' };
    } catch (/** @type {any} */ e) {
        return { error: e.message };
    }
};
