package websockets

import (
	"encoding/json"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
	ps "scanner/product_state"
	"sync"
)

const (
	EANNotFoundCode = iota + 1001
	NoneScansToUndoCode
	InternalServerErrorCode
)

var (
	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}
	connections = make(map[*websocket.Conn]bool)
	mu          sync.Mutex
)

type WebSocketMessage struct {
	Ean      string `json:"ean"`
	Quantity uint   `json:"quantity"`
}

func HandleXMLWebSocket(ctx *gin.Context) {
	conn, err := upgrader.Upgrade(ctx.Writer, ctx.Request, nil)
	if err != nil {
		ctx.JSON(500, gin.H{
			"status": false,
			"error":  "unable to upgrade connection",
		})
		return
	}
	defer func() {
		mu.Lock()
		delete(connections, conn)
		mu.Unlock()
		conn.Close()
	}()

	mu.Lock()
	connections[conn] = true
	mu.Unlock()

	for {
		// get ean from websocket
		_, message, err := conn.ReadMessage()
		if err != nil {
			log.Println("WebSocket read error:", err)
			sendError(conn, InternalServerErrorCode, "Unable to read message")
			break
		}

		var msg WebSocketMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			log.Println("WebSocket unmarshal error:", err)
			sendError(conn, InternalServerErrorCode, "Unable to unmarshal message")
			continue
		}

		state := ps.GetProductState(msg.Ean)
		if state == nil {
			sendError(conn, EANNotFoundCode, "EAN not found")
			continue
		}

		state.Scanned += msg.Quantity
		state.Difference = int(state.Scanned - uint(state.Amount))

		ps.SetProductState(msg.Ean, state)

		// status = 1 - completed / status = 2 - working / status = 3 - error / status = 0 - waiting
		if state.Difference == 0 {
			state.Status = 1
		} else if state.Difference < 0 {
			state.Status = 2
		} else if state.Difference > 0 {
			state.Status = 3
		} else {
			state.Status = 0
		}

		ps.ScanHistArr = append(ps.ScanHistArr, ps.ScanHistory{
			Ean:      msg.Ean,
			Quantity: msg.Quantity,
		})

		res := WebSocketResponse{
			Action:       "scan",
			ProductState: state,
		}

		jsonResponse, err := json.Marshal(res)
		if err != nil {
			log.Println("WebSocket marshal error:", err)
			sendError(conn, InternalServerErrorCode, "Unable to marshal message")
			continue
		}

		Broadcast(jsonResponse)
	}
}

type WebSocketResponse struct {
	Action string `json:"action"`
	*ps.ProductState
}

func Broadcast(message []byte) {
	mu.Lock()
	defer mu.Unlock()

	for conn := range connections {
		if err := conn.WriteMessage(websocket.TextMessage, message); err != nil {
			conn.Close()
			delete(connections, conn)
		}
	}
}

type ErrorMessage struct {
	Status bool   `json:"status"`
	Code   int    `json:"code"`
	Error  string `json:"error"`
}

func sendError(conn *websocket.Conn, code int, message string) {
	errorResponse, _ := json.Marshal(ErrorMessage{Status: false, Code: code, Error: message})
	if err := conn.WriteMessage(websocket.TextMessage, errorResponse); err != nil {
		log.Println("WebSocket write error:", err)
		return
	}
}
