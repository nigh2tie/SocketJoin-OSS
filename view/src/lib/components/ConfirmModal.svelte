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
    /** @type {{ message: string, onConfirm: function } | null} */
    export let modal = null;
    /** @type {() => void} */
    export let onCancel = () => {};
</script>

{#if modal}
<div class="fixed inset-0 z-[6000] flex items-center justify-center bg-black/40 backdrop-blur-sm transition-opacity">
    <div class="bg-white rounded-2xl shadow-2xl p-6 max-w-sm w-full mx-4 border border-gray-100 transform transition-all animate-in zoom-in-95 duration-200">
        <h3 class="text-xl font-extrabold text-gray-900 mb-2">確認</h3>
        <p class="text-gray-600 text-sm mb-6 whitespace-pre-wrap leading-relaxed">{modal.message}</p>
        <div class="flex gap-3 justify-end">
            <button class="px-5 py-2.5 text-sm font-bold text-gray-700 bg-gray-100 hover:bg-gray-200 rounded-xl transition-colors focus:ring-2 focus:ring-gray-200 outline-none" on:click={onCancel}>キャンセル</button>
            <button class="px-5 py-2.5 text-sm font-bold text-white bg-blue-600 hover:bg-blue-700 rounded-xl shadow-md transition-colors focus:ring-2 focus:ring-blue-400 outline-none" on:click={() => {
                modal?.onConfirm();
                onCancel();
            }}>OK</button>
        </div>
    </div>
</div>
{/if}
