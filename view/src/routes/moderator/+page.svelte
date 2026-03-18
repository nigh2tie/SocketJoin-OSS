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
    import { page } from '$app/stores';
    import { goto } from '$app/navigation';

    let errorMsg = '';
    let loading = true;

    onMount(async () => {
        // Ensure CSRF token is available
        await fetch('/api/csrf', { method: 'GET' });

        const token = $page.url.searchParams.get('token');
        if (!token) {
            errorMsg = 'トークンが指定されていません。正しいURLにアクセスしてください。';
            loading = false;
            return;
        }

        try {
            function getCsrfToken() {
                if (typeof document !== 'undefined') {
                    const match = document.cookie.match(new RegExp('(^| )csrf_token=([^;]+)'));
                    if (match) return match[2];
                }
                return '';
            }

            const res = await fetch('/api/moderator/login', {
                method: 'POST',
                body: JSON.stringify({ token }),
                headers: { 
                    'Content-Type': 'application/json',
                    'X-CSRF-Token': getCsrfToken()
                }
            });

            if (res.ok) {
                const data = await res.json();
                goto(`/host/${data.event_id}`);
            } else {
                errorMsg = 'トークンが無効か、有効期限が切れています。管理者に再度URLを発行してもらってください。';
                loading = false;
            }
        } catch (err) {
            console.error('Login failed', err);
            errorMsg = 'サーバー通信エラーが発生しました。時間を置いて再度お試しください。';
            loading = false;
        }
    });
</script>

<div class="min-h-screen bg-gray-50 flex items-center justify-center p-6 font-sans">
    <div class="bg-white p-8 rounded-xl shadow-lg border border-gray-100 max-w-md w-full text-center">
        <h1 class="text-2xl font-bold text-gray-900 mb-6">モデレーターログイン</h1>
        
        {#if loading}
            <div class="flex flex-col items-center justify-center space-y-4">
                <div class="w-10 h-10 border-4 border-blue-500 border-t-transparent rounded-full animate-spin"></div>
                <p class="text-gray-600">認証中...</p>
            </div>
        {:else if errorMsg}
            <div class="p-4 bg-red-50 text-red-700 rounded-lg border border-red-200">
                <p class="font-medium text-lg mb-2">ログインエラー</p>
                <p class="text-sm">{errorMsg}</p>
            </div>
            <div class="mt-6">
                <a href="/" class="text-blue-600 hover:text-blue-800 hover:underline text-sm font-medium">トップページへ戻る</a>
            </div>
        {/if}
    </div>
</div>
