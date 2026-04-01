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
    import { goto } from '$app/navigation';
    import { questions } from '$lib/store';
    import { connect, disconnect } from '$lib/ws';
    import { getCsrfToken, ensureCsrfToken } from '$lib/api';

    import Toast from '$lib/components/Toast.svelte';
    import ConfirmModal from '$lib/components/ConfirmModal.svelte';
    import PollForm from '$lib/components/PollForm.svelte';
    import PollList from '$lib/components/PollList.svelte';
    import RankingCard from '$lib/components/RankingCard.svelte';
    import CsvImportCard from '$lib/components/CsvImportCard.svelte';
    import EmbedCard from '$lib/components/EmbedCard.svelte';
    import ModeratorCard from '$lib/components/ModeratorCard.svelte';
    import QaManagement from '$lib/components/QaManagement.svelte';

    /** @type {string} */
    let eventId = $page.params.id || '';

    /** @type {any} */
    let createdEvent = null;
    let hostTab = 'polls'; // 'polls', 'qa', 'settings'
    /** @type {any[]} */
    let polls = [];

    /** @type {any} */
    let activePollId = null;

    // Edit Event
    let editingTitle = false;
    let editTitleInput = '';

    // Ranking
    /** @type {any[]} */
    let rankingData = [];
    let showRanking = false;
    let updatingQaVisibility = false;

    // Moderators
    /** @type {any[]} */
    let moderators = [];

    // Toast
    let toastMessage = '';
    /** @type {'success' | 'error'} */
    let toastType = 'success';
    /** @type {ReturnType<typeof setTimeout> | undefined} */
    let toastTimeout;

    function showToast(/** @type {string} */ msg, /** @type {'success'|'error'} */ type = 'success') {
        toastMessage = msg;
        toastType = type;
        if (toastTimeout) {
            clearTimeout(toastTimeout);
        }
        toastTimeout = setTimeout(() => {
            toastMessage = '';
        }, 5000);
    }

    /** @type {{ message: string, onConfirm: function } | null} */
    let confirmModal = null;

    function requestConfirm(/** @type {string} */ message, /** @type {function} */ onConfirm) {
        confirmModal = { message, onConfirm };
    }

    onMount(async () => {
        await ensureCsrfToken();

        try {
            const eventRes = await fetch(`/api/events/${eventId}`);
            if (!eventRes.ok) {
                showToast('イベントが見つかりません。URLが間違っている可能性があります。', 'error');
                goto('/host');
                return;
            }
            createdEvent = await eventRes.json();
            console.log("DEBUG: Fetched Event", createdEvent);
            activePollId = createdEvent.current_poll_id || null;
            editTitleInput = createdEvent.title;

            const pollsRes = await fetch(`/api/events/${eventId}/polls`);
            if (!pollsRes.ok) {
                showToast('管理権限がありません（このブラウザで作成されていないイベントか、セッションが切れています）。', 'error');
                goto('/host');
                return;
            }
            polls = await pollsRes.json() || [];

            if (createdEvent.role === 'host') {
                const modsRes = await fetch(`/api/events/${eventId}/moderators`, { headers: { 'X-CSRF-Token': getCsrfToken() }});
                if (modsRes.ok) moderators = await modsRes.json() || [];
            }

            connect(eventId);
            fetch(`/api/events/${eventId}/questions`)
                .then(res => res.json())
                .then(qs => questions.set(qs || []));

        } catch (err) {
            console.error('Failed to load event data', err);
            showToast('データの読み込みに失敗しました。', 'error');
            goto('/host');
        }
    });

    onDestroy(() => {
        disconnect();
    });

    // ---- Event Edit / Delete ----

    async function saveTitle() {
        if (!editTitleInput.trim()) return;
        const res = await fetch(`/api/events/${eventId}`, {
            method: 'PUT',
            body: JSON.stringify({ title: editTitleInput.trim() }),
            headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': getCsrfToken() }
        });
        if (res.ok) {
            createdEvent = { ...createdEvent, title: editTitleInput.trim() };
            editingTitle = false;
        } else {
            showToast('タイトルの更新に失敗しました。', 'error');
        }
    }

    function deleteEvent() {
        requestConfirm(`「${createdEvent.title}」を削除しますか？この操作は元に戻せません。`, async () => {
            const res = await fetch(`/api/events/${eventId}`, {
                method: 'DELETE',
                headers: { 'X-CSRF-Token': getCsrfToken() }
            });
            if (res.ok) {
                goto('/host');
            } else {
                showToast('削除に失敗しました。', 'error');
            }
        });
    }

    // ---- Ranking ----

    async function fetchRanking() {
        const res = await fetch(`/api/events/${eventId}/ranking`);
        if (res.ok) {
            rankingData = await res.json();
            showRanking = true;
        } else {
            showToast('ランキングの取得に失敗しました。', 'error');
        }
    }

    async function toggleScreenQaVisibility() {
        if (!createdEvent || createdEvent.role !== 'host') return;

        const nextValue = !createdEvent.show_qa_on_screen;
        updatingQaVisibility = true;
        try {
            const res = await fetch(`/api/events/${eventId}`, {
                method: 'PUT',
                body: JSON.stringify({ show_qa_on_screen: nextValue }),
                headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': getCsrfToken() }
            });
            if (!res.ok) {
                showToast('投影画面のQ&A表示設定を更新できませんでした。', 'error');
                return;
            }
            createdEvent = { ...createdEvent, show_qa_on_screen: nextValue };
            showToast(nextValue ? '投影画面のQ&A表示を有効にしました。' : '投影画面のQ&A表示を無効にしました。');
        } catch (err) {
            console.error('toggleScreenQaVisibility failed', err);
            showToast('投影画面のQ&A表示設定を更新できませんでした。', 'error');
        } finally {
            updatingQaVisibility = false;
        }
    }

    function downloadTemplate() {
        window.open('/api/polls/template.csv', '_blank');
    }

    function exportCSV() {
        window.open(`/api/events/${eventId}/polls/export`, '_blank');
    }
