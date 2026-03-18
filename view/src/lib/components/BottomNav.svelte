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
    export let currentTab = 'poll';
    /** @type {(tab: string) => void} */
    export let onTabChange;
</script>

<nav class="bottom-nav">
    <button class="nav-btn" class:active={currentTab === 'poll'} on:click={() => onTabChange('poll')}>
        <svg xmlns="http://www.w3.org/2000/svg" class="nav-icon" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z" /></svg>
        <span class="nav-label">投票</span>
    </button>
    <button class="nav-btn" class:active={currentTab === 'qa'} on:click={() => onTabChange('qa')}>
        <svg xmlns="http://www.w3.org/2000/svg" class="nav-icon" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z" /></svg>
        <span class="nav-label">Q&A</span>
    </button>
    <button class="nav-btn" class:active={currentTab === 'history'} on:click={() => onTabChange('history')}>
        <svg xmlns="http://www.w3.org/2000/svg" class="nav-icon" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" /></svg>
        <span class="nav-label">履歴</span>
    </button>
</nav>

<style>
    .bottom-nav { position: fixed; bottom: 0; left: 0; right: 0; height: 75px; background: rgba(255, 255, 255, 0.95); backdrop-filter: blur(10px); display: flex; box-shadow: 0 -2px 15px rgba(0,0,0,0.08); z-index: 1000; border-top: 1px solid rgba(0,0,0,0.05); }
    .nav-btn { flex: 1; display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 4px; background: transparent; border: none; color: #9ca3af; border-radius: 0; cursor: pointer; transition: color 0.2s; padding: 0; }
    .nav-btn:hover { background: transparent; color: #6b7280; }
    .nav-btn.active { color: #2563eb; }
    .nav-icon { width: 28px; height: 28px; transition: transform 0.2s ease; }
    .nav-btn.active .nav-icon { transform: scale(1.1); }
    .nav-label { font-size: 0.75rem; font-weight: 600; }
</style>
