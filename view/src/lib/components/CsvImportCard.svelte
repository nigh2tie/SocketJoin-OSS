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
    /** @type {(polls: any[]) => void} */
    export let onPollsReloaded;

    /** @type {File | null} */
    let importFile = null;
    let importDryRun = true;
    let importLoading = false;
    /** @type {any} */
    let importResult = null;

    /** @param {Event} e */
    function onFileChange(e) {
        const target = /** @type {HTMLInputElement} */ (e.target);
        importFile = target.files?.[0] || null;
    }

    async function importCSV() {
        if (!importFile) { showToast('ファイルを選択してください。', 'error'); return; }
        importLoading = true;
        importResult = null;

        const form = new FormData();
        form.append('file', importFile);
        form.append('dry_run', importDryRun ? 'true' : 'false');

        const res = await fetch(`/api/events/${eventId}/polls/import`, {
            method: 'POST',
            body: form,
            headers: { 'X-CSRF-Token': csrfToken }
        });

        importLoading = false;
        if (res.ok) {
            importResult = await res.json();
            if (!importDryRun && importResult.failed === 0 && importResult.success > 0) {
                const pollsRes = await fetch(`/api/events/${eventId}/polls`);
                if (pollsRes.ok) onPollsReloaded(await pollsRes.json() || []);
            }
            showToast('インポート処理が完了しました。');
        } else {
            const msg = await res.text();
            showToast('インポートに失敗しました: ' + msg, 'error');
        }
    }

    function downloadTemplate() {
        window.open('/api/polls/template.csv', '_blank');
    }
</script>

<div class="bg-white p-6 rounded-xl shadow-sm border border-gray-200">
    <h3 class="text-lg font-bold mb-4 text-gray-900 border-b pb-2">CSVで一括インポート</h3>
    <div class="space-y-3">
        <input type="file" accept=".csv" on:change={onFileChange} class="block w-full text-xs text-gray-500 file:mr-4 file:py-2 file:px-4 file:rounded-lg file:border-0 file:text-xs file:font-medium file:bg-blue-50 file:text-blue-700 hover:file:bg-blue-100 cursor-pointer" />
        <label class="flex items-center gap-2 text-xs text-gray-600 cursor-pointer">
            <input type="checkbox" bind:checked={importDryRun} class="rounded" />
            検証のみ（登録しない）
        </label>
        <button on:click={importCSV} disabled={importLoading || !importFile} class="w-full py-2 bg-blue-600 text-white font-medium rounded-lg hover:bg-blue-700 disabled:bg-gray-300 text-xs">
            {importLoading ? '処理中...' : importDryRun ? '検証する' : 'インポートする'}
        </button>
        {#if importResult}
            <div class="mt-3 p-3 rounded-lg text-xs {importResult.failed > 0 ? 'bg-red-50 border border-red-200' : 'bg-green-50 border border-green-200'}">
                <p class="font-semibold mb-1">成功: {importResult.success} / 失敗: {importResult.failed}</p>
            </div>
        {/if}
    </div>
</div>
