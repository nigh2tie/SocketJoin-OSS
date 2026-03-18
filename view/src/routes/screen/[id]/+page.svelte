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
    import { poll, votes, connectionStatus, event, ranking, questions } from '$lib/store';
    import { connect, disconnect } from '$lib/ws';

    export let data;

    $: totalVotes = Object.values($votes).reduce((a, b) => a + b, 0);

    // Show ranking overlay when quiz poll closes and ranking is available
    $: showRankingOverlay = $poll?.is_quiz && $poll?.status === 'closed' && $ranking.length > 0;

    onMount(async () => {
        if (data.error || !data.event) return;
        await fetch('/api/csrf', { method: 'GET' });
        event.set(data.event);
        connect(data.event.id);

        if (data.event.current_poll_id) {
            fetch(`/api/poll/${data.event.current_poll_id}`)
                .then(res => res.json())
                .then(p => {
                    poll.set(p);
                    fetch(`/api/poll/${p.id}/result`)
                        .then(r => r.json())
                        .then(counts => votes.set(counts))
                        .catch(() => votes.set({}));
                    // Load ranking if this is a closed quiz poll
                    if (p.is_quiz && p.status === 'closed') {
                        fetch(`/api/events/${data.event.id}/ranking`)
                            .then(r => r.json())
                            .then(r => ranking.set(r))
                            .catch(() => {});
                    }
                });
        }
        
        fetch(`/api/events/${data.event.id}/questions`)
            .then(res => res.json())
            .then(qs => questions.set(qs || []));
    });

    onDestroy(() => {
        disconnect();
    });
</script>

