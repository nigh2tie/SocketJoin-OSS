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
    export let eventId;
    /** @type {string} */
    export let csrfToken;
    /** @type {(msg: string, type?: 'success'|'error') => void} */
    export let showToast;

    let embedToken = '';
    let embedAllowedOrigins = '';

    async function generateEmbed() {
        const origin = embedAllowedOrigins.trim() || '*';
        const res = await fetch(`/api/events/${eventId}/embed_token`, {
            method: 'POST',
            body: JSON.stringify({ allowed_origins: [origin] }),
            headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': csrfToken }
        });
        if (res.ok) {
            const data = await res.json();
            embedToken = data.token;
            showToast('埋め込みコードを発行しました。');
        } else {
            showToast('トークンの発行に失敗しました', 'error');
        }
    }
</script>

<div class="bg-white p-6 rounded-xl shadow-sm border border-gray-200">
    <h3 class="text-lg font-bold mb-4 text-gray-900 border-b pb-2">イベント埋め込み (Live iframe)</h3>
    <p class="text-sm text-gray-600 mb-4">外部サイトにこのイベントの進行状況（現在アクティブな投票やQ&A）をリアルタイムで埋め込むことができます。</p>
    <div class="flex flex-col gap-3 mb-4">
        <input type="text" bind:value={embedAllowedOrigins} placeholder="許可ドメイン (例: https://example.com)" class="w-full p-2 text-sm border rounded-lg" />
        <button on:click={generateEmbed} class="w-full py-2 bg-gray-600 text-white font-medium rounded-lg hover:bg-gray-700 transition">コード発行</button>
    </div>
    {#if embedToken}
        <div class="p-3 bg-gray-800 text-gray-300 rounded-lg mt-2">
            <p class="text-xs font-semibold mb-2 text-gray-400 uppercase tracking-wide">埋め込みコード:</p>
            <textarea class="w-full h-24 p-2 bg-gray-900 text-green-400 font-mono text-xs border border-gray-700 rounded outline-none resize-none" readonly>&lt;iframe src="{typeof window !== 'undefined' ? window.location.origin : ''}/embed/join/{eventId}?token={embedToken}" width="100%" height="600px" frameborder="0"&gt;&lt;/iframe&gt;</textarea>
        </div>
    {/if}
</div>
