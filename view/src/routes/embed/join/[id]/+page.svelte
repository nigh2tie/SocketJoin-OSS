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
    import { onMount, onDestroy } from 'svelte';
    import { page } from '$app/stores';
    import { poll, connectionStatus, event, nickname, questions } from '$lib/store';
    import { connect, disconnect } from '$lib/ws';
    import { fetchEventHistory, submitPollVote, ensureCsrfToken } from '$lib/api';
    import { votedPollsStore, markPollAsVoted, hasVotedForPoll } from '$lib/storage';

    import BottomNav from '$lib/components/BottomNav.svelte';
    import QaTab from '$lib/components/QaTab.svelte';

    export let data;

    /** @type {string[]} */
    let selectedOptionIds = [];
    let errorMessage = '';

    let currentTab = 'poll';
    /** @type {string | null} */
    let activePollIdForRedirect = null;
    /** @type {any[]} */
    let voteHistory = [];

    let embedAccess = false;
    let checkingAccess = true;
    let accessError = '';

    $: maxSel = $poll?.max_selections ?? 1;
    $: isMulti = maxSel > 1;
    $: hasVoted = $poll ? hasVotedForPoll($poll.id, $votedPollsStore) : false;

    onMount(async () => {
        if (data.error || !data.event) {
            accessError = "イベントが見つかりません";
            checkingAccess = false;
            return;
        }
        await ensureCsrfToken();

        // Validate Embed Token first
        const token = $page.url.searchParams.get('token');
        if (!token) {
            accessError = "埋め込みトークンがありません";
            checkingAccess = false;
            return;
        }

        try {
            const res = await fetch(`/api/embed/verify/${data.event.id}?token=${token}`);
            if (res.ok) {
                embedAccess = true;
                event.set(data.event);
                connect(data.event.id);
                nickname.set('Anonymous Viewer');

                // Fetch existing questions
                fetch(`/api/events/${data.event.id}/questions`)
                    .then(res => res.json())
                    .then(qs => questions.set(qs || []));

                voteHistory = await fetchEventHistory(data.event.id);
            } else {
                accessError = "アクセスが拒否されました";
            }
        } catch (e) {
            accessError = "アクセス確認に失敗しました";
        }
        checkingAccess = false;
    });

    onDestroy(() => {
        disconnect();
    });

    // Reset selection and force tab switch when poll changes
    $: if ($poll) {
        if ($poll.id !== activePollIdForRedirect) {
            selectedOptionIds = [];

            // If the poll is freshly opened, force the user back to the poll tab
            if ($poll.status === 'open') {
                currentTab = 'poll';
            }
            activePollIdForRedirect = $poll.id;
        } else if ($poll.status === 'open' && hasVoted === false) {
            selectedOptionIds = [];
        }
    }

    /** @param {string} optId */
    function toggleOption(optId) {
        if (hasVoted || !$poll || $poll.status === 'closed') return;

        if (isMulti) {
            if (selectedOptionIds.includes(optId)) {
                selectedOptionIds = selectedOptionIds.filter(id => id !== optId);
            } else if (selectedOptionIds.length < maxSel) {
                selectedOptionIds = [...selectedOptionIds, optId];
            }
        } else {
            selectedOptionIds = [optId];
        }
    }

    async function submitVote() {
        if (selectedOptionIds.length === 0 || hasVoted || !$poll || $poll.status === 'closed') return;

        errorMessage = '';
        try {
            await submitPollVote($poll.id, selectedOptionIds, $nickname);
            markPollAsVoted($poll.id);
        } catch (err) {
            if (err instanceof Error) {
                if (err.message === 'accessed_denied') {
                    errorMessage = 'アクセスが制限されています。';
                } else if (err.message === 'already_voted') {
                    errorMessage = 'すでに投票済みです。';
                    markPollAsVoted($poll.id);
                } else {
                    errorMessage = err.message || 'エラーが発生しました。';
                }
            } else {
                errorMessage = 'エラーが発生しました。';
            }
        }
    }

    /** @param {string} tab */
    async function handleTabChange(tab) {
        currentTab = tab;
        if (tab === 'history') {
            voteHistory = await fetchEventHistory(data.event.id);
        }
    }
</script>

