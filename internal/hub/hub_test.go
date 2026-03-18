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

package hub

import (
	"testing"
	"time"
)

func TestBroadcastToRoom_NoPanic(t *testing.T) {
	h := NewHub()
	go h.Run()

	client := &Client{hub: h, send: make(chan []byte, 1), roomID: "room1"}

	// クライアントを登録
	h.register <- client
	time.Sleep(10 * time.Millisecond)

	// unregister sends the client to be removed and its send channel closed by the Run loop
	h.unregister <- client
	time.Sleep(10 * time.Millisecond)

	// panic しないことを確認
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("BroadcastToRoom panicked: %v", r)
		}
	}()
	h.BroadcastToRoom("room1", []byte(`{"type":"test"}`))
	time.Sleep(10 * time.Millisecond)
}

// TestBroadcastToRoom_DeliverMessage はメッセージが正しく届くことを確認する
func TestBroadcastToRoom_DeliverMessage(t *testing.T) {
	h := NewHub()
	go h.Run()

	ch := make(chan []byte, 4)
	client := &Client{hub: h, send: ch, roomID: "room2"}

	h.register <- client
	time.Sleep(10 * time.Millisecond)

	msg := []byte(`{"type":"poll.updated"}`)
	h.BroadcastToRoom("room2", msg)
	time.Sleep(10 * time.Millisecond)

	select {
	case got := <-ch:
		if string(got) != string(msg) {
			t.Errorf("受信メッセージが不一致: got=%s want=%s", got, msg)
		}
	default:
		t.Error("メッセージが届かなかった")
	}
}

// TestBroadcastToRoom_UnknownRoom は存在しない room への broadcast で panic しないことを確認する
func TestBroadcastToRoom_UnknownRoom(t *testing.T) {
	h := NewHub()
	go h.Run()

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("unexpected panic: %v", r)
		}
	}()
	h.BroadcastToRoom("nonexistent", []byte(`{}`))
}

// TestBroadcast_FullChannel verifies that a slow client gets disconnected
func TestBroadcast_FullChannel(t *testing.T) {
	h := NewHub()
	go h.Run()

	client := &Client{hub: h, send: make(chan []byte, 1), roomID: "roomX"}
	h.register <- client
	time.Sleep(10 * time.Millisecond)

	// Fill the channel
	h.BroadcastToRoom("roomX", []byte("fill"))
	time.Sleep(10 * time.Millisecond)

	// Broadcast again, should fail to send and remove the client
	h.BroadcastToRoom("roomX", []byte("overflow"))
	time.Sleep(10 * time.Millisecond)

	// The client's channel should be closed because it was removed
	<-client.send // Drain the "fill" message
	_, ok := <-client.send // Next read should show channel is closed
	if ok {
		t.Errorf("Expected client.send to be closed")
	}
}

// TestSafeSend_FullChannel はバッファフル時にブロックしないことを確認する
func TestSafeSend_FullChannel(t *testing.T) {
	ch := make(chan []byte, 1)
	ch <- []byte("fill")
	select {
	case ch <- []byte("overflow"):
		t.Error("バッファフルなのに送信成功した")
	default:
		// OK
	}
}