</script>

<div class="max-w-4xl mx-auto p-6 font-sans text-gray-800">
    <div class="flex items-center justify-between mb-8">
        <h1 class="text-3xl font-extrabold text-gray-900 flex items-center gap-3">
            SocketJoin 管理画面
            {#if createdEvent && createdEvent.role === 'moderator'}
                <span class="inline-flex items-center px-3 py-1 rounded-full text-sm font-bold bg-gray-600 text-white shadow-sm align-middle mt-1 tracking-wider uppercase">
                    モデレーター権限
                </span>
            {/if}
        </h1>
        <div class="flex gap-2">
            <button on:click={fetchRanking} class="px-3 py-2 bg-yellow-100 text-yellow-800 font-medium hover:bg-yellow-200 rounded-lg transition text-sm">ランキング</button>
            <a href="/host" class="px-4 py-2 bg-gray-100 text-gray-700 font-medium hover:bg-gray-200 rounded-lg transition text-sm">Topに戻る</a>
        </div>
    </div>

    {#if createdEvent}
    <div class="space-y-6">

        <RankingCard {rankingData} show={showRanking} onClose={() => showRanking = false} />

        <!-- Tab Navigation -->
        <div class="flex gap-2 border-b border-gray-200 mb-6 w-full overflow-x-auto">
            <button on:click={() => hostTab = 'polls'} class="px-6 py-3 font-medium text-sm focus:outline-none transition border-b-2 {hostTab === 'polls' ? 'border-blue-600 text-blue-600' : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300'}">投票 (Polls)</button>
            <button on:click={() => hostTab = 'qa'} class="px-6 py-3 font-medium text-sm focus:outline-none transition border-b-2 {hostTab === 'qa' ? 'border-blue-600 text-blue-600' : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300'}">Q&A</button>
            <button on:click={() => hostTab = 'settings'} class="px-6 py-3 font-medium text-sm focus:outline-none transition border-b-2 {hostTab === 'settings' ? 'border-blue-600 text-blue-600' : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300'}">設定 (Settings)</button>
        </div>

        {#if hostTab === 'settings'}
        <!-- Event Info Card -->
        <div class="bg-white p-6 rounded-xl shadow-sm border border-gray-200">
            {#if createdEvent.role === 'host'}
                {#if editingTitle}
                    <div class="flex gap-2 mb-4">
                        <input type="text" bind:value={editTitleInput} class="flex-1 p-2 border rounded-lg" />
                        <button on:click={saveTitle} class="px-4 py-2 bg-blue-600 text-white rounded-lg text-sm">保存</button>
                        <button on:click={() => { editingTitle = false; editTitleInput = createdEvent.title; }} class="px-4 py-2 bg-gray-100 text-gray-700 rounded-lg text-sm">キャンセル</button>
                    </div>
                {:else}
                    <div class="flex items-center gap-3 mb-4">
                        <h2 class="text-2xl font-bold text-gray-900">{createdEvent.title}</h2>
                        <button on:click={() => { editingTitle = true; editTitleInput = createdEvent.title; }} class="px-3 py-1 text-xs bg-gray-100 text-gray-600 hover:bg-gray-200 rounded-lg">編集</button>
                        <button on:click={deleteEvent} class="px-3 py-1 text-xs bg-red-100 text-red-600 hover:bg-red-200 rounded-lg">削除</button>
                    </div>
                {/if}
            {:else}
                <div class="flex items-center gap-3 mb-4">
                    <h2 class="text-2xl font-bold text-gray-900">{createdEvent.title} <span class="text-sm font-normal text-gray-500 bg-gray-100 px-2 py-0.5 rounded-full ml-2">モデレーター</span></h2>
                </div>
            {/if}
            <div class="space-y-2 text-sm text-gray-600">
                <p class="flex items-center">
                    <span class="w-24 font-semibold text-gray-500">参加URL:</span>
                    <a href={`/join/${createdEvent.id}`} target="_blank" class="text-blue-600 hover:underline break-all">{typeof window !== 'undefined' ? window.location.origin : ''}/join/{createdEvent.id}</a>
                </p>
                <p class="flex items-center">
                    <span class="w-24 font-semibold text-gray-500">投影URL:</span>
                    <a href={`/screen/${createdEvent.id}`} target="_blank" class="text-blue-600 hover:underline break-all">{typeof window !== 'undefined' ? window.location.origin : ''}/screen/{createdEvent.id}</a>
                </p>
            </div>
            {#if createdEvent.role === 'host'}
                <div class="mt-4 rounded-xl border border-gray-200 bg-gray-50 p-4">
                    <div class="flex items-start justify-between gap-4">
                        <div>
                            <h3 class="text-sm font-semibold text-gray-900">投影URLのQ&A表示</h3>
                            <p class="mt-1 text-xs text-gray-500">質問が存在していても、投影画面のQ&Aパネルを表示しないように切り替えられます。</p>
                        </div>
                        <button
                            type="button"
                            class={`min-w-[92px] px-3 py-2 rounded-lg text-sm font-bold transition ${createdEvent.show_qa_on_screen ? 'bg-blue-600 text-white hover:bg-blue-700' : 'bg-gray-200 text-gray-700 hover:bg-gray-300'}`}
                            on:click={toggleScreenQaVisibility}
                            disabled={updatingQaVisibility}
                        >
                            {#if updatingQaVisibility}
                                更新中...
                            {:else if createdEvent.show_qa_on_screen}
                                ON
                            {:else}
                                OFF
                            {/if}
                        </button>
                    </div>
                </div>
                <div class="mt-4 flex gap-2">
                    <button on:click={downloadTemplate} class="px-3 py-1.5 text-xs bg-green-50 text-green-700 hover:bg-green-100 rounded-lg border border-green-200">CSVテンプレート</button>
                    <button on:click={exportCSV} class="px-3 py-1.5 text-xs bg-green-50 text-green-700 hover:bg-green-100 rounded-lg border border-green-200">結果CSVエクスポート</button>
                </div>
            {/if}
        </div>

        {#if createdEvent.role === 'host'}
            <ModeratorCard
                {eventId}
                csrfToken={getCsrfToken()}
                bind:moderators
                {showToast}
                {requestConfirm}
            />
        {/if}
        {/if}

        {#if hostTab === 'polls'}
        <div class="grid grid-cols-1 lg:grid-cols-3 gap-6">
            <!-- Left Column: Main Poll Controls -->
            <div class="lg:col-span-2 space-y-6">
                {#if createdEvent.role === 'host'}
                    <PollForm
                        {eventId}
                        csrfToken={getCsrfToken()}
                        {showToast}
                        onPollCreated={(poll) => { polls = [...polls, poll]; }}
                    />
                {/if}

                <PollList
                    bind:polls
                    {activePollId}
                    role={createdEvent.role}
                    csrfToken={getCsrfToken()}
                    {eventId}
                    {showToast}
                    {requestConfirm}
                    onFetchRanking={fetchRanking}
                    onActivePollChange={(id) => { activePollId = id; }}
                />
            </div>

            <!-- Right Column: Sidebar -->
            <div class="space-y-6">
                {#if createdEvent.role === 'host'}
                    <CsvImportCard
                        {eventId}
                        csrfToken={getCsrfToken()}
                        {showToast}
                        onPollsReloaded={(p) => { polls = p; }}
                    />
                    <EmbedCard
                        {eventId}
                        csrfToken={getCsrfToken()}
                        {showToast}
                    />
                {/if}
            </div>
        </div>
        {/if}

        {#if hostTab === 'qa'}
            <QaManagement
                {eventId}
                csrfToken={getCsrfToken()}
                {showToast}
            />
        {/if}

    </div>
    {/if}
</div>

<Toast message={toastMessage} type={toastType} onClose={() => toastMessage = ''} />
<ConfirmModal modal={confirmModal} onCancel={() => confirmModal = null} />