<div class="container">
    {#if checkingAccess}
        <div class="card"><p class="loading">アクセスを確認中...</p></div>
    {:else if !embedAccess}
        <div class="card"><p class="error">{accessError}</p></div>
    {:else}
        <div class="header">
            <span>イベント: {data.event.title}</span>
            <span class="status {$connectionStatus}">
                {$connectionStatus === 'connected' ? '接続中' : '接続待機...'}
            </span>
        </div>

        {#if currentTab === 'poll'}
            {#if !$poll}
                <div class="waiting card">
                    <div class="waiting-icon">
                        <svg xmlns="http://www.w3.org/2000/svg" class="h-16 w-16 text-blue-500 mx-auto animate-pulse" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
                        </svg>
                    </div>
                    <h2 class="text-xl font-bold text-gray-800 mt-4">現在、実施中の投票はありません</h2>
                    <p class="text-gray-500 mt-2">主催者が投票を開始すると、ここに表示されます。</p>
                    <div class="mt-8 flex flex-col gap-3">
                        <button class="px-6 py-2 bg-blue-100 text-blue-700 font-bold rounded-lg hover:bg-blue-200" on:click={() => handleTabChange('history')}>これまでの投票を見る</button>
                        <button class="px-6 py-2 bg-gray-100 text-gray-700 font-bold rounded-lg hover:bg-gray-200" on:click={() => currentTab = 'qa'}>質問を送る (Q&A)</button>
                    </div>
                </div>
            {:else}
                <div class="poll-view card">
                    <h2>{$poll.title}</h2>

                    {#if isMulti}
                        <p class="multi-hint">最大 {maxSel} 個まで選択できます（{selectedOptionIds.length}/{maxSel}）</p>
                    {/if}

                    <div class="options">
                        {#each $poll.options as opt}
                            {@const isSelected = selectedOptionIds.includes(opt.id)}
                            {@const isDisabledExtra = isMulti && !isSelected && selectedOptionIds.length >= maxSel}
                            <button
                                class="option-btn"
                                class:selected={isSelected}
                                class:disabled={hasVoted || $poll.status === 'closed' || isDisabledExtra}
                                on:click={() => toggleOption(opt.id)}
                                disabled={hasVoted || $poll.status === 'closed' || isDisabledExtra}
                                type="button"
                            >
                                {#if isMulti}
                                    <span class="checkbox">{isSelected ? '☑' : '☐'}</span>
                                {:else}
                                    <span class="radio">{isSelected ? '●' : '○'}</span>
                                {/if}
                                {opt.label}
                            </button>
                        {/each}
                    </div>

                    {#if errorMessage}
                        <p class="error">{errorMessage}</p>
                    {/if}

                    {#if $poll.status === 'closed'}
                        <div class="voted-msg closed">投票受付は終了しました。</div>
                    {:else if hasVoted}
                        <div class="voted-msg">投票ありがとうございました！</div>
                    {:else}
                        <button class="vote-btn" on:click={submitVote} disabled={selectedOptionIds.length === 0}>
                            投票する{#if isMulti && selectedOptionIds.length > 0}（{selectedOptionIds.length}件）{/if}
                        </button>
                    {/if}
                </div>
            {/if}
        {:else if currentTab === 'history'}
            <div class="history-view card">
                <h2 class="text-xl font-bold mb-4 text-gray-800 border-b pb-2">投票一覧 (履歴)</h2>
                {#if voteHistory.length === 0}
                    <p class="empty-msg text-gray-400 py-10">まだ回答済みの投票がありません。</p>
                {:else}
                    <ul class="space-y-4 text-left">
                        {#each voteHistory as item}
                            <li class="p-4 bg-gray-50 border border-gray-100 rounded-xl">
                                <div class="font-bold text-gray-800 mb-2">{item.poll_title}</div>
                                <div class="text-sm text-gray-600">
                                    あなたの回答: <span class="font-bold text-blue-600">{item.option_label}</span>
                                    {#if item.poll_status === 'closed' && item.is_quiz}
                                        {#if item.option_is_correct}
                                            <span class="ml-2 inline-block bg-green-100 text-green-700 px-2 py-0.5 rounded text-xs font-bold">正解</span>
                                        {:else}
                                            <span class="ml-2 inline-block bg-red-100 text-red-700 px-2 py-0.5 rounded text-xs font-bold">不正解</span>
                                        {/if}
                                    {/if}
                                </div>
                                <div class="text-[10px] text-gray-400 mt-1">{new Date(item.created_at).toLocaleString()}</div>
                            </li>
                        {/each}
                    </ul>
                {/if}
            </div>
        {:else if currentTab === 'qa'}
            <QaTab eventId={data.event.id} />
        {/if}

        <BottomNav {currentTab} onTabChange={handleTabChange} />
    {/if}
</div>

<style>
    :global(body) {
        background: #f8fafc;
        margin: 0;
        padding: 0;
        font-family: 'Inter', -apple-system, sans-serif;
    }
    .container {
        width: 100%;
        height: 100vh;
        overflow-y: auto;
        padding: 16px;
        padding-bottom: 100px;
        box-sizing: border-box;
    }
    .card {
        background: rgba(255, 255, 255, 0.9);
        backdrop-filter: blur(10px);
        padding: 24px 20px;
        text-align: center;
        border-radius: 20px;
        box-shadow: 0 10px 30px rgba(0,0,0,0.05);
        border: 1px solid rgba(255,255,255,0.7);
        margin-bottom: 20px;
    }

    h2 { font-size: 1.25rem; color: #1e293b; margin-top: 0; }

    .header {
        display: flex;
        justify-content: space-between;
        align-items: center;
        margin-bottom: 24px;
        padding: 0 8px;
    }
    .status {
        font-size: 0.75rem;
        padding: 4px 10px;
        border-radius: 20px;
        font-weight: 600;
    }
    .status.connected { background: #dcfce7; color: #166534; }
    .status.disconnected { background: #fee2e2; color: #991b1b; }

    .waiting {
        padding: 60px 20px;
        display: flex;
        flex-direction: column;
        align-items: center;
    }
    .waiting-icon { margin-bottom: 20px; opacity: 0.8; }

    .poll-view h2 { margin-bottom: 12px; }
    .multi-hint { font-size: 0.8rem; color: #64748b; margin-bottom: 20px; background: #f1f5f9; padding: 6px 12px; border-radius: 10px; display: inline-block; }

    .options { display: flex; flex-direction: column; gap: 12px; }
    .option-btn {
        display: flex; align-items: center; gap: 12px;
        padding: 16px; border: 2px solid #e2e8f0; border-radius: 14px;
        cursor: pointer; transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);
        background: #fff;
        font-size: 1rem; color: #334155; text-align: left; width: 100%;
    }
    .option-btn:hover:not(:disabled) { border-color: #3b82f6; background: #eff6ff; transform: translateY(-1px); }
    .option-btn.selected { border-color: #3b82f6; background: #eff6ff; box-shadow: 0 4px 12px rgba(59, 130, 246, 0.1); }
    .option-btn.disabled { opacity: 0.6; cursor: default; }
    .checkbox, .radio { font-size: 1.2em; color: #3b82f6; min-width: 1.2em; }

    .vote-btn {
        width: 100%; margin-top: 24px;
        background: linear-gradient(135deg, #3b82f6 0%, #2563eb 100%);
        color: white; border: none; padding: 16px; border-radius: 14px;
        font-size: 1.1rem; cursor: pointer; font-weight: 700;
        box-shadow: 0 4px 15px rgba(37, 99, 235, 0.3);
        transition: all 0.2s;
    }
    .vote-btn:hover:not(:disabled) { transform: translateY(-2px); box-shadow: 0 6px 20px rgba(37, 99, 235, 0.4); }
    .vote-btn:active:not(:disabled) { transform: translateY(0); }
    .vote-btn:disabled { background: #cbd5e1; box-shadow: none; cursor: not-allowed; }

    .voted-msg { background: #f0fdf4; color: #166534; padding: 16px; text-align: center; border-radius: 14px; margin-top: 24px; font-size: 0.95rem; font-weight: 600; border: 1px solid #bbfcce; }
    .voted-msg.closed { background: #f1f5f9; color: #475569; border-color: #e2e8f0; }

    .error { color: #ef4444; text-align: center; margin-top: 12px; font-size: 0.85rem; font-weight: 500; }
    .loading { color: #64748b; text-align: center; margin-top: 40px; font-weight: 500; }

    .empty-msg { color: #94a3b8; text-align: center; padding: 40px 20px; }
</style>
