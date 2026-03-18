// SocketJoin: Real-time event interaction platform.
// Copyright (C) 2026 Q-Q
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

import { poll, votes, connectionStatus, ranking, questions } from './store';
import { get } from 'svelte/store';

/** @type {WebSocket | null} */
let socket = null;
/** @type {ReturnType<typeof setTimeout> | null} */
let reconnectTimer = null;
/** @type {string | null} */
let activeEventID = null;
let reconnectAttempts = 0;

const reconnectBaseMs = 500;
const reconnectMaxMs = 10000;

/**
 * Connect to WebSocket for a specific Event
 * @param {string} eventID
 */
export function connect(eventID) {
    activeEventID = eventID;
    reconnectAttempts = 0;
    clearReconnectTimer();
    closeSocket();

    openSocket(eventID);
}

export function disconnect() {
    activeEventID = null;
    reconnectAttempts = 0;
    clearReconnectTimer();
    closeSocket();
    connectionStatus.set('disconnected');
}

/**
 * @param {string} eventID
 */
function openSocket(eventID) {
    connectionStatus.set('connecting');

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const host = window.location.host;
    const wsUrl = `${protocol}//${host}/ws/event/${eventID}`;

    const ws = new WebSocket(wsUrl);
    socket = ws;

    ws.onopen = () => {
        if (socket !== ws) {
            return;
        }
        connectionStatus.set('connected');
        reconnectAttempts = 0;
        clearReconnectTimer();
        console.log('WS Connected');
    };

    ws.onmessage = (eventMsg) => {
        try {
            const data = JSON.parse(eventMsg.data);
            console.log('WS Message:', data);

            switch (data.type) {
                case 'poll.updated':
                    {
                        const currentPoll = get(poll);
                        if (!currentPoll || !data?.payload) {
                            break;
                        }
                        if (currentPoll.id === data.payload.poll_id) {
                            votes.set(data.payload.counts || {});
                        }
                    }
                    break;
                case 'poll.closed':
                    {
                        const currentPoll = get(poll);
                        if (currentPoll && data?.payload && currentPoll.id === data.payload.poll_id) {
                            if (data.payload.counts) {
                                votes.set(data.payload.counts);
                            }
                            
                            // 締切後は is_correct がアンマスクされる。
                            // バックエンドが全データを送るように改善されたため、fetch なしで更新可能。
                            // 既に投票済みの参加者のために poll ストアを更新する。
                            poll.update(p => p ? { ...p, status: 'closed' } : null);

                            // クイズの場合はランキングを取得する
                            if (currentPoll.is_quiz) {
                                fetch(`/api/events/${currentPoll.event_id}/ranking`)
                                    .then(r => r.json())
                                    .then(r => ranking.set(r))
                                    .catch(() => {});
                            }
                        }
                    }
                    break;
                case 'poll.reset':
                    {
                        const currentPoll = get(poll);
                        if (currentPoll && data?.payload && currentPoll.id === data.payload.poll_id) {
                            // Clear localStorage voted flag so participants can vote again
                            try {
                                const votedPolls = JSON.parse(localStorage.getItem('voted_polls') || '[]');
                                const updated = votedPolls.filter(/** @param {string} id */ id => id !== data.payload.poll_id);
                                localStorage.setItem('voted_polls', JSON.stringify(updated));
                            } catch {}
                            votes.set({});
                            // Update poll object to trigger $: if ($poll) reactive block in join page,
                            // which calls checkVoted and will now find hasVoted=false.
                            poll.update(p => p ? { ...p, status: 'open' } : null);
                        }
                    }
                    break;
                case 'event.updated':
                    if (data?.payload?.poll) {
                        // 新しい Poll 情報が直接送られてきた場合
                        poll.set(data.payload.poll);
                        votes.set(data.payload.counts || {});
                    } else if (data?.payload?.current_poll_id) {
                        // poll 移行時（互換性/フォールバック）
                        fetch(`/api/poll/${data.payload.current_poll_id}`)
                            .then(res => res.json())
                            .then(newPoll => {
                                poll.set(newPoll);
                                votes.set({});
                                fetch(`/api/poll/${newPoll.id}/result`)
                                    .then(r => r.json())
                                    .then(counts => votes.set(counts))
                                    .catch(() => votes.set({}));
                            })
                            .catch(() => {});
                    } else if (data?.payload?.title) {
                        // イベントタイトル更新
                        const p = get(poll);
                        if (p) {
                            // イベント全体を再取得するのではなく、タイトルだけ更新
                            // (event ストアがないため poll.event_title などがある場合はここで更新)
                        }
                    } else {
                        poll.set(null);
                        votes.set({});
                    }
                    break;
                case 'qa.question_created':
                case 'qa.question_upvoted':
                case 'qa.question_status_updated':
                    if (activeEventID) {
                        fetch(`/api/events/${activeEventID}/questions`)
                            .then(res => res.json())
                            .then(qs => questions.set(qs || []))
                            .catch(e => console.error(e));
                    }
                    break;
                default:
                    break;
            }
        } catch (e) {
            console.error('WS Error:', e);
        }
    };

    ws.onclose = () => {
        if (socket !== ws) {
            return;
        }
        socket = null;
        connectionStatus.set('disconnected');
        scheduleReconnect();
    };

    ws.onerror = () => {
        if (socket !== ws) {
            return;
        }
        connectionStatus.set('disconnected');
    };
}

function clearReconnectTimer() {
    if (!reconnectTimer) {
        return;
    }
    clearTimeout(reconnectTimer);
    reconnectTimer = null;
}

function closeSocket() {
    if (!socket) {
        return;
    }
    socket.onopen = null;
    socket.onmessage = null;
    socket.onclose = null;
    socket.onerror = null;
    socket.close();
    socket = null;
}

function scheduleReconnect() {
    if (!activeEventID) {
        return;
    }

    clearReconnectTimer();

    const delay = Math.min(
        reconnectBaseMs * Math.pow(2, reconnectAttempts),
        reconnectMaxMs
    );
    reconnectAttempts += 1;

    reconnectTimer = setTimeout(() => {
        if (!activeEventID) {
            return;
        }
        openSocket(activeEventID);
    }, delay);
}
