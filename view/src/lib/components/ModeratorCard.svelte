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
    /** @type {any[]} */
    export let moderators = [];
    /** @type {(msg: string, type?: 'success'|'error') => void} */
    export let showToast;
    /** @type {(message: string, onConfirm: function) => void} */
    export let requestConfirm;

    let newModName = '';

    async function createModerator() {
        if (!newModName.trim()) return;
        const res = await fetch(`/api/events/${eventId}/moderators`, {
            method: 'POST',
            body: JSON.stringify({ name: newModName.trim() }),
            headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': csrfToken }
        });
        if (res.ok) {
            const mod = await res.json();
            moderators = [...moderators, mod];
            newModName = '';
            copyModUrl(mod);
            showToast(`「${mod.name}」を追加しました。\nクリップボードにログインURLをコピーしました。\n※ URLは作成時にしか表示・コピーできません。`);
        } else {
            const msg = await res.text();
            showToast('モデレーターの追加に失敗しました: ' + msg, 'error');
        }
    }

    /** @param {string} modId */
    function deleteModerator(modId) {
        requestConfirm('このモデレーターを削除しますか？', async () => {
            const res = await fetch(`/api/events/${eventId}/moderators/${modId}`, {
                method: 'DELETE',
                headers: { 'X-CSRF-Token': csrfToken }
            });
            if (res.ok) {
                moderators = moderators.filter(m => m.id !== modId);
                showToast('削除しました。');
            } else {
                showToast('削除に失敗しました。', 'error');
            }
        });
    }

    /** @param {any} mod */
    function copyModUrl(mod) {
        if (mod.token) {
            const url = `${window.location.origin}/moderator?token=${mod.token}`;
            navigator.clipboard.writeText(url).catch(err => {
                console.error('Copy failed', err);
                showToast('URLのコピーに失敗しました。');
            });
        } else {
            showToast('ログインURLは作成時にしか表示・コピーできません。再発行する場合は一度削除して作成し直してください。', 'error');
        }
    }
</script>

<div class="bg-white p-6 rounded-xl shadow-sm border border-gray-200">
    <h3 class="text-lg font-bold mb-4 text-gray-900 border-b pb-2">モデレーター管理</h3>

    <div class="mb-6 space-y-3">
        <div class="bg-yellow-50 p-3 rounded-lg border border-yellow-200 text-xs text-gray-700 leading-relaxed">
            <span class="font-bold text-yellow-800 block mb-1">■ ログインURLの仕様（セキュリティ仕様）</span>
            モデレーターのログイン用URLは、セキュリティ上の理由から<strong class="text-red-600">追加した瞬間に一度だけ自動でクリップボードへコピー</strong>されます。<br>
            以降、システム上でURLを再確認・再生成することはできません。紛失した場合は一度削除して再度追加してください。
        </div>

        <div class="bg-blue-50 p-3 rounded-lg border border-blue-200 text-xs text-gray-700 leading-relaxed">
            <span class="font-bold text-blue-800 block mb-1">■ モデレーターの権限仕様</span>
            モデレーターはイベント進行補助として、以下の機能<span class="font-bold">のみ操作可能</span>です。
            <ul class="list-disc list-inside mt-1 ml-1 text-gray-600">
                <li>作成済みの投票の「開始」および「締め切り」</li>
                <li>参加者からのQ&amp;Aへのステータス変更・アーカイブ対応</li>
            </ul>
            ※イベント設定の変更、新たな投票・クイズの作成、リセット操作、参加者のBAN操作などは一切行えません。
        </div>

        <div class="bg-gray-50 p-3 rounded-lg border border-gray-200 text-xs text-gray-700 leading-relaxed">
            <span class="font-bold text-gray-800 block mb-1">■ 動作確認時のご注意（Cookieについて）</span>
            現在オーナー権限でログインしているブラウザでモデレーターURLを開くと、最も強い権限である「オーナー」として処理され、モデレーター専用の画面になりません。<br>
            権限や画面の見え方を確認する場合は、必ず<span class="font-bold text-red-600">シークレットウィンドウ（プライベートブラウズ）など別のブラウザ環境</span>でURLを開いてください。
        </div>
    </div>

    <div class="flex gap-2 mb-4">
        <input type="text" bind:value={newModName} placeholder="モデレーター名" class="flex-1 p-2 border rounded-lg" />
        <button on:click={createModerator} disabled={moderators.length >= 3} class="px-4 py-2 bg-blue-600 text-white font-medium rounded-lg hover:bg-blue-700 disabled:bg-gray-400 transition">追加</button>
    </div>
    {#if moderators.length === 0}
        <p class="text-gray-500 text-sm">モデレーターは登録されていません。</p>
    {:else}
        <ul class="space-y-2 text-sm">
            {#each moderators as mod}
                <li class="flex items-center justify-between p-3 bg-gray-50 border rounded-lg">
                    <span class="font-medium text-gray-900">{mod.name}</span>
                    <div class="flex items-center gap-2">
                        <span class="text-xs text-gray-400 mr-2">URL表示不可</span>
                        <button class="px-3 py-1 text-xs font-bold bg-red-100 text-red-600 hover:bg-red-200 rounded" on:click={() => deleteModerator(mod.id)}>削除</button>
                    </div>
                </li>
            {/each}
        </ul>
    {/if}
</div>
