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
    import { get } from 'svelte/store';
    import { fetchEventHistory, submitPollVote, getCsrfToken, ensureCsrfToken } from '$lib/api';
    import { votedPollsStore, markPollAsVoted, saveNickname, getSavedNickname, hasVotedForPoll } from '$lib/storage';

    import BottomNav from '$lib/components/BottomNav.svelte';
    import QaTab from '$lib/components/QaTab.svelte';

    export let data;

    let inputNickname = '';
    let hasJoined = false;
    /** @type {string[]} */
    let selectedOptionIds = [];
    let errorMessage = '';

    let currentTab = 'poll'; // 'poll', 'history', 'qa'
    /** @type {string | null} */
    let activePollIdForRedirect = null;
    /** @type {any[]} */
    let voteHistory = [];

    $: maxSel = $poll?.max_selections ?? 1;
    $: isMulti = maxSel > 1;
    $: hasVoted = $poll ? hasVotedForPoll($poll.id, $votedPollsStore) : false;

    onMount(async () => {
        if (data.error || !data.event) return;
        await ensureCsrfToken();
        event.set(data.event);
        connect(data.event.id);

        const policy = data.event.nickname_policy || 'optional';

        if (policy === 'hidden') {
            hasJoined = true;
            nickname.set('Anonymous');
            loadHistory();
        } else {
            const cachedName = getSavedNickname();
            if (cachedName) {
                inputNickname = cachedName;
                nickname.set(cachedName);
                hasJoined = true;
                loadHistory();
            }
        }

        if (data.event.current_poll_id) {
            fetch(`/api/poll/${data.event.current_poll_id}`)
                .then(res => res.json())
                .then(p => {
                    poll.set(p);
                });
        }

        // Fetch existing questions
        fetch(`/api/events/${data.event.id}/questions`)
            .then(res => res.json())
            .then(qs => questions.set(qs || []));
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
             // Handle poll resets without ID changing
             selectedOptionIds = [];
        }
    }

    function joinEvent() {
        if (!inputNickname.trim() && data.event.nickname_policy === 'required') {
            alert('ニックネームを入力してください。');
            return;
        }
        const nameToUse = inputNickname.trim() || 'Anonymous';
        saveNickname(nameToUse);
        nickname.set(nameToUse);
        hasJoined = true;
        loadHistory();
    }

    function skipNickname() {
        saveNickname('Anonymous');
        nickname.set('Anonymous');
        hasJoined = true;
        loadHistory();
    }

    async function loadHistory() {
        voteHistory = await fetchEventHistory(data.event.id);
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
    function handleTabChange(tab) {
        currentTab = tab;
        if (tab === 'history') loadHistory();
    }
</script>

<div class="container">
    {#if data.error || !data.event}
        <div class="card"><p class="error">イベントが見つかりません。</p></div>
    {:else if !hasJoined}
        <!-- Nickname Entry -->
        <div class="card">
            <h1>{data.event.title} に参加</h1>
            <p>ニックネームを入力してください。</p>
            <input type="text" bind:value={inputNickname} placeholder="ニックネーム" />
            <div class="flex-actions">
                {#if data.event.nickname_policy === 'optional'}
                    <button class="secondary-btn" on:click={skipNickname}>スキップ (匿名)</button>
                {/if}
                <button on:click={joinEvent}>参加する</button>
            </div>
            {#if data.event.nickname_policy === 'required'}
                <p class="required-note">* このイベントはニックネームの入力が必須です。</p>
            {/if}
        </div>
    {:else}
        <!-- Event Lobby / Poll View -->
        <div class="header">
            <span>@{ $nickname }</span>
            <span class="status {$connectionStatus}">
                {$connectionStatus === 'connected' ? '接続中' : '接続待機...'}
            </span>
        </div>

        {#if currentTab === 'poll'}
            {#if !$poll}
                <div class="waiting">
                    <h2>待機中...</h2>
                    <p>次の質問を待っています。</p>
                    <div class="loader"></div>
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
                <h2>過去の回答履歴</h2>
                {#if voteHistory.length === 0}
                    <p class="empty-msg">まだ回答履歴がありません。</p>
                {:else}
                    <ul class="history-list">
                        {#each voteHistory as item}
                            <li class="history-item">
                                <div class="h-title">{item.poll_title}</div>
                                <div class="h-answer">
                                    あなたの回答: <strong>{item.option_label}</strong>
                                    {#if item.poll_status === 'closed' && item.is_quiz}
                                        {#if item.option_is_correct}
                                            <span class="correct-badge">正解</span>
                                        {:else}
                                            <span class="incorrect-badge">不正解</span>
                                        {/if}
                                    {/if}
                                </div>
                                <div class="h-date">{new Date(item.created_at).toLocaleString()}</div>
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
    .container { max-width: 480px; margin: 0 auto; padding: 20px; font-family: sans-serif; padding-bottom: 90px; }
    .card { background: #fff; padding: 30px 20px; text-align: center; border-radius: 12px; box-shadow: 0 4px 20px rgba(0,0,0,0.06); }
    input[type=text] { padding: 10px; font-size: 1.2em; width: 80%; margin-bottom: 20px; border: 2px solid #ddd; border-radius: 4px; }
    button { padding: 12px 30px; font-size: 1.1em; background: #007bff; color: white; border: none; border-radius: 30px; cursor: pointer; transition: background 0.2s; }
    button:disabled { background: #ccc; cursor: not-allowed; }

    .header { display: flex; justify-content: space-between; margin-bottom: 20px; color: #666; font-size: 0.9em; }
    .status.connected { color: #28a745; }

    .waiting { text-align: center; padding: 50px 0; color: #888; }
    .loader { display: inline-block; width: 40px; height: 40px; border: 4px solid #f3f3f3; border-top: 4px solid #007bff; border-radius: 50%; animation: spin 1s linear infinite; margin-top: 20px; }
    @keyframes spin { 0% { transform: rotate(0deg); } 100% { transform: rotate(360deg); } }

    .poll-view h2 { margin-bottom: 10px; }
    .multi-hint { font-size: 0.85em; color: #666; margin-bottom: 15px; }

    .options { display: flex; flex-direction: column; gap: 10px; }
    .option-btn {
        display: flex; align-items: center; gap: 10px;
        padding: 15px; border: 2px solid #eee; border-radius: 8px;
        cursor: pointer; transition: all 0.2s; background: #fff;
        font-size: 1em; color: #333; text-align: left; width: 100%;
    }
    .option-btn.selected { border-color: #007bff; background: #f0f7ff; }
    .option-btn.disabled { opacity: 0.7; cursor: default; }
    .checkbox, .radio { font-size: 1.2em; color: #007bff; min-width: 1.2em; }

    .vote-btn { width: 100%; margin-top: 20px; background: #007bff; color: white; border: none; padding: 15px; border-radius: 8px; font-size: 1.2em; cursor: pointer; }
    .voted-msg { background: #d4edda; color: #155724; padding: 15px; text-align: center; border-radius: 8px; margin-top: 20px; }
    .voted-msg.closed { background: #e2e3e5; color: #383d41; }
    .error { color: #dc3545; text-align: center; margin-top: 10px; }

    .flex-actions { display: flex; gap: 10px; justify-content: center; }
    .secondary-btn { background: #f0f0f0; color: #333; }
    .secondary-btn:hover { background: #e0e0e0; }
    .required-note { font-size: 0.85em; color: #dc3545; margin-top: 15px; }

    .history-view h2 { margin-bottom: 20px; border-bottom: 2px solid #eee; padding-bottom: 10px;}
    .empty-msg { color: #888; text-align: center; padding: 30px; }
    .history-list { list-style: none; padding: 0; margin: 0; text-align: left; }
    .history-item { padding: 15px 0; border-bottom: 1px solid #eee; }
    .history-item:last-child { border-bottom: none; }
    .h-title { font-weight: bold; font-size: 1.1em; margin-bottom: 5px; color: #333; }
    .h-answer { font-size: 0.95em; color: #555; margin-bottom: 5px; }
    .h-date { font-size: 0.8em; color: #aaa; }
    .correct-badge { display: inline-block; background: #28a745; color: white; padding: 2px 6px; border-radius: 4px; font-size: 0.8em; margin-left: 8px; }
    .incorrect-badge { display: inline-block; background: #dc3545; color: white; padding: 2px 6px; border-radius: 4px; font-size: 0.8em; margin-left: 8px; }
</style>
