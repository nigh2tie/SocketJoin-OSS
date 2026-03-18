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
    /** @type {any[]} */
    export let rankingData = [];
    /** @type {boolean} */
    export let show = false;
    /** @type {() => void} */
    export let onClose = () => {};
</script>

{#if show}
<div class="bg-white p-6 rounded-xl shadow-sm border border-yellow-200">
    <div class="flex items-center justify-between mb-4">
        <h3 class="text-lg font-bold text-gray-900">クイズランキング</h3>
        <button on:click={onClose} class="text-gray-400 hover:text-gray-600 text-xl">✕</button>
    </div>
    {#if rankingData.length === 0}
        <p class="text-gray-500 text-center py-4">まだ得点データがありません。</p>
    {:else}
        <ol class="space-y-2">
            {#each rankingData as entry}
                <li class="flex items-center gap-3 p-3 rounded-lg {entry.rank === 1 ? 'bg-yellow-50 border border-yellow-200' : 'bg-gray-50'}">
                    <span class="w-8 h-8 flex items-center justify-center rounded-full font-bold text-sm {entry.rank === 1 ? 'bg-yellow-400 text-white' : entry.rank === 2 ? 'bg-gray-300 text-gray-700' : entry.rank === 3 ? 'bg-orange-300 text-white' : 'bg-gray-100 text-gray-600'}">
                        {entry.rank}
                    </span>
                    <span class="flex-1 font-medium">{entry.nickname}</span>
                    <span class="font-bold text-blue-700">{entry.total_score}pt</span>
                </li>
            {/each}
        </ol>
    {/if}
</div>
{/if}
