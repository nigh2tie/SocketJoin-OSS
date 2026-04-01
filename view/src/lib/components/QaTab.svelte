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
    import { submitQuestionData, toggleUpvoteData } from '$lib/api';
    import UpvoteIcon from '$lib/components/UpvoteIcon.svelte';

    /** @type {string} */
    export let eventId;

    let newQuestionContent = '';
    let questionError = '';

    async function submitQuestion() {
        if (!newQuestionContent.trim()) return;
        questionError = '';
        try {
            await submitQuestionData(eventId, newQuestionContent.trim());
            newQuestionContent = '';
        } catch (err) {
            if (err instanceof Error) {
                questionError = err.message || '通信エラーが発生しました';
            } else {
                questionError = '通信エラーが発生しました';
            }
        }
    }

    /** @param {string} qid */
    async function toggleUpvote(qid) {
        try {
            await toggleUpvoteData(eventId, qid);
        } catch (e) {
            console.error('Failed to toggle upvote', e);
        }
    }
</script>

<div class="qa-view card">
    <h2>Q&A (質問と回答)</h2>
    <div class="qa-input-area">
        <textarea bind:value={newQuestionContent} placeholder="質問を入力してください..." rows="3"></textarea>
        <button class="submit-question-btn" on:click={submitQuestion} disabled={!newQuestionContent.trim()}>
            質問を投稿する
        </button>
        {#if questionError}
            <p class="error">{questionError}</p>
        {/if}
    </div>

    <div class="qa-list">
        {#if $questions.length === 0}
            <p class="empty-msg">まだ質問がありません。</p>
        {:else}
            {#each $questions as q (q.id)}
                <div class="qa-item">
                    <div class="qa-content">{q.content}</div>
                    <div class="qa-meta">
                        <span class="qa-status {q.status}">{q.status === 'answered' ? '回答済み' : '受付中'}</span>
                        <span class="qa-date">{new Date(q.created_at).toLocaleString()}</span>
                    </div>
                    <button class="upvote-btn" class:upvoted={q.is_upvoted} on:click={() => toggleUpvote(q.id)}>
                        <UpvoteIcon size="1.1rem" /> {q.upvotes}
                    </button>
                </div>
            {/each}
        {/if}
    </div>
</div>

<style>
    .card { background: #fff; padding: 30px 20px; text-align: center; border-radius: 12px; box-shadow: 0 4px 20px rgba(0,0,0,0.06); }
    .error { color: #dc3545; text-align: center; margin-top: 10px; }
    .empty-msg { color: #888; text-align: center; padding: 30px; }

    .qa-input-area { margin-bottom: 20px; display: flex; flex-direction: column; gap: 10px; }
    .qa-input-area textarea { padding: 10px; border: 2px solid #eee; border-radius: 8px; resize: vertical; margin-bottom: 5px; font-size: 1em; font-family: inherit; }
    .submit-question-btn { background: #17a2b8; width: 100%; border-radius: 8px; padding: 12px; color: white; border: none; font-size: 1.1em; cursor: pointer; }
    .submit-question-btn:disabled { background: #ccc; cursor: not-allowed; }
    .qa-list { display: flex; flex-direction: column; gap: 15px; text-align: left; }
    .qa-item { padding: 15px; border: 1px solid #eee; border-radius: 8px; background: #fafafa; display: flex; flex-direction: column; position: relative; }
    .qa-content { font-size: 1.05em; margin-bottom: 10px; white-space: pre-wrap; padding-right: 55px; word-break: break-word; }
    .qa-meta { font-size: 0.8em; color: #888; display: flex; gap: 10px; align-items: center; }
    .qa-status.answered { color: #28a745; font-weight: bold; }
    .upvote-btn { position: absolute; right: 15px; top: 15px; padding: 5px 12px; background: #fff; border: 1px solid #ccc; border-radius: 20px; color: #555; font-size: 0.9em; display: flex; align-items: center; gap: 5px; cursor: pointer; transition: all 0.2s; }
    .upvote-btn.upvoted { background: #2563eb; color: white; border-color: #2563eb; }
</style>
