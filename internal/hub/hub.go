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
	"encoding/json"
)

type Message struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

type RoomMessage struct {
	RoomID  string
	Message []byte
}

type Hub struct {
	// Registered clients.
	clients map[*Client]bool

	// Inbound messages from the clients.
	broadcast chan []byte

	// Broadcast to specific room (poll_id)
	broadcastRoom chan RoomMessage

	// Broadcast to specific room internal map
	rooms map[string]map[*Client]bool

	// Register requests from the clients.
	register chan *Client

	// Unregister requests from clients.
	unregister chan *Client
}

func NewHub() *Hub {
	return &Hub{
		broadcast:     make(chan []byte),
		broadcastRoom: make(chan RoomMessage),
		register:      make(chan *Client),
		unregister:    make(chan *Client),
		clients:       make(map[*Client]bool),
		rooms:         make(map[string]map[*Client]bool),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = true
			if h.rooms[client.roomID] == nil {
				h.rooms[client.roomID] = make(map[*Client]bool)
			}
			h.rooms[client.roomID][client] = true

		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				h.removeClient(client)
			}

		case message := <-h.broadcast: // グローバルブロードキャスト
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					h.removeClient(client)
				}
			}

		case rm := <-h.broadcastRoom: // ルームへのブロードキャスト
			if clients, ok := h.rooms[rm.RoomID]; ok {
				for client := range clients {
					select {
					case client.send <- rm.Message:
					default:
						h.removeClient(client)
					}
				}
			}
		}
	}
}

func (h *Hub) removeClient(client *Client) {
	delete(h.clients, client)
	close(client.send)

	if h.rooms[client.roomID] != nil {
		delete(h.rooms[client.roomID], client)
		if len(h.rooms[client.roomID]) == 0 {
			delete(h.rooms, client.roomID)
		}
	}
}

func (h *Hub) BroadcastToRoom(roomID string, message []byte) {
	h.broadcastRoom <- RoomMessage{RoomID: roomID, Message: message}
}
