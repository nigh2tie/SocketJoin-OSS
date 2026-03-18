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
    export let polls = [];
    /** @type {string | null} */
    export let activePollId = null;
    /** @type {string} */
    export let role = 'host';
    /** @type {string} */
    export let csrfToken;
    /** @type {string} */
    export let eventId;
    /** @type {(msg: string, type?: 'success'|'error') => void} */
    export let showToast;
    /** @type {(message: string, onConfirm: function) => void} */
    export let requestConfirm;
    /** @type {() => void} */
    export let onFetchRanking;
    /** @type {(pollId: string | null) => void} */
    export let onActivePollChange;

    /** @param {string} pollId */
    async function activatePoll(pollId) {
        const res = await fetch(`/api/events/${eventId}/active_poll`, {
            method: 'PUT',
            body: JSON.stringify({ poll_id: pollId }),
            headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': csrfToken }
        });
        if (res.ok) {
            onActivePollChange(pollId);
        }
    }

    /** @param {string} pollId */
    function closePoll(pollId) {
        requestConfirm('この投票を締め切りますか？', async () => {
            const res = await fetch(`/api/events/${eventId}/polls/${pollId}/close`, {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': csrfToken }
            });
            if (res.ok) {
                if (activePollId === pollId) onActivePollChange(null);
                const idx = polls.findIndex(p => p.id === pollId);
                if (idx !== -1) {
                    polls[idx].status = 'closed';
                    polls = [...polls];
                }
                const closedPoll = polls.find(p => p.id === pollId);
                if (closedPoll?.is_quiz) {
                    onFetchRanking();
                }
                showToast('締め切りました。');
            } else {
                showToast('締め切りに失敗しました。', 'error');
            }
        });
    }

    /** @param {string} pollId */
    function resetPoll(pollId) {
        requestConfirm('この投票の集計をリセットしますか？\n（参加者は再度投票できるようになります。クイズ得点は巻き戻りません。）', async () => {
            const res = await fetch(`/api/events/${eventId}/polls/${pollId}/reset`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': csrfToken }
            });
            if (res.ok) {
                const idx = polls.findIndex(p => p.id === pollId);
                if (idx !== -1) {
                    polls[idx].status = 'open';
                    polls = [...polls];
                }
                showToast('リセットしました。');
            } else {
                showToast('リセットに失敗しました。', 'error');
            }
        });
    }
</script>

<div class="bg-white p-6 rounded-xl shadow-sm border border-gray-200">
    <h3 class="text-lg font-bold mb-4 text-gray-900 border-b pb-2">投票一覧</h3>
    {#if polls.length === 0}
        <p class="text-gray-500 text-center py-6">まだ投票がありません。</p>
    {:else}
        <ul class="space-y-4">
            {#each polls as poll}
                <li class={`flex flex-col p-5 border rounded-2xl transition-all duration-300 ${activePollId === poll.id ? (poll.is_quiz ? 'bg-purple-50 border-purple-300 shadow-md ring-1 ring-purple-200' : 'bg-blue-50 border-blue-300 shadow-md ring-1 ring-blue-200') : 'bg-white border-gray-200 hover:border-gray-300 hover:shadow-sm'}`}>
                    <div class="flex justify-between items-start w-full flex-wrap gap-4">
                        <div class="flex-1">
                            <div class="flex items-center gap-2 mb-2">
                                {#if poll.is_quiz}
                                    <span class="flex items-center gap-1 text-[10px] font-black uppercase tracking-widest bg-purple-600 text-white px-2 py-0.5 rounded shadow-sm">
                                        QUIZ
                                    </span>
                                    {#if poll.points}
                                        <span class="text-[10px] font-black text-purple-600 bg-purple-100 px-2 py-0.5 rounded border border-purple-200">
                                            +{poll.points} PT
                                        </span>
                                    {/if}
                                {:else}
                                    <span class="flex items-center gap-1 text-[10px] font-black uppercase tracking-widest bg-blue-600 text-white px-2 py-0.5 rounded shadow-sm">
                                        SURVEY
                                    </span>
                                {/if}
                                {#if poll.max_selections > 1}
                                    <span class="text-[10px] font-bold text-gray-500 bg-gray-100 px-2 py-0.5 rounded uppercase">
                                        Multi: {poll.max_selections}
                                    </span>
                                {/if}
                                {#if poll.status === 'closed'}
                                    <span class="text-[10px] font-bold text-red-500 bg-red-50 px-2 py-0.5 rounded uppercase border border-red-100">Closed</span>
                                {/if}
                            </div>
                            <h4 class={`text-lg font-bold leading-tight ${activePollId === poll.id ? (poll.is_quiz ? 'text-purple-900' : 'text-blue-900') : 'text-gray-900'}`}>
                                {poll.title}
                            </h4>
                        </div>

                        <div class="flex items-center gap-2">
                            {#if activePollId === poll.id}
                                <div class="flex items-center gap-2">
                                    <span class={`flex items-center gap-1.5 px-3 py-1.5 rounded-full text-xs font-black uppercase tracking-tight text-white animate-pulse ${poll.is_quiz ? 'bg-purple-500' : 'bg-green-500'}`}>
                                        <span class="w-1.5 h-1.5 bg-white rounded-full"></span>
                                        Live Now
                                    </span>
                                    <button class="px-4 py-1.5 text-xs font-bold bg-red-500 text-white hover:bg-red-600 rounded-full shadow-lg transition-transform hover:scale-105" on:click={() => closePoll(poll.id)}>締め切る</button>
                                </div>
                            {:else if poll.status !== 'closed'}
                                <button class={`px-6 py-1.5 text-xs font-bold text-white rounded-full shadow-md transition-all hover:scale-105 ${poll.is_quiz ? 'bg-purple-600 hover:bg-purple-700' : 'bg-blue-600 hover:bg-blue-700'}`} on:click={() => activatePoll(poll.id)}>開始</button>
                            {/if}
                            {#if role === 'host'}
                                <button class="p-1.5 text-gray-400 hover:text-orange-600 transition-colors" type="button" title="リセット" on:click|preventDefault|stopPropagation={() => resetPoll(poll.id)}>
                                    <svg xmlns="http://www.w3.org/2000/svg" class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
                                    </svg>
                                </button>
                            {/if}
                            {#if poll.is_quiz && poll.status === 'closed'}
                                <button class="px-3 py-1.5 text-xs font-bold bg-yellow-400 text-gray-900 hover:bg-yellow-500 rounded-full shadow-sm transition-all" on:click={onFetchRanking}>ランキング</button>
                            {/if}
                        </div>
                    </div>
                </li>
            {/each}
        </ul>
    {/if}
</div>
