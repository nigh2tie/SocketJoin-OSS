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
    /** @type {(poll: any) => void} */
    export let onPollCreated;

    let newPollTitle = '';
    let newPollType = 'survey'; // 'survey' or 'quiz'
    let newPollPoints = 10;
    let newPollMaxSelections = 1;
    /** @type {{label: string, is_correct: boolean}[]} */
    let newPollOptions = [{label: '', is_correct: false}, {label: '', is_correct: false}];
    $: isQuiz = newPollType === 'quiz';
    $: quizHasCorrectOption = newPollOptions.some((o) => o.is_correct);
    $: canCreatePoll = Boolean(newPollTitle) && !newPollOptions.some((o) => !o.label) && (!isQuiz || quizHasCorrectOption);

    async function createPoll() {
        if (!canCreatePoll) return;

        const optionsToSend = newPollOptions.map(o => ({
            label: o.label,
            is_correct: isQuiz ? o.is_correct : false
        }));

        const maxSel = Math.min(Math.max(1, newPollMaxSelections), newPollOptions.length);

        const res = await fetch(`/api/events/${eventId}/polls`, {
            method: 'POST',
            body: JSON.stringify({
                title: newPollTitle,
                is_quiz: isQuiz,
                points: isQuiz ? newPollPoints : 0,
                max_selections: maxSel,
                options: optionsToSend
            }),
            headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': csrfToken }
        });

        if (res.ok) {
            const poll = await res.json();
            onPollCreated(poll);
            newPollTitle = '';
            newPollType = 'survey';
            newPollMaxSelections = 1;
            newPollPoints = 10;
            newPollOptions = [{label: '', is_correct: false}, {label: '', is_correct: false}];
            showToast('作成が完了しました。');
        } else {
            let msg = '作成に失敗しました。';
            try {
                const body = await res.json();
                msg = body.error || msg;
            } catch {
                const text = await res.text();
                if (text) msg = text;
            }
            showToast('作成に失敗しました: ' + msg, 'error');
        }
    }

    function addOption() {
        newPollOptions = [...newPollOptions, {label: '', is_correct: false}];
    }

    /** @param {number} index */
    function removeOption(index) {
        if (newPollOptions.length <= 2) return;
        newPollOptions = newPollOptions.filter((_, i) => i !== index);
        if (newPollMaxSelections > newPollOptions.length) {
            newPollMaxSelections = newPollOptions.length;
        }
    }
</script>

    <div class={`bg-white p-6 rounded-xl shadow-sm border transition-all duration-300 ${isQuiz ? 'border-purple-200 bg-purple-50/30' : 'border-gray-200'}`}>
    <div class="flex items-center justify-between mb-4 border-b pb-2">
        <h3 class="text-lg font-bold text-gray-900">新しい項目を追加</h3>
        <div class="flex bg-gray-100 p-1 rounded-lg">
            <button
                class={`px-4 py-1.5 text-xs font-bold rounded-md transition-all ${newPollType === 'survey' ? 'bg-white text-blue-600 shadow-sm' : 'text-gray-500 hover:text-gray-700'}`}
                on:click={() => newPollType = 'survey'}
            >
                アンケート
            </button>
            <button
                class={`px-4 py-1.5 text-xs font-bold rounded-md transition-all ${isQuiz ? 'bg-purple-600 text-white shadow-sm' : 'text-gray-500 hover:text-gray-700'}`}
                on:click={() => newPollType = 'quiz'}
            >
                クイズ
            </button>
        </div>
    </div>

    <div class="space-y-4">
        <div>
            <label for="new-poll-title" class="block text-xs font-bold text-gray-500 uppercase tracking-wider mb-1">
                {isQuiz ? 'クイズの問題文' : '質問タイトル'}
            </label>
            <input id="new-poll-title" type="text" bind:value={newPollTitle} placeholder={isQuiz ? '例: 日本で一番高い山は？' : '例: 本日の満足度は？'} class="w-full p-3 bg-white border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 focus:outline-none" />
        </div>

        <!-- Options -->
        <div class="space-y-2">
            <div class="block text-xs font-bold text-gray-500 uppercase tracking-wider mb-1">選択肢</div>
            {#each newPollOptions as opt, i}
                <div class="flex items-center gap-2">
                    {#if isQuiz}
                        <button
                            class={`px-3 py-2 text-xs font-bold rounded-lg border transition-all ${newPollOptions[i].is_correct ? 'bg-green-500 border-green-600 text-white' : 'bg-gray-100 border-gray-200 text-gray-400 hover:bg-gray-200'}`}
                            on:click={() => {
                                if (newPollMaxSelections === 1) {
                                    newPollOptions = newPollOptions.map((o, idx) => ({ ...o, is_correct: idx === i }));
                                } else {
                                    newPollOptions[i].is_correct = !newPollOptions[i].is_correct;
                                }
                            }}
                        >
                            {newPollOptions[i].is_correct ? '正解' : '不正解'}
                        </button>
                    {/if}
                    <input type="text" bind:value={newPollOptions[i].label} placeholder={`選択肢 ${i + 1}`} class="flex-1 p-3 bg-white border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 focus:outline-none" />
                    {#if newPollOptions.length > 2}
                        <button class="px-3 py-2 text-gray-400 hover:text-red-600 font-bold" on:click={() => removeOption(i)}>✕</button>
                    {/if}
                </div>
            {/each}
            {#if isQuiz && !quizHasCorrectOption}
                <p class="text-xs font-medium text-amber-700 bg-amber-50 border border-amber-200 rounded-lg px-3 py-2">
                    クイズを作成するには、少なくとも1つの選択肢を「正解」にしてください。
                </p>
            {/if}
        </div>

        <!-- Advanced Settings -->
        <div class="grid grid-cols-2 gap-4 pt-2 border-t">
            <div class="flex flex-col gap-1">
                <label for="max-sel" class="text-xs font-bold text-gray-500 uppercase">最大選択数</label>
                <div class="flex items-center gap-2">
                    <input id="max-sel" type="number" bind:value={newPollMaxSelections} min="1" max={newPollOptions.length} class="w-full p-2 border border-gray-300 rounded-lg text-center" />
                    <span class="text-xs text-gray-400 whitespace-nowrap">つ選択</span>
                </div>
            </div>
            {#if isQuiz}
                <div class="flex flex-col gap-1">
                    <label for="poll-points" class="text-xs font-bold text-gray-500 uppercase">配点</label>
                    <div class="flex items-center gap-2">
                        <input id="poll-points" type="number" bind:value={newPollPoints} min="0" class="w-full p-2 border border-purple-300 bg-purple-50 rounded-lg text-center font-bold text-purple-700" />
                        <span class="text-xs text-purple-600 font-bold">pt</span>
                    </div>
                </div>
            {/if}
        </div>

        <div class="flex gap-3 pt-2">
            <button class="flex-1 py-2.5 bg-gray-100 text-gray-700 font-bold hover:bg-gray-200 rounded-lg transition text-sm" on:click={addOption}>+ 選択肢を追加</button>
            <button
                on:click={createPoll}
                class={`flex-1 py-2.5 text-white font-bold rounded-lg transition text-sm ${isQuiz ? 'bg-purple-600 hover:bg-purple-700 shadow-purple-200 shadow-lg' : 'bg-blue-600 hover:bg-blue-700 shadow-blue-200 shadow-lg'} ${!canCreatePoll ? 'opacity-60 cursor-not-allowed' : ''}`}
                disabled={!canCreatePoll}
            >
                {isQuiz ? 'クイズを作成' : '投票を作成'}
            </button>
        </div>
    </div>
</div>
