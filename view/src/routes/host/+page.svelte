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
    import { onMount } from 'svelte';
    import { goto } from '$app/navigation';
    
    let title = '';
    let nicknamePolicy = 'optional'; // hidden, optional, required
    
    onMount(async () => {
        // Init CSRF cookie from the backend API
        await fetch('/api/csrf', { method: 'GET' });
    });

    function getCsrfToken() {
        if (typeof document !== 'undefined') {
            const match = document.cookie.match(new RegExp('(^| )csrf_token=([^;]+)'));
            if (match) return match[2];
        }
        return '';
    }

    let toastMessage = '';
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

    // Event Creation
    async function createEvent() {
        if (!title.trim()) {
            showToast('イベント名を入力してください。', 'error');
            return;
        }

        try {
            const res = await fetch('/api/events', {
                method: 'POST',
                body: JSON.stringify({ title: title.trim(), nickname_policy: nicknamePolicy }),
                headers: {
                    'Content-Type': 'application/json',
                    'X-CSRF-Token': getCsrfToken()
                }
            });

            if (!res.ok) {
                const message = await res.text();
                showToast(`イベント作成に失敗しました (${res.status}): ${message || 'unknown error'}`, 'error');
                return;
            }

            const createdEvent = await res.json();

            // Navigate to the event management page (token stored in HttpOnly cookie)
            goto(`/host/${createdEvent.id}`);
        } catch (err) {
            console.error('createEvent failed', err);
            showToast('イベント作成に失敗しました。APIサーバーへの接続を確認してください。', 'error');
        }
    }
</script>

<div class="max-w-4xl mx-auto p-6 font-sans text-gray-800">
    <h1 class="text-3xl font-extrabold text-center mb-8 text-gray-900">SocketJoin ダッシュボード</h1>

    <!-- Create New Event -->
    <div class="bg-white p-8 rounded-xl shadow-sm border border-gray-200 max-w-xl mx-auto mb-10">
        <h2 class="text-2xl font-bold mb-6 text-gray-900 border-b pb-2">新しくイベントを作成</h2>
        
        <div class="mb-5">
            <div class="block text-sm font-semibold text-gray-700 mb-2">イベント名</div>
            <input type="text" bind:value={title} placeholder="例：社内勉強会、ハッカソン審査" class="w-full p-3 bg-gray-50 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 focus:outline-none transition" />
        </div>

        <div class="mb-8">
            <div class="block text-sm font-semibold text-gray-700 mb-2">参加者のニックネーム入力</div>
            <div class="flex flex-col sm:flex-row gap-4">
                <label class="flex items-center cursor-pointer p-3 border rounded-lg hover:bg-gray-50 flex-1 transition" class:border-blue-500={nicknamePolicy === 'hidden'} class:bg-blue-50={nicknamePolicy === 'hidden'}>
                    <input type="radio" value="hidden" bind:group={nicknamePolicy} class="accent-blue-600 w-4 h-4 mr-3" />
                    <div>
                        <div class="font-medium text-gray-900">不要（完全匿名）</div>
                        <div class="text-xs text-gray-500 mt-1">誰でも気軽に参加</div>
                    </div>
                </label>
                <label class="flex items-center cursor-pointer p-3 border rounded-lg hover:bg-gray-50 flex-1 transition" class:border-blue-500={nicknamePolicy === 'optional'} class:bg-blue-50={nicknamePolicy === 'optional'}>
                    <input type="radio" value="optional" bind:group={nicknamePolicy} class="accent-blue-600 w-4 h-4 mr-3" />
                    <div>
                        <div class="font-medium text-gray-900">任意入力</div>
                        <div class="text-xs text-gray-500 mt-1">MVPデフォルト</div>
                    </div>
                </label>
                <label class="flex items-center cursor-pointer p-3 border rounded-lg hover:bg-gray-50 flex-1 transition" class:border-blue-500={nicknamePolicy === 'required'} class:bg-blue-50={nicknamePolicy === 'required'}>
                    <input type="radio" value="required" bind:group={nicknamePolicy} class="accent-blue-600 w-4 h-4 mr-3" />
                    <div>
                        <div class="font-medium text-gray-900">必須入力</div>
                        <div class="text-xs text-gray-500 mt-1">ランキング等に最適</div>
                    </div>
                </label>
            </div>
        </div>

        <button on:click={createEvent} class="w-full py-3.5 bg-blue-600 text-white font-bold text-lg rounded-lg shadow-sm hover:bg-blue-700 hover:shadow transition cursor-pointer">イベントを作成する</button>
    </div>
</div>

{#if toastMessage}
<div class="fixed top-6 right-6 z-[5000] animate-fade-in-up">
    <div class={`px-4 py-3 rounded-xl shadow-xl border flex items-center gap-3 min-w-[300px] ${toastType === 'error' ? 'bg-red-50 border-red-200 text-red-800' : 'bg-green-50 border-green-200 text-green-800'}`}>
        <div class={`w-2 h-2 rounded-full ${toastType === 'error' ? 'bg-red-500' : 'bg-green-500'}`}></div>
        <p class="font-bold text-sm whitespace-pre-wrap">{toastMessage}</p>
        <button on:click={() => toastMessage = ''} class="ml-auto text-gray-400 hover:text-gray-600 font-bold p-1">✕</button>
    </div>
</div>
{/if}
