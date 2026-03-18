<!--
SocketJoin: Real-time event interaction platform.
Copyright (C) 2026 Q-Q

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <https://www.gnu.org/licenses/>.
-->

<script>
    /** @type {string} */
    export let message = '';
    /** @type {'success' | 'error'} */
    export let type = 'success';
    /** @type {() => void} */
    export let onClose = () => {};
</script>

{#if message}
<div class="fixed top-6 right-6 z-[5000] animate-fade-in-up">
    <div class={`px-4 py-3 rounded-xl shadow-xl border flex items-center gap-3 min-w-[300px] ${type === 'error' ? 'bg-red-50 border-red-200 text-red-800' : 'bg-green-50 border-green-200 text-green-800'}`}>
        <div class={`w-2 h-2 rounded-full ${type === 'error' ? 'bg-red-500' : 'bg-green-500'}`}></div>
        <p class="font-bold text-sm whitespace-pre-wrap">{message}</p>
        <button on:click={onClose} class="ml-auto text-gray-400 hover:text-gray-600 font-bold p-1">✕</button>
    </div>
</div>
{/if}
