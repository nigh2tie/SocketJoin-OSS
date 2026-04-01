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
    import { questions } from '$lib/store';
    import UpvoteIcon from '$lib/components/UpvoteIcon.svelte';

    /** @type {string} */
    export let eventId;
    /** @type {string} */
    export let csrfToken;
    /** @type {(msg: string, type?: 'success'|'error') => void} */
    export let showToast;

    /**
     * @param {string} qid
     * @param {string} status
     */
    async function updateQuestionStatus(qid, status) {
        const res = await fetch(`/api/events/${eventId}/questions/${qid}/status`, {
            method: 'PATCH',
            headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': csrfToken },
            body: JSON.stringify({ status })
        });
        if (res.ok) {
            showToast('ステータスを更新しました。');
        } else {
            showToast('ステータスの更新に失敗しました', 'error');
        }
    }
</script>

<div class="bg-white p-6 rounded-xl shadow-sm border border-gray-200">
    <h3 class="text-lg font-bold mb-4 text-gray-900 border-b pb-2">Q&A管理</h3>
    {#if $questions.length === 0}
        <p class="text-gray-500 text-center py-6">まだ質問がありません。</p>
    {:else}
        <ul class="space-y-3">
            {#each $questions as q}
                <li class="flex flex-col p-4 bg-gray-50 border border-gray-200 rounded-xl relative">
                    <div class="flex justify-between items-start gap-4">
                        <div class="flex-1 min-w-0">
                            <p class="text-gray-800 whitespace-pre-wrap break-words mb-2">{q.content}</p>
                            <div class="text-xs text-gray-500 flex gap-4">
                                <span class="font-semibold text-blue-600 flex items-center gap-1">
                                    <UpvoteIcon className="h-4 w-4" size="1rem" />
                                    {q.upvotes}
                                </span>
                                <span>{new Date(q.created_at).toLocaleString()}</span>
                                <span class={q.status === 'answered' ? 'text-green-600 font-bold' : q.status === 'archived' ? 'text-gray-400 font-bold' : 'text-orange-500 font-bold'}>{q.status === 'active' ? '回答待ち' : q.status === 'answered' ? '回答済み' : 'アーカイブ'}</span>
                            </div>
                        </div>
                        <div class="flex flex-col gap-2 min-w-[120px]">
                            {#if q.status === 'active'}
                                <button class="px-3 py-1.5 text-xs bg-green-100 text-green-700 hover:bg-green-200 rounded shadow-sm transition" on:click={() => updateQuestionStatus(q.id, 'answered')}>回答済みにする</button>
                            {/if}
                            {#if q.status !== 'archived'}
                                <button class="px-3 py-1.5 text-xs bg-gray-200 text-gray-700 hover:bg-gray-300 rounded shadow-sm transition" on:click={() => updateQuestionStatus(q.id, 'archived')}>アーカイブ</button>
                            {/if}
                            {#if q.status !== 'active'}
                                <button class="px-3 py-1.5 text-xs bg-blue-50 text-blue-700 hover:bg-blue-100 rounded shadow-sm transition" on:click={() => updateQuestionStatus(q.id, 'active')}>未回答に戻す</button>
                            {/if}
                        </div>
                    </div>
                </li>
            {/each}
        </ul>
    {/if}
</div>