<div class="screen">
    {#if data.error || !data.event}
        <div style="color:white;text-align:center;padding:40px">イベントが見つかりません。</div>
    {:else}
    <div class="header">
        <div class="event-title">{data.event.title}</div>
        <div class="join-info">
            参加URL: <span>{typeof window !== 'undefined' ? window.location.origin : ''}/join/{data.event.id}</span>
        </div>
    </div>

    <div class="content" class:split-view={$questions.length > 0}>
        <div class="main-panel">
            {#if !$poll}
                <div class="waiting">
                    <h1>準備中...</h1>
                    <p>次の投票が始まるまでお待ちください</p>
                    <div class="qr-placeholder">
                        <img src={`https://api.qrserver.com/v1/create-qr-code/?size=300x300&data=${encodeURIComponent((typeof window !== 'undefined' ? window.location.origin : '') + '/join/' + data.event.id)}`} alt="QR Code" />
                    </div>
                </div>
            {:else if showRankingOverlay}
                <!-- ランキング表示 -->
                <div class="ranking-view">
                    <h1 class="ranking-title">クイズランキング</h1>
                    <ol class="ranking-list">
                        {#each $ranking as entry}
                            <li class="ranking-item rank-{entry.rank}">
                                <span class="rank-badge">{entry.rank}</span>
                                <span class="rank-nickname">{entry.nickname}</span>
                                <span class="rank-score">{entry.total_score}pt</span>
                            </li>
                        {/each}
                    </ol>
                    {#if $ranking.length === 0}
                        <p style="color:#aaa;text-align:center">まだ得点データがありません</p>
                    {/if}
                </div>
            {:else}
                {@const countsArray = $poll.options.map(o => $votes[o.id] || 0)}
                {@const maxCount = Math.max(0, ...countsArray)}
                <div class="poll-result">
                    <h1>{$poll.title}</h1>
                    <div class="total">総票数: {totalVotes}</div>

                    <div class="charts">
                        {#each $poll.options as opt}
                            {@const count = $votes[opt.id] || 0}
                            {@const percent = totalVotes > 0 ? (count / totalVotes) * 100 : 0}
                            {@const isTop = count === maxCount && count > 0}
                            {@const showHighlight = $poll.status === 'closed' && ($poll.is_quiz ? opt.is_correct : isTop)}
                            {@const showDim = $poll.status === 'closed' && ($poll.is_quiz ? !opt.is_correct : !isTop)}
                            <div class="bar-row" class:incorrect={showDim}>
                                <div class="label">
                                    {opt.label}
                                    {#if showHighlight}
                                        <span class="correct-badge">{$poll.is_quiz ? '正解' : '最多得票'}</span>
                                    {/if}
                                </div>
                                <div class="bar-container" class:correct={showHighlight}>
                                    <div class="bar" style="width: {percent}%"></div>
                                    <div class="value">{count} ({percent.toFixed(1)}%)</div>
                                </div>
                            </div>
                        {/each}
                    </div>
                </div>
            {/if}
        </div>

        {#if $questions.length > 0}
            <div class="qa-side-panel">
                <h2 class="qa-side-title">Q&A</h2>
                <div class="qa-side-list">
                    {#each $questions.slice(0, 5) as q}
                        <div class="qa-side-card" class:answered={q.status === 'answered'}>
                            <div class="qa-side-header">
                                <span class="qa-side-upvotes">▲ {q.upvotes}</span>
                                {#if q.status === 'answered'}
                                    <span class="qa-side-badge">回答済み</span>
                                {/if}
                            </div>
                            <div class="qa-side-body">{q.content}</div>
                        </div>
                    {/each}
                </div>
                {#if $questions.length > 5}
                    <div class="qa-side-more">他 {$questions.length - 5} 件...</div>
                {/if}
            </div>
        {/if}
    </div>

    <div class="footer">
        Powered by SocketJoin
    </div>
    {/if}
</div>

<style>
    :global(body) { margin: 0; background: #1a1a1a; color: white; font-family: sans-serif; }
    .screen { display: flex; flex-direction: column; height: 100vh; }

    .header { padding: 20px 40px; display: flex; justify-content: space-between; align-items: center; background: #333; }
    .event-title { font-size: 1.5em; font-weight: bold; }
    .join-info { font-size: 1.2em; color: #aaa; }
    .join-info span { color: #4db8ff; font-weight: bold; margin-left: 10px; }

    .waiting { text-align: center; }
    .waiting h1 { font-size: 3em; margin-bottom: 20px; }
    .qr-placeholder img { border: 10px solid #fff; border-radius: 10px; margin-top: 30px; }

    .poll-result { width: 80%; max-width: 1000px; }
    .poll-result h1 { font-size: 2.5em; margin-bottom: 10px; text-align: center; }
    .total { text-align: right; font-size: 1.2em; color: #aaa; margin-bottom: 30px; }

    .charts { display: flex; flex-direction: column; gap: 20px; }
    .bar-row { margin-bottom: 20px; transition: opacity 0.5s; }
    .bar-row.incorrect { opacity: 0.4; }
    .label { font-size: 1.5em; margin-bottom: 5px; display: flex; align-items: center; }
    .correct-badge { margin-left: 10px; background: #28a745; color: white; padding: 2px 8px; border-radius: 12px; font-size: 0.6em; font-weight: bold; }
    .bar-container { background: #444; height: 50px; border-radius: 4px; position: relative; overflow: hidden; }
    .bar { height: 100%; background: #007bff; transition: width 0.5s ease-out, background-color 0.5s; }
    .bar-container.correct .bar { background: #28a745; }
    .value { position: absolute; top: 0; right: 10px; height: 100%; display: flex; align-items: center; font-weight: bold; font-size: 1.2em; text-shadow: 1px 1px 2px #000; }

    /* Ranking */
    .ranking-view { width: 80%; max-width: 700px; }
    .ranking-title { font-size: 2.5em; text-align: center; margin-bottom: 30px; color: #ffd700; }
    .ranking-list { list-style: none; padding: 0; margin: 0; display: flex; flex-direction: column; gap: 12px; }
    .ranking-item { display: flex; align-items: center; gap: 20px; padding: 16px 24px; border-radius: 12px; background: #2a2a2a; }
    .ranking-item.rank-1 { background: linear-gradient(135deg, #7b6000, #2a2a2a); border: 2px solid #ffd700; }
    .ranking-item.rank-2 { background: linear-gradient(135deg, #4a4a4a, #2a2a2a); border: 2px solid #c0c0c0; }
    .ranking-item.rank-3 { background: linear-gradient(135deg, #5c3100, #2a2a2a); border: 2px solid #cd7f32; }
    .rank-badge { width: 48px; height: 48px; border-radius: 50%; display: flex; align-items: center; justify-content: center; font-size: 1.4em; font-weight: bold; background: #444; color: #fff; flex-shrink: 0; }
    .rank-nickname { flex: 1; font-size: 1.5em; font-weight: bold; }
    .rank-score { font-size: 1.4em; font-weight: bold; color: #4db8ff; white-space: nowrap; }

    .footer { text-align: center; padding: 10px; font-size: 0.8em; color: #555; }
    
    /* Layout */
    .content { flex: 1; display: flex; padding: 40px; gap: 40px; align-items: stretch; }
    .main-panel { flex: 1; display: flex; flex-direction: column; align-items: center; justify-content: center; width: 100%; transition: width 0.3s; }
    .content.split-view .main-panel { width: 65%; flex: none; }
    
    /* Q&A Side Panel */
    .qa-side-panel { width: 35%; display: flex; flex-direction: column; background: #222; padding: 30px; border-radius: 16px; border: 1px solid #444; box-shadow: 0 10px 30px rgba(0,0,0,0.5); }
    .qa-side-title { font-size: 2em; margin-bottom: 20px; color: #fff; text-align: center; border-bottom: 2px solid #444; padding-bottom: 10px; }
    .qa-side-list { display: flex; flex-direction: column; gap: 15px; overflow-y: hidden; }
    .qa-side-card { background: #333; padding: 20px; border-radius: 12px; border-left: 5px solid #007bff; display: flex; flex-direction: column; transition: all 0.3s; }
    .qa-side-card.answered { border-left-color: #28a745; opacity: 0.7; }
    .qa-side-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 15px; }
    .qa-side-upvotes { font-size: 1.5em; font-weight: bold; color: #4db8ff; }
    .qa-side-badge { background: #28a745; color: white; padding: 4px 10px; border-radius: 20px; font-size: 0.8em; font-weight: bold; }
    .qa-side-body { font-size: 1.3em; line-height: 1.4; color: #eee; white-space: pre-wrap; word-break: break-word; }
    .qa-side-more { text-align: center; color: #888; font-size: 1.2em; font-style: italic; margin-top: 15px; }
</style>
